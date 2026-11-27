// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
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

// patternArgs renders the patterns as repeated flag pairs, e.g. --include or --patterns.
func patternArgs(flag string) []string {
	args := make([]string, 0, len(modelFilePatterns)*2)
	for _, p := range modelFilePatterns {
		args = append(args, flag, p)
	}
	return args
}

// Protocols supported in spec.base and spec.draft.
const (
	protocolHuggingFace = "hf"
	protocolModelPack   = "modelpack"
)

// downloadContainers returns one downloader container per weights fetch (base
// and optional draft), using modctl for modelpack:// artifacts and the
// Hugging Face CLI otherwise.
func downloadContainers(artifact *v1alpha1.ModelArtifact) []corev1.Container {
	fetches := []struct{ repoID, outputDir string }{
		{artifact.BaseRepoID(), baseModelPath},
	}
	if draft := artifact.DraftRepoID(); draft != "" {
		fetches = append(fetches, struct{ repoID, outputDir string }{draft, draftModelPath})
	}

	protocol, _, _ := strings.Cut(artifact.Spec.Base, "://")
	containers := make([]corev1.Container, 0, len(fetches))
	for i, fetch := range fetches {
		name := modelDownloadContainer
		if i > 0 {
			name += "-draft"
		}
		switch protocol {
		case protocolModelPack:
			containers = append(containers, modelPackContainer(name, fetch.repoID, fetch.outputDir))
		case protocolHuggingFace:
			fallthrough
		default:
			containers = append(containers, huggingFaceContainer(name, fetch.repoID, fetch.outputDir))
		}
		containers[i].SecurityContext = hardenedContainerSecurityContext(ubuntuNobody)
		containers[i].Env = cacheEnv
		// Scratch space for the read-only root filesystem.
		containers[i].VolumeMounts = append(containers[i].VolumeMounts,
			corev1.VolumeMount{Name: "cache", MountPath: "/cache"},
			corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"},
		)
	}
	return containers
}

// BuildArtifactDownloadJob returns the artifact's prefetch Job.
func BuildArtifactDownloadJob(artifact *v1alpha1.ModelArtifact, name string) *batchv1.Job {
	claim := artifact.Spec.Volume
	job := &batchv1.Job{
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyOnFailure,
					SecurityContext: hardenedPodSecurityContext(ubuntuNobody, new(ubuntuNobody)),
					Containers:      downloadContainers(artifact),
					Volumes: []corev1.Volume{
						{
							Name: modelVolumeName,
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: claim.ClaimName,
							},
						},
						{Name: "cache", EmptyDir: &corev1.EmptyDirVolumeSource{}},
						{Name: "tmp", EmptyDir: &corev1.EmptyDirVolumeSource{}},
					},
				},
			},
		},
	}
	job.Name = name
	job.Namespace = artifact.Namespace
	return job
}

// huggingFaceContainer downloads weights with the Hugging Face CLI.
func huggingFaceContainer(name, repoID, outputDir string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   modelDownloadImage,
		Command: []string{"uvx"},
		Args: slices.Concat(
			[]string{"--from", "huggingface_hub", "hf", "download", repoID, "--local-dir", outputDir},
			patternArgs("--include"),
		),
		VolumeMounts: []corev1.VolumeMount{modelVolumeMount()},
	}
}

// modelDownloadInitContainers returns the init containers that download the weights
// into the pod-local model volume, reusing the engine's env and volume mounts.
func modelDownloadInitContainers(engine *corev1.Container, artifact *v1alpha1.ModelArtifact) []corev1.Container {
	containers := downloadContainers(artifact)
	for i := range containers {
		containers[i].Env = engine.Env
		for _, m := range engine.VolumeMounts {
			containers[i].VolumeMounts = setVolumeMount(containers[i].VolumeMounts, m)
		}
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
