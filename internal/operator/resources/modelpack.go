package resources

import (
	"fmt"
	"slices"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// downloaderEnv is required for modctl to run properly in rootless environment
var downloaderEnv = []corev1.EnvVar{
	{Name: "USER", Value: fmt.Sprint(ubuntuNobody)},
	{Name: "HOME", Value: "/models/.home"},
}

// BuildModelPackJob creates a new job resource which must download a ModelPack
// image of model weights once
func BuildModelPackJob(artifact *v1alpha1.ModelArtifact) *v1.Job {
	containers := []corev1.Container{buildModelPackContainer(artifact, false)}
	if artifact.Spec.Draft != nil {
		containers = append(containers, buildModelPackContainer(artifact, true))
	}

	return &v1.Job{
		Name:      artifact.Name + "-modelpack-downloader",
		Namespace: artifact.Namespace,
		Spec: v1.JobSpec{
			BackoffLimit: new(int32(4)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: containers,
					Volumes: []corev1.Volume{
						{
							Name: "model-cache",
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								// fixme: use user-supplied claim name from the artifact
								ClaimName: artifact.Name,
							},
						},
					},
				},
			},
		},
	}
}

func buildModelPackContainer(artifact *v1alpha1.ModelArtifact, draft bool) corev1.Container {
	name := "base"
	if draft {
		name = "draft"
	}

	return corev1.Container{
		Name:            name,
		Image:           "modctl:latest",
		ImagePullPolicy: corev1.PullNever,
		Env:             slices.Clone(downloaderEnv),
		Args:            buildModelPackArgs(artifact, draft),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "model-cache",
				MountPath: "/models",
			},
		},
		RestartPolicy: new(corev1.ContainerRestartPolicyNever),
	}
}

func buildModelPackArgs(artifact *v1alpha1.ModelArtifact, draft bool) []string {
	outputDir, weightsURL := "/models/base", artifact.Spec.Base
	if draft && artifact.Spec.Draft != nil {
		outputDir, weightsURL = "/models/draft", *artifact.Spec.Draft
	}

	return []string{
		"fetch",
		"--storage-dir",
		"/models/.modctl",
		"--output",
		outputDir,
		weightsURL,
		"--patterns",
		"*",
	}
}
