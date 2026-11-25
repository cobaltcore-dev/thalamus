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
	modelDownloadInclude   = `--include "*.safetensors" --include "*.json" --include "*.txt" --include "*.model" --include "*.jinja"`
)

// BuildArtifactDownloadJob returns the artifact's prefetch Job.
func BuildArtifactDownloadJob(artifact *v1alpha1.ModelArtifact, name string) *batchv1.Job {
	claim := artifact.Spec.Volume
	job := &batchv1.Job{
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name: modelDownloadContainer, Image: modelDownloadImage,
						Command: []string{"/bin/sh", "-c"}, Args: []string{downloadCommand(artifact)},
						VolumeMounts: []corev1.VolumeMount{{Name: modelVolumeName, MountPath: modelVolumeMountPath}},
					}},
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

func downloadCommand(artifact *v1alpha1.ModelArtifact) string {
	commands := []string{
		fmt.Sprintf("uvx --from huggingface_hub hf download %s --local-dir %s %s", artifact.BaseRepoID(), baseModelPath, modelDownloadInclude),
	}
	if draft := artifact.DraftRepoID(); draft != "" {
		commands = append(commands, fmt.Sprintf("uvx --from huggingface_hub hf download %s --local-dir %s %s", draft, draftModelPath, modelDownloadInclude))
	}
	return "set -eu; " + strings.Join(commands, "; ")
}

// modelDownloadInitContainer returns the init container that downloads the weights into the pod-local model volume.
func modelDownloadInitContainer(engine *corev1.Container, artifact *v1alpha1.ModelArtifact) corev1.Container {
	return corev1.Container{
		Name:         modelDownloadContainer,
		Image:        modelDownloadImage,
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{downloadCommand(artifact)},
		Env:          engine.Env,
		VolumeMounts: slices.Concat(engine.VolumeMounts, []corev1.VolumeMount{modelVolumeMount()}),
	}
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
