package resources

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
)

const (
	modelPackImage      = "ghcr.io/modelpack/modctl:0.2.3"
	modelPackStorageDir = modelVolumeMountPath + "/.modctl"
	modelPackLogDir     = modelPackStorageDir + "/logs"
)

func modelPackContainer(name, repoID, outputDir string) corev1.Container {
	return corev1.Container{
		Name:         name,
		Image:        modelPackImage,
		Args:         modelPackFetchArgs(repoID, outputDir),
		VolumeMounts: []corev1.VolumeMount{modelVolumeMount()},
	}
}

func modelPackFetchArgs(repoID, outputDir string) []string {
	return slices.Concat([]string{
		"fetch",
		"--storage-dir", modelPackStorageDir,
		"--log-dir", modelPackLogDir,
		"--output", outputDir,
		repoID,
	}, patternArgs("--patterns"))
}
