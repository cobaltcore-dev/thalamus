// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// engineHTTPPort is the TCP port the vLLM engine listens on.
const engineHTTPPort = 8000

// ubuntuNobody is the Ubuntu "nobody" uid/gid.
const ubuntuNobody int64 = 65534

// BuildEngineDeployment returns the desired Deployment for the inference engine.
func BuildEngineDeployment(model *v1alpha1.Model) *appsv1.Deployment {
	podSpec := model.Spec.Serving.Engine.DeepCopy()
	container := defaultContainer(podSpec, "engine", "")
	isVLLM := model.DetectedEngineType() == v1alpha1.EngineTypeVLLM

	if isVLLM && container.Command == nil {
		container.Command = []string{"vllm", "serve"}
	}
	if model.Spec.Weights.Type == v1alpha1.WeightsTypeHF && model.Spec.Weights.HF != nil {
		hf := model.Spec.Weights.HF
		if isVLLM {
			container.Args = append([]string{hf.RepoID}, container.Args...)
			addArgIfAbsent(container, "--served-model-name="+hf.RepoID)
		}
		if hf.TokenSecret != nil {
			addEnvIfAbsent(container, corev1.EnvVar{
				Name:      "HF_TOKEN",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: hf.TokenSecret},
			})
		}
	}

	// Root filesystem read-only; process runs as uid nobody
	// Point caches at /cache instead of images $HOME (/root)
	// Appended last so user env can't override the redirection
	for _, e := range []corev1.EnvVar{
		{Name: "HOME", Value: "/cache"},
		{Name: "HF_HOME", Value: "/cache/huggingface"},
		{Name: "XDG_CACHE_HOME", Value: "/cache"},
	} {
		addEnvIfAbsent(container, e)
	}

	if container.SecurityContext == nil {
		container.SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: new(false),
			ReadOnlyRootFilesystem:   new(true),
			RunAsNonRoot:             new(true),
			RunAsUser:                new(ubuntuNobody),
			RunAsGroup:               new(ubuntuNobody),
			Privileged:               new(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		}
	}
	if isVLLM && container.StartupProbe == nil {
		container.StartupProbe = &corev1.Probe{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/v1/models",
				Port: intstr.FromInt32(engineHTTPPort),
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    360,
		}
	}
	if isVLLM && container.LivenessProbe == nil {
		container.LivenessProbe = &corev1.Probe{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
				Port: intstr.FromInt32(engineHTTPPort),
			},
			PeriodSeconds:    10,
			TimeoutSeconds:   5,
			FailureThreshold: 3,
		}
	}
	if isVLLM && container.ReadinessProbe == nil {
		container.ReadinessProbe = &corev1.Probe{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/v1/models",
				Port: intstr.FromInt32(engineHTTPPort),
			},
			PeriodSeconds:    5,
			TimeoutSeconds:   2,
			FailureThreshold: 3,
		}
	}
	addPortIfAbsent(container, corev1.ContainerPort{
		Name: "http", ContainerPort: engineHTTPPort, Protocol: corev1.ProtocolTCP,
	})

	addMountIfAbsent(container, corev1.VolumeMount{Name: "cache", MountPath: "/cache"})
	addMountIfAbsent(container, corev1.VolumeMount{Name: "dshm", MountPath: "/dev/shm"})
	addMountIfAbsent(container, corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"})
	addVolumeIfAbsent(podSpec, corev1.Volume{Name: "cache", EmptyDir: &corev1.EmptyDirVolumeSource{}})
	addVolumeIfAbsent(podSpec, corev1.Volume{Name: "dshm", EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}})
	addVolumeIfAbsent(podSpec, corev1.Volume{Name: "tmp", EmptyDir: &corev1.EmptyDirVolumeSource{}})

	if podSpec.SecurityContext == nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: new(true),
			RunAsUser:    new(ubuntuNobody),
			RunAsGroup:   new(ubuntuNobody),
			// Chowns cache volume to group nobody; pre-populated PVC
			// stays writable regardless of seeding uid
			FSGroup: new(ubuntuNobody),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		}
	}

	labels := map[string]string{
		"thalamus.cloud/engine": model.EngineName(),
	}

	return &appsv1.Deployment{
		Name:      model.EngineName(),
		Namespace: model.Namespace,
		Spec: appsv1.DeploymentSpec{
			Replicas: &model.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       *podSpec,
			},
		},
	}
}

// BuildEngineService returns the Service for the vLLM engine.
func BuildEngineService(model *v1alpha1.Model) *corev1.Service {
	return &corev1.Service{
		Name:      model.EngineName(),
		Namespace: model.Namespace,
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"thalamus.cloud/engine": model.EngineName()},
			Ports: []corev1.ServicePort{
				{
					Name:       "vllm",
					Port:       engineHTTPPort,
					TargetPort: intstr.FromInt32(engineHTTPPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
