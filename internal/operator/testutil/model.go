// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// DefaultModelBase is the base weights location used by test fixtures.
const DefaultModelBase = "hf://arnir0/Tiny-LLM"

// NewModel returns a fully populated Model for use as a test fixture.
func NewModel(name, namespace string) *v1alpha1.Model {
	return &v1alpha1.Model{
		Name: name, Namespace: namespace,
		UID: types.UID(namespace + "/" + name),
		Spec: v1alpha1.ModelSpec{
			Backend:  v1alpha1.BackendTypeNative,
			Replicas: 1,
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
			ArtifactRef: v1alpha1.ModelArtifactReference{Name: name + "-artifact"},
		},
	}
}

// NewModelArtifact returns a ModelArtifact fixture for the given name.
func NewModelArtifact(name, namespace string) *v1alpha1.ModelArtifact {
	return &v1alpha1.ModelArtifact{
		Name: name, Namespace: namespace,
		Spec: v1alpha1.ModelArtifactSpec{Base: DefaultModelBase},
	}
}

// NewModelWithArtifact returns a Model and its default ModelArtifact fixture.
func NewModelWithArtifact(name, namespace string) (*v1alpha1.Model, *v1alpha1.ModelArtifact) {
	model := NewModel(name, namespace)
	return model, NewModelArtifact(model.Spec.ArtifactRef.Name, namespace)
}
