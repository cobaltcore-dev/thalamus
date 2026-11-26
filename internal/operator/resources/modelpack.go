package resources

import (
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	modelPackImage      = "ghcr.io/modelpack/modctl:0.2.3"
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
		Name:         modelDownloadContainer,
		Image:        modelPackImage,
		Env:          slices.Clone(downloaderEnv),
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{modelPackCommand(artifact)},
		VolumeMounts: []corev1.VolumeMount{modelVolumeMount()},
	}
}

func modelPackCommand(artifact *v1alpha1.ModelArtifact) string {
	commands := []string{modelPackFetch(artifact.Spec.Base, baseModelPath)}
	if artifact.Spec.Draft != nil {
		commands = append(commands, modelPackFetch(*artifact.Spec.Draft, draftModelPath))
	}
	return "set -eu; " + strings.Join(commands, "; ")
}

func modelPackFetch(weightsURL, outputDir string) string {
	args := slices.Concat([]string{
		"modctl", "fetch",
		"--storage-dir", modelPackStorageDir,
		"--output", outputDir,
		weightsURL,
	}, modctlPatternArgs())
	return strings.Join(args, " ")
}
