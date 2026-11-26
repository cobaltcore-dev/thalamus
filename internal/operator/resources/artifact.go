// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"slices"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	modelVolumeName        = "model"
	modelVolumeMountPath   = "/models"
	baseModelPath          = modelVolumeMountPath + "/base"
	draftModelPath         = modelVolumeMountPath + "/draft"
	modelDownloadImage     = "ghcr.io/astral-sh/uv:python3.12-bookworm-slim"
	modelDownloadContainer = "model-download"
)

// modelFilePatterns limits downloads to weight, config and tokenizer files.
var modelFilePatterns = []string{"*.safetensors", "*.json", "*.txt", "*.model", "*.jinja"}

// huggingFaceIncludeArgs renders the patterns as "hf download --include" flags.
func huggingFaceIncludeArgs() string {
	args := make([]string, 0, len(modelFilePatterns))
	for _, p := range modelFilePatterns {
		args = append(args, fmt.Sprintf("--include %q", p))
	}
	return strings.Join(args, " ")
}

// modctlPatternFlags renders the patterns as quoted "modctl --patterns" flags for
// use inside a shell command; quoting stops the shell from expanding the globs.
func modctlPatternFlags() string {
	flags := make([]string, 0, len(modelFilePatterns))
	for _, p := range modelFilePatterns {
		flags = append(flags, fmt.Sprintf("--patterns %q", p))
	}
	return strings.Join(flags, " ")
}

// Source schemes supported in spec.base and spec.draft.
const (
	schemeHuggingFace = "hf"
	schemeModelPack   = "modelpack"
)

// sourceScheme returns the scheme of a weights URL (the part before "://"),
// defaulting to Hugging Face when the URL carries no scheme.
func sourceScheme(url string) string {
	if scheme, _, ok := strings.Cut(url, "://"); ok {
		return scheme
	}
	return schemeHuggingFace
}

// downloadContainers returns the container(s) that fetch the artifact's weights
// into the model volume, dispatched on the scheme of spec.base. The scheme is
// constrained by CEL validation; anything unexpected falls back to Hugging Face.
func downloadContainers(artifact *v1alpha1.ModelArtifact) []corev1.Container {
	switch sourceScheme(artifact.Spec.Base) {
	case schemeModelPack:
		return []corev1.Container{modelPackContainer(artifact)}
	default:
		return []corev1.Container{huggingFaceContainer(artifact)}
	}
}

// BuildArtifactDownloadJob returns the artifact's prefetch Job.
func BuildArtifactDownloadJob(artifact *v1alpha1.ModelArtifact, name string) *batchv1.Job {
	claim := artifact.Spec.Volume
	job := &batchv1.Job{
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers:    downloadContainers(artifact),
					Volumes: []corev1.Volume{{
						Name: modelVolumeName,
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: claim.ClaimName,
						},
					}},
				},
			},
		},
	}
	job.Name = name
	job.Namespace = artifact.Namespace
	return job
}

// huggingFaceContainer downloads base and draft weights with the Hugging Face CLI.
func huggingFaceContainer(artifact *v1alpha1.ModelArtifact) corev1.Container {
	return corev1.Container{
		Name:         modelDownloadContainer,
		Image:        modelDownloadImage,
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{downloadCommand(artifact)},
		VolumeMounts: []corev1.VolumeMount{modelVolumeMount()},
	}
}

func downloadCommand(artifact *v1alpha1.ModelArtifact) string {
	include := huggingFaceIncludeArgs()
	commands := []string{
		fmt.Sprintf("uvx --from huggingface_hub hf download %s --local-dir %s %s", artifact.BaseRepoID(), baseModelPath, include),
	}
	if draft := artifact.DraftRepoID(); draft != "" {
		commands = append(commands, fmt.Sprintf("uvx --from huggingface_hub hf download %s --local-dir %s %s", draft, draftModelPath, include))
	}
	return "set -eu; " + strings.Join(commands, "; ")
}

// modelDownloadInitContainers returns the init containers that download the weights
// into the pod-local model volume, reusing the engine's env and volume mounts.
func modelDownloadInitContainers(engine *corev1.Container, artifact *v1alpha1.ModelArtifact) []corev1.Container {
	containers := downloadContainers(artifact)
	for i := range containers {
		containers[i].Env = slices.Concat(engine.Env, containers[i].Env)
		containers[i].VolumeMounts = slices.Concat(engine.VolumeMounts, containers[i].VolumeMounts)
	}
	return containers
}

// ModelVolume returns the artifact's model volume.
func ModelVolume(artifact *v1alpha1.ModelArtifact) corev1.Volume {
	return corev1.Volume{
		Name: modelVolumeName,
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: artifact.Spec.Volume.ClaimName,
		},
	}
}

// modelEmptyDirVolume returns the pod-local model volume.
func modelEmptyDirVolume() corev1.Volume {
	return corev1.Volume{
		Name:     modelVolumeName,
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
}

func modelVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{Name: modelVolumeName, MountPath: modelVolumeMountPath}
}

func setVolume(volumes []corev1.Volume, volume corev1.Volume) []corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == volume.Name {
			volumes[i] = volume
			return volumes
		}
	}
	return append(volumes, volume)
}

func setVolumeMount(mounts []corev1.VolumeMount, mount corev1.VolumeMount) []corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == mount.Name {
			mounts[i] = mount
			return mounts
		}
	}
	return append(mounts, mount)
}
