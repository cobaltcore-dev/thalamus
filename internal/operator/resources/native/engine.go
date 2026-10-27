// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/helpers"
)

// engineHTTPPort is the TCP port the engine listens on.
const engineHTTPPort = 8000

// ubuntuNobody is the Ubuntu "nobody" uid/gid.
const ubuntuNobody int64 = 65534

// BuildEngineDeployment returns the desired Deployment for the inference engine.
func BuildEngineDeployment(model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) (*appsv1.Deployment, error) {
	// TODO: defaulted fields can't be cleared from the engine configuration in the Model CR
	podSpec, err := helpers.MergePodSpecs(defaultEnginePodSpec(model), &model.Spec.Serving.Engine)
	if err != nil {
		return nil, err
	}

	// Apply the model path after the merge so user args can't drop it.
	// TODO: append other default arguments
	if model.DetectedEngineType() == v1alpha1.EngineTypeVLLM {
		c := helpers.GetContainerByName(&podSpec, "engine")
		c.Args = append([]string{artifact.BaseRepoID()}, c.Args...)
		if draft := artifact.DraftRepoID(); draft != "" {
			c.Args = append(c.Args, "--spec-model="+draft)
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
				Spec:       podSpec,
			},
		},
	}, nil
}

// defaultEnginePodSpec is the out-of-the-box engine pod spec,
// configuration from spec.serving.engine is merged on top of it.
func defaultEnginePodSpec(model *v1alpha1.Model) *corev1.PodSpec {
	env := []corev1.EnvVar{
		// Point the engine caches at the cache volume instead of $HOME.
		{Name: "HOME", Value: "/cache"},
		{Name: "HF_HOME", Value: "/cache/huggingface"},
		{Name: "XDG_CACHE_HOME", Value: "/cache"},
		// Aliases are not passed to the engine; skip model name validation
		// instead so alias changes do not require an engine restart.
		{Name: "VLLM_SKIP_MODEL_NAME_VALIDATION", Value: "1"},
	}

	container := corev1.Container{
		Name:  "engine",
		Env:   env,
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: engineHTTPPort, Protocol: corev1.ProtocolTCP}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "cache", MountPath: "/cache"},
			{Name: "dshm", MountPath: "/dev/shm"},
			{Name: "tmp", MountPath: "/tmp"},
		},
		SecurityContext: &corev1.SecurityContext{
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
		},
	}
	if model.DetectedEngineType() == v1alpha1.EngineTypeVLLM {
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
		container.LivenessProbe = &corev1.Probe{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
				Port: intstr.FromInt32(engineHTTPPort),
			},
			PeriodSeconds:    10,
			TimeoutSeconds:   5,
			FailureThreshold: 3,
		}
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

	return &corev1.PodSpec{
		Containers: []corev1.Container{container},
		Volumes: []corev1.Volume{
			{Name: "cache", EmptyDir: &corev1.EmptyDirVolumeSource{}},
			{Name: "dshm", EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			{Name: "tmp", EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: new(true),
			RunAsUser:    new(ubuntuNobody),
			RunAsGroup:   new(ubuntuNobody),
			// Chowns cache volume to group nobody; pre-populated PVC
			// stays writable regardless of seeding uid
			FSGroup: new(ubuntuNobody),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
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
