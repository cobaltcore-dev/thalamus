// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// NewModel returns a fully populated Model for use as a text fixture.
func NewModel(name, namespace string) *v1alpha1.Model {
	return &v1alpha1.Model{
		Name: name, Namespace: namespace,
		Spec: v1alpha1.ModelSpec{
			Backend:  v1alpha1.BackendTypeNative,
			Replicas: 1,
			Serving: v1alpha1.ServingSpec{
				Engine: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "engine",
						Image: "test/vllm:latest",
						Args:  []string{"--max-model-len=512"},
						Env:   []corev1.EnvVar{{Name: "EXTRA", Value: "val"}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("6Gi"),
							},
						},
					}},
				},
				Router: &corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "epp",
						Image: "test/epp:latest",
					}},
				},
			},
			Weights: v1alpha1.WeightsSpec{
				Type: v1alpha1.WeightsTypeHF,
				HF: &v1alpha1.HFWeightsSpec{
					RepoID: "arnir0/Tiny-LLM",
					TokenSecret: &corev1.SecretKeySelector{
						Name: "hf-token",
						Key:  "token",
					},
				},
			},
		},
	}
}
