package resources

import (
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	modelPackImage      = "ghcr.io/modelpack/modctl:0.2"
	modelPackStorageDir = modelVolumeMountPath + "/.modctl"
)

// downloaderEnv is required for modctl to run properly in rootless environment
var downloaderEnv = []corev1.EnvVar{
	{Name: "USER", Value: strconv.FormatInt(ubuntuNobody, 10)},
	{Name: "HOME", Value: modelVolumeMountPath + "/.home"},
}

// modelPackContainers downloads base and draft weights from ModelPack images,
// one modctl container per weight set.
func modelPackContainers(artifact *v1alpha1.ModelArtifact) []corev1.Container {
	containers := []corev1.Container{modelPackContainer(artifact, false)}
	if artifact.Spec.Draft != nil {
		containers = append(containers, modelPackContainer(artifact, true))
	}
	return containers
}

func modelPackContainer(artifact *v1alpha1.ModelArtifact, draft bool) corev1.Container {
	name := modelDownloadContainer
	if draft {
		name = modelDownloadContainer + "-draft"
	}

	return corev1.Container{
		Name:            name,
		Image:           modelPackImage,
		ImagePullPolicy: corev1.PullNever,
		Env:             slices.Clone(downloaderEnv),
		Args:            modelPackArgs(artifact, draft),
		VolumeMounts:    []corev1.VolumeMount{modelVolumeMount()},
	}
}

func modelPackArgs(artifact *v1alpha1.ModelArtifact, draft bool) []string {
	outputDir, weightsURL := baseModelPath, artifact.Spec.Base
	if draft && artifact.Spec.Draft != nil {
		outputDir, weightsURL = draftModelPath, *artifact.Spec.Draft
	}

	return []string{
		"fetch",
		"--storage-dir",
		modelPackStorageDir,
		"--output",
		outputDir,
		weightsURL,
		"--patterns",
		"*",
	}
}
