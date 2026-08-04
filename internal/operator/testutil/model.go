// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// NewModel returns a fully populated Model for use as a text fixture.
func NewModel(name, namespace string) *v1alpha1.Model {
	return &v1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.ModelSpec{
			Backend: v1alpha1.BackendTypeNative,
			Serving: v1alpha1.ServingSpec{
				Engine: v1alpha1.EngineSpec{
					Image: "test/engine:latest",
					Args:  []string{"--max-model-len=512"},
					Env:   []corev1.EnvVar{{Name: "EXTRA", Value: "val"}},
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("6Gi"),
						},
					},
				},
				EPP: &v1alpha1.EPPSpec{
					Image: "test/epp:latest",
					Args:  []string{},
					Env:   []corev1.EnvVar{},
				},
			},
			Weights: v1alpha1.WeightsSpec{
				Type: v1alpha1.WeightsTypeHF,
				HF: &v1alpha1.HFWeightsSpec{
					RepoID: "arnir0/Tiny-LLM",
					TokenSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "hf-token"},
						Key:                  "token",
					},
				},
			},
		},
	}
}
