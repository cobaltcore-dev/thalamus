// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The model is ready to serve traffic.
	ModelConditionReady = "Ready"
	// The referenced artifact exists, is not terminating, and is protected.
	ModelConditionArtifactReady = "ArtifactReady"
)

// ModelReason identifies the reason for a Model's Ready condition.
type ModelReason string

const (
	ModelReasonReady                     ModelReason = "Ready"
	ModelReasonEngineNotReady            ModelReason = "EngineNotReady"
	ModelReasonEngineDeploymentFailed    ModelReason = "EngineDeploymentFailed"
	ModelReasonEPPNotReady               ModelReason = "EPPNotReady"
	ModelReasonEPPDeploymentFailed       ModelReason = "EPPDeploymentFailed"
	ModelReasonNoReplicasDesired         ModelReason = "NoReplicasDesired"
	ModelReasonInferencePoolNotAccepted  ModelReason = "InferencePoolNotAccepted"
	ModelReasonInferencePoolRejected     ModelReason = "InferencePoolRejected"
	ModelReasonHTTPRouteNotAccepted      ModelReason = "HTTPRouteNotAccepted"
	ModelReasonHTTPRouteRejected         ModelReason = "HTTPRouteRejected"
	ModelReasonHTTPRoutePartiallyInvalid ModelReason = "HTTPRoutePartiallyInvalid"
	ModelReasonReconcileError            ModelReason = "ReconcileError"
)

// ModelArtifactReason identifies the reason for a Model's ArtifactReady condition.
type ModelArtifactReason string

const (
	ModelArtifactReasonReady          ModelArtifactReason = "ArtifactReady"
	ModelArtifactReasonNotReady       ModelArtifactReason = "ArtifactNotReady"
	ModelArtifactReasonNotFound       ModelArtifactReason = "ArtifactNotFound"
	ModelArtifactReasonTerminating    ModelArtifactReason = "ArtifactTerminating"
	ModelArtifactReasonReconcileError ModelArtifactReason = "ReconcileError"
)

// +kubebuilder:validation:Enum=native;kserve;kaito
type BackendType string

const (
	BackendTypeNative BackendType = "native"
	BackendTypeKServe BackendType = "kserve"
	BackendTypeKAITO  BackendType = "kaito"
)

// +kubebuilder:validation:Enum=vllm;unknown
type EngineType string

const (
	EngineTypeVLLM    EngineType = "vllm"
	EngineTypeUnknown EngineType = "unknown"
)

// +kubebuilder:validation:Enum=llm-d;unknown
type EPPType string

const (
	EPPTypeLLMD    EPPType = "llm-d"
	EPPTypeUnknown EPPType = "unknown"
)

// ModelArtifactReference identifies a ModelArtifact in the same namespace.
type ModelArtifactReference struct {
	// Name is the name of the referenced ModelArtifact.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// ServingSpec defines the serving configuration for the model instance.
type ServingSpec struct {
	// Engine is the pod spec for the inference engine. The container named
	// 'engine' is the main container and must set an image. Containers with
	// other names are added as sidecars.
	Engine corev1.PodSpec `json:"engine"`

	// Router is the pod spec for the entry point. The container named
	// 'router' is the main container and must set an image. Containers with
	// other names are added as sidecars.
	Router corev1.PodSpec `json:"router"`
}

// ModelSpec defines the configuration for the model instance.
// +kubebuilder:validation:XValidation:rule="self.backend == 'native'",message="spec.backend must be native"
// +kubebuilder:validation:XValidation:rule="self.serving.engine.containers.exists(c, c.name == 'engine' && c.image.size() > 0)",message="spec.serving.engine.containers requires a container named 'engine' with an image"
// +kubebuilder:validation:XValidation:rule="self.serving.router.containers.exists(c, c.name == 'router' && c.image.size() > 0)",message="spec.serving.router.containers requires a container named 'router' with an image"
// +kubebuilder:validation:XValidation:rule="!has(self.serving.engine.restartPolicy) || self.serving.engine.restartPolicy == 'Always'",message="spec.serving.engine.restartPolicy must be 'Always' (Deployments do not support other policies)"
// +kubebuilder:validation:XValidation:rule="!has(self.serving.router.restartPolicy) || self.serving.router.restartPolicy == 'Always'",message="spec.serving.router.restartPolicy must be 'Always' (Deployments do not support other policies)"
type ModelSpec struct {
	// Backend selects which inference backend manages this model's resources.
	// Defaults to "native".
	// +kubebuilder:default=native
	// +kubebuilder:validation:Optional
	Backend BackendType `json:"backend,omitempty"`

	// Replicas defines the number of engine replicas to deploy for this model.
	// Defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`

	// Serving defines how the model is served.
	Serving ServingSpec `json:"serving"`

	// ArtifactRef references the ModelArtifact that provides this model's weights.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.artifactRef is immutable"
	ArtifactRef ModelArtifactReference `json:"artifactRef"`

	// Aliases are the model names under which this model is exposed.
	// When empty, the base model name from the artifact is used.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=5
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=4096
	// +kubebuilder:validation:items:Pattern=`^[!-~]+([\t ]?[!-~]+)*$`
	// +listType=set
	Aliases []string `json:"aliases,omitempty"`
	// The validation pattern above mirrors Gateway API's HTTPHeaderMatch contract:
	// https://github.com/kubernetes-sigs/gateway-api/blob/v1.5.1/apis/v1/httproute_types.go#L625-L635
	// MaxItems is 5: HTTPRouteRule allows 64 matches and each name uses 8
	// (5*8=40), leaving headroom for future routes.
}

// Identifier is a client-facing name under which a Model is exposed.
type Identifier struct {
	// Name is the model identifier clients use.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// OwnedBy is the organization that owns the underlying model.
	// +kubebuilder:validation:MinLength=1
	OwnedBy string `json:"ownedBy"`
}

// ModelStatus defines the observed state of a Model.
type ModelStatus struct {
	// EngineType is the detected inference engine type, derived from the engine image.
	// +kubebuilder:validation:Optional
	EngineType EngineType `json:"engineType,omitempty"`

	// EPPType is the detected EPP type, derived from the EPP image.
	// +kubebuilder:validation:Optional
	EPPType EPPType `json:"eppType,omitempty"`

	// Identifiers are the model identifiers this Model exposes.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	Identifiers []Identifier `json:"identifiers,omitempty"`

	// Conditions represent the latest observations of the model's state.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mdl,categories=thalamus
// +kubebuilder:printcolumn:name="Backend",type="string",JSONPath=".spec.backend"
// +kubebuilder:printcolumn:name="Engine",type="string",JSONPath=".status.engineType"
// +kubebuilder:printcolumn:name="EPP",type="string",JSONPath=".status.eppType"
// +kubebuilder:printcolumn:name="Artifact",type="string",JSONPath=".spec.artifactRef.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// Service names must be DNS-1035 labels.
// +kubebuilder:validation:XValidation:rule="self.metadata.name.matches('^[a-z]([-a-z0-9]*[a-z0-9])?$')",message="metadata.name must be a DNS-1035 label (lowercase alphanumerics and '-', starting with a letter)."
// The "-engine" suffix must fit in the 63-character limit.
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 56",message="metadata.name must be at most 56 characters."

// Model is the Schema for the models API.
type Model struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of the Model.
	// +required
	Spec ModelSpec `json:"spec"`

	// status defines the observed state of the Model.
	// +optional
	Status ModelStatus `json:"status,omitempty,omitzero"`
}

// IsReady reports whether the Model is ready to serve traffic.
func (m *Model) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ModelConditionReady)
}

// SetReady updates the Model's Ready condition in place without patching the status.
func (m *Model) SetReady(status metav1.ConditionStatus, reason ModelReason, message string) {
	meta.SetStatusCondition(&m.Status.Conditions, metav1.Condition{
		Type:               ModelConditionReady,
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: m.Generation,
	})
}

// Identifiers returns the model identifiers exposed by aliases, or the base
// repository ID when no aliases are set.
func Identifiers(aliases []string, baseRepoID string) []Identifier {
	names := aliases
	if len(names) == 0 {
		if baseRepoID == "" {
			return nil
		}
		names = []string{baseRepoID}
	}

	ownedBy := baseRepoID
	if before, _, found := strings.Cut(baseRepoID, "/"); found {
		ownedBy = before
	}

	identifiers := make([]Identifier, len(names))
	for i, name := range names {
		identifiers[i] = Identifier{Name: name, OwnedBy: ownedBy}
	}
	return identifiers
}

// EngineName returns the Kubernetes resource name for the model's inference engine.
func (m *Model) EngineName() string { return m.Name + "-engine" }

// EPPName returns the Kubernetes resource name for the model's Endpoint Picker Proxy.
func (m *Model) EPPName() string { return m.Name + "-epp" }

// DetectedEngineType derives the engine type from the engine container image.
func (m *Model) DetectedEngineType() EngineType {
	for _, c := range m.Spec.Serving.Engine.Containers {
		if c.Name == "engine" && strings.Contains(c.Image, "vllm") {
			return EngineTypeVLLM
		}
	}
	return EngineTypeUnknown
}

// DetectedEPPType derives the EPP type from the EPP container image.
func (m *Model) DetectedEPPType() EPPType {
	for _, c := range m.Spec.Serving.Router.Containers {
		if c.Name == "router" && strings.Contains(c.Image, "llm-d") {
			return EPPTypeLLMD
		}
	}
	return EPPTypeUnknown
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model.
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
