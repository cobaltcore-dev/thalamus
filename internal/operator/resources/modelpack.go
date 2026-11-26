package resources

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	modelPackImage      = "ghcr.io/modelpack/modctl:0.2.3"
	modelPackStorageDir = modelVolumeMountPath + "/.modctl"
)

// modelPackContainer downloads base and optional draft weights with modctl in a
// single container, chaining the fetches with ";" like the Hugging Face path.
func modelPackContainer(artifact *v1alpha1.ModelArtifact) corev1.Container {
	return corev1.Container{
		Name:         modelDownloadContainer,
		Image:        modelPackImage,
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{modelPackCommand(artifact)},
		VolumeMounts: []corev1.VolumeMount{modelVolumeMount()},
	}
}

func modelPackCommand(artifact *v1alpha1.ModelArtifact) string {
	commands := []string{modelPackFetch(artifact.BaseRepoID(), baseModelPath)}
	if draft := artifact.DraftRepoID(); draft != "" {
		commands = append(commands, modelPackFetch(draft, draftModelPath))
	}
	return "set -eu; " + strings.Join(commands, "; ")
}

func modelPackFetch(repoID, outputDir string) string {
	args := slices.Concat([]string{
		"modctl", "fetch",
		"--storage-dir", modelPackStorageDir,
		"--log-dir", modelPackStorageDir + "/logs",
		"--output", outputDir,
		repoID,
	}, modctlPatternArgs())
	return strings.Join(args, " ")
}
