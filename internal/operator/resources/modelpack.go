package resources

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

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

// modelPackContainer downloads base and optional draft weights with modctl in a
// single container, chaining the fetches with ";" like the Hugging Face path.
func modelPackContainer(artifact *v1alpha1.ModelArtifact) corev1.Container {
	return corev1.Container{
		Name:            modelDownloadContainer,
		Image:           modelPackImage,
		ImagePullPolicy: corev1.PullNever,
		Env:             slices.Clone(downloaderEnv),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{modelPackCommand(artifact)},
		VolumeMounts:    []corev1.VolumeMount{modelVolumeMount()},
	}
}

func modelPackArgs(artifact *v1alpha1.ModelArtifact, draft bool) []string {
	outputDir, weightsURL := baseModelPath, artifact.Spec.Base
	if draft && artifact.Spec.Draft != nil {
		outputDir, weightsURL = draftModelPath, *artifact.Spec.Draft
	}

	return slices.Concat([]string{
		"fetch",
		"--storage-dir",
		modelPackStorageDir,
		"--output",
		outputDir,
		weightsURL,
	}, modctlPatternArgs())
}
