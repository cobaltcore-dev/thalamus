// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	_ "embed"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/helpers"
)

const (
	eppConfigKey = "default-plugins.yaml"

	// EPP port names.
	eppGRPCExtProcPortName = "grpc-ext-proc"
	eppGRPCHealthPortName  = "grpc-health"
	eppMetricsPortName     = "metrics"
	eppHTTPMetricsPortName = "http-metrics"

	// EPP port numbers.
	eppGRPCExtProcPort int32 = 9002
	eppGRPCHealthPort  int32 = 9003
	eppMetricsPort     int32 = 9090

	// eppHealthService is the gRPC health probe service name.
	eppHealthService = "inference-extension"

	// distrolessNobody is the distroless "nobody" uid/gid.
	distrolessNobody int64 = 65532
)

//go:embed epp_config.yaml
var eppConfig string

// BuildEPPConfigMap returns the ConfigMap carrying the EPP plugin configuration.
func BuildEPPConfigMap(model *v1alpha1.Model) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		Name:      model.EPPName(),
		Namespace: model.Namespace,
		Data: map[string]string{
			eppConfigKey: eppConfig,
		},
	}
}

// BuildEPPDeployment returns the Deployment for the Endpoint Picker Proxy.
func BuildEPPDeployment(model *v1alpha1.Model, serviceAccountName string) (*appsv1.Deployment, error) {
	// TODO: defaulted fields can't be cleared from the epp configuration in the Model CR
	podSpec, err := helpers.MergePodSpecs(defaultEPPPodSpec(model), &model.Spec.Serving.Router)
	if err != nil {
		return nil, err
	}

	// Pool identity and flags are the contract with the operator-created
	// InferencePool and ConfigMap. They are appended after user args so user
	// args can't drop or override them.
	c := helpers.GetContainerByName(&podSpec, "router")
	c.Args = append(c.Args,
		"--pool-name", model.EngineName(),
		"--pool-namespace", model.Namespace,
		"--pool-group", inferencev1.GroupName,
		"--zap-encoder", "json",
		"--config-file", "/config/"+eppConfigKey,
		"--metrics-endpoint-auth=false",
	)
	// Set after the merge so the Model can't override the router identity or token.
	podSpec.ServiceAccountName = serviceAccountName
	podSpec.AutomountServiceAccountToken = new(true)

	recreate := appsv1.RecreateDeploymentStrategyType
	return &appsv1.Deployment{
		Name:      model.EPPName(),
		Namespace: model.Namespace,
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Strategy: appsv1.DeploymentStrategy{Type: recreate},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"thalamus.cloud/epp": model.EPPName()},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"thalamus.cloud/epp": model.EPPName()},
				},
				Spec: podSpec,
			},
		},
	}, nil
}

// defaultEPPPodSpec is the out-of-the-box EPP pod spec; the user's
// configuration from spec.serving.router is merged on top of it.
func defaultEPPPodSpec(model *v1alpha1.Model) *corev1.PodSpec {
	return &corev1.PodSpec{
		TerminationGracePeriodSeconds: new(int64(130)),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: new(true),
			RunAsUser:    new(distrolessNobody),
			RunAsGroup:   new(distrolessNobody),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		Containers: []corev1.Container{
			{
				Name: "router",
				Env: []corev1.EnvVar{
					{
						Name: "NAMESPACE",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
						},
					},
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{Name: eppGRPCExtProcPortName, ContainerPort: eppGRPCExtProcPort, Protocol: corev1.ProtocolTCP},
					{Name: eppGRPCHealthPortName, ContainerPort: eppGRPCHealthPort, Protocol: corev1.ProtocolTCP},
					{Name: eppMetricsPortName, ContainerPort: eppMetricsPort, Protocol: corev1.ProtocolTCP},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "config", MountPath: "/config", ReadOnly: true},
				},
				LivenessProbe: &corev1.Probe{
					GRPC:                &corev1.GRPCAction{Port: eppGRPCHealthPort, Service: new(eppHealthService)},
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
				},
				ReadinessProbe: &corev1.Probe{
					GRPC:          &corev1.GRPCAction{Port: eppGRPCHealthPort, Service: new(eppHealthService)},
					PeriodSeconds: 2,
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: new(false),
					ReadOnlyRootFilesystem:   new(true),
					RunAsNonRoot:             new(true),
					RunAsUser:                new(distrolessNobody),
					RunAsGroup:               new(distrolessNobody),
					Privileged:               new(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name:      "config",
				ConfigMap: &corev1.ConfigMapVolumeSource{Name: model.EPPName()},
			},
		},
	}
}

// BuildEPPService returns the Service exposing the EPP's gRPC and metrics ports.
func BuildEPPService(model *v1alpha1.Model) *corev1.Service {
	return &corev1.Service{
		Name:      model.EPPName(),
		Namespace: model.Namespace,
		Labels:    map[string]string{"thalamus.cloud/epp": model.EPPName()},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"thalamus.cloud/epp": model.EPPName()},
			Ports: []corev1.ServicePort{
				{
					Name:       eppGRPCExtProcPortName,
					Port:       eppGRPCExtProcPort,
					TargetPort: intstr.FromInt32(eppGRPCExtProcPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       eppHTTPMetricsPortName,
					Port:       eppMetricsPort,
					TargetPort: intstr.FromInt32(eppMetricsPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
