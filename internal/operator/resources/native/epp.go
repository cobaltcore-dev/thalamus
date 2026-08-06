// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	_ "embed"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
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
)

//go:embed epp_config.yaml
var eppConfig string

// BuildEPPServiceAccount returns the ServiceAccount for the EPP.
func BuildEPPServiceAccount(model *v1alpha1.Model) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
	}
}

// BuildEPPRole returns the Role granting the EPP the access it needs.
func BuildEPPRole(model *v1alpha1.Model) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"inference.networking.k8s.io"},
				Resources: []string{"inferencepools"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"inference.networking.x-k8s.io"},
				Resources: []string{"inferencemodelrewrites", "inferenceobjectives"},
				Verbs:     []string{"get", "watch", "list"},
			},
		},
	}
}

// BuildEPPRoleBinding binds the EPP Role to its ServiceAccount.
func BuildEPPRoleBinding(model *v1alpha1.Model) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     model.EPPName(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      model.EPPName(),
				Namespace: model.Namespace,
			},
		},
	}
}

// BuildEPPConfigMap returns the ConfigMap carrying the EPP plugin configuration.
func BuildEPPConfigMap(model *v1alpha1.Model) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
		Data: map[string]string{
			eppConfigKey: eppConfig,
		},
	}
}

// BuildEPPDeployment returns the Deployment for the Endpoint Picker Proxy.
func BuildEPPDeployment(model *v1alpha1.Model) *appsv1.Deployment {
	epp := model.Spec.Serving.EPP

	args := []string{
		"--pool-name", model.EngineName(),
		"--pool-namespace", model.Namespace,
		"--pool-group", "inference.networking.k8s.io",
		"--zap-encoder", "json",
		"--config-file", "/config/" + eppConfigKey,
		"--metrics-endpoint-auth=false",
	}
	args = append(args, epp.Args...)

	eppService := eppHealthService
	container := corev1.Container{
		Name:            "epp",
		Image:           epp.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            args,
		Env: append([]corev1.EnvVar{
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
		}, epp.Env...),
		Ports: []corev1.ContainerPort{
			{Name: eppGRPCExtProcPortName, ContainerPort: eppGRPCExtProcPort, Protocol: corev1.ProtocolTCP},
			{Name: eppGRPCHealthPortName, ContainerPort: eppGRPCHealthPort, Protocol: corev1.ProtocolTCP},
			{Name: eppMetricsPortName, ContainerPort: eppMetricsPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config", MountPath: "/config", ReadOnly: true},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				GRPC: &corev1.GRPCAction{Port: eppGRPCHealthPort, Service: &eppService},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				GRPC: &corev1.GRPCAction{Port: eppGRPCHealthPort, Service: &eppService},
			},
			PeriodSeconds: 2,
		},
	}

	if epp.Resources != nil {
		container.Resources = *epp.Resources
	}

	recreate := appsv1.RecreateDeploymentStrategyType
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Strategy: appsv1.DeploymentStrategy{Type: recreate},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"thalamus.cloud/epp": model.EPPName()},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"thalamus.cloud/epp": model.EPPName()},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            model.EPPName(),
					TerminationGracePeriodSeconds: ptr.To[int64](130),
					Containers:                    []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: model.EPPName()},
								},
							},
						},
					},
				},
			},
		},
	}
}

// BuildEPPService returns the Service exposing the EPP's gRPC and metrics ports.
func BuildEPPService(model *v1alpha1.Model) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EPPName(),
			Namespace: model.Namespace,
		},
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
