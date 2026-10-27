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
				Router: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "router",
						Image: "test/epp:latest",
					}},
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

// ModelFixture builds a Model and its ModelArtifact test fixture.
type ModelFixture struct {
	model    *v1alpha1.Model
	artifact *v1alpha1.ModelArtifact
}

// Fixture returns a ModelFixture with an artifact named "<name>-artifact",
// base DefaultModelBase, and no draft or status phase.
func Fixture(name, namespace string) *ModelFixture {
	model := NewModel(name, namespace)
	return &ModelFixture{
		model:    model,
		artifact: NewModelArtifact(model.Spec.ArtifactRef.Name, namespace),
	}
}

// Base overrides the artifact's base weights location.
func (f *ModelFixture) Base(base string) *ModelFixture {
	f.artifact.Spec.Base = base
	return f
}

// Draft sets the artifact's optional draft weights location.
func (f *ModelFixture) Draft(draft string) *ModelFixture {
	f.artifact.Spec.Draft = &draft
	return f
}

// Phase sets the model's status phase.
func (f *ModelFixture) Phase(phase v1alpha1.ModelPhase) *ModelFixture {
	f.model.Status.Phase = phase
	return f
}

// Build returns the configured Model and ModelArtifact.
func (f *ModelFixture) Build() (*v1alpha1.Model, *v1alpha1.ModelArtifact) {
	return f.model, f.artifact
}
