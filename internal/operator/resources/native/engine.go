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

// BuildEngineDeployment returns the desired Deployment for the vLLM inference engine.
func BuildEngineDeployment(model *v1alpha1.Model) *appsv1.Deployment {
	engine := model.Spec.Serving.Engine

	command := []string{"vllm", "serve"}
	args := []string{}
	env := engine.Env

	if model.Spec.Weights.Type == v1alpha1.WeightsTypeHF && model.Spec.Weights.HF != nil {
		hf := model.Spec.Weights.HF
		args = append(args, hf.RepoID, "--served-model-name="+hf.RepoID)
		env = append([]corev1.EnvVar{{
			Name:      "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &hf.TokenSecret},
		}}, env...)
	}

	args = append(args, engine.Args...)

	container := corev1.Container{
		Name:            "engine",
		Image:           engine.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         command,
		Args:            args,
		Env:             env,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: engineHTTPPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "vllm-cache", MountPath: "/root/.cache"},
			{Name: "dshm", MountPath: "/dev/shm"},
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/models",
					Port: intstr.FromInt32(engineHTTPPort),
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    360,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt32(engineHTTPPort),
				},
			},
			PeriodSeconds:    10,
			TimeoutSeconds:   5,
			FailureThreshold: 3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/models",
					Port: intstr.FromInt32(engineHTTPPort),
				},
			},
			PeriodSeconds:    5,
			TimeoutSeconds:   2,
			FailureThreshold: 3,
		},
	}

	if engine.Resources != nil {
		container.Resources = *engine.Resources
	}

	cacheVolumeSource := corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	if engine.Cache != nil {
		cacheVolumeSource = *engine.Cache
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
		Volumes: []corev1.Volume{
			{
				Name:         "vllm-cache",
				VolumeSource: cacheVolumeSource,
			},
			{
				Name: "dshm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory},
				},
			},
		},
	}

	if model.Spec.Scheduling != nil {
		podSpec.NodeSelector = model.Spec.Scheduling.NodeSelector
	}

	labels := map[string]string{
		"thalamus.cloud/engine": model.EngineName(),
	}

	// TODO: updateStrategy should be a field on the Model CRD.
	// Ideally the operator implements a best-effort rolling strategy: check whether
	// the cluster has enough spare capacity to bring up a new replica before terminating
	// the old one (e.g. by inspecting node allocatable vs pending resource requests),
	// and fall back to Recreate when it does not. For now Recreate is the safe default
	// because LLM pods are large and a second replica will often not fit.
	deploymentStrategy := appsv1.RecreateDeploymentStrategyType

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EngineName(),
			Namespace: model.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &model.Spec.Replicas,
			Strategy: appsv1.DeploymentStrategy{Type: deploymentStrategy},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}
}

// BuildEngineService returns the Service for the vLLM engine.
func BuildEngineService(model *v1alpha1.Model) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EngineName(),
			Namespace: model.Namespace,
		},
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
