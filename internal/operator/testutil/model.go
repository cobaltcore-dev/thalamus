// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	// DefaultModelBase is the base weights location used by test fixtures.
	DefaultModelBase = "hf://arnir0/Tiny-LLM"
	// DefaultModelBaseRepoID is the base model identifier used by test fixtures.
	DefaultModelBaseRepoID = "arnir0/Tiny-LLM"
)

// ModelBuilder builds a Model and its ModelArtifact test fixture.
type ModelBuilder struct {
	model    *v1alpha1.Model
	artifact *v1alpha1.ModelArtifact
}

// Fixture returns a builder for a Model and its artifact.
func Fixture(name, namespace string) *ModelBuilder {
	return &ModelBuilder{
		model: &v1alpha1.Model{
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
		},
		artifact: NewModelArtifact(name+"-artifact", namespace),
	}
}

// Aliases sets the model aliases.
func (b *ModelBuilder) Aliases(aliases ...string) *ModelBuilder {
	b.model.Spec.Aliases = aliases
	return b
}

// Base overrides the artifact's base weights location.
func (b *ModelBuilder) Base(base string) *ModelBuilder {
	b.artifact.Spec.Base = base
	return b
}

// Draft sets the artifact's optional draft weights location.
func (b *ModelBuilder) Draft(draft string) *ModelBuilder {
	b.artifact.Spec.Draft = &draft
	return b
}

// BuildModel returns the configured Model fixture.
func (b *ModelBuilder) BuildModel() *v1alpha1.Model {
	b.model.Status.Identifiers = v1alpha1.Identifiers(b.model.Spec.Aliases, b.artifact.BaseRepoID())
	return b.model
}

// Build returns the configured Model and ModelArtifact.
func (b *ModelBuilder) Build() (*v1alpha1.Model, *v1alpha1.ModelArtifact) {
	return b.BuildModel(), b.artifact
}

// NewModel returns a fully populated Model for use as a test fixture.
func NewModel(name, namespace string) *v1alpha1.Model {
	return Fixture(name, namespace).BuildModel()
}

// NewModelArtifact returns a ModelArtifact fixture for the given name.
func NewModelArtifact(name, namespace string) *v1alpha1.ModelArtifact {
	return &v1alpha1.ModelArtifact{
		Name: name, Namespace: namespace,
		Spec: v1alpha1.ModelArtifactSpec{Base: DefaultModelBase},
	}
}
