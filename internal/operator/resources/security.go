// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	corev1 "k8s.io/api/core/v1"
)

// hardenedContainerSecurityContext returns the restricted container security context running as uid.
func hardenedContainerSecurityContext(uid int64) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: new(false),
		ReadOnlyRootFilesystem:   new(true),
		RunAsNonRoot:             new(true),
		RunAsUser:                new(uid),
		RunAsGroup:               new(uid),
		Privileged:               new(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// hardenedPodSecurityContext returns the pod security context running as uid; a nil fsGroup omits it.
func hardenedPodSecurityContext(uid int64, fsGroup *int64) *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: new(true),
		RunAsUser:    new(uid),
		RunAsGroup:   new(uid),
		FSGroup:      fsGroup,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
