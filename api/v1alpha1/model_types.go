// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=hf
type WeightsType string

const WeightsTypeHF WeightsType = "hf"

type ModelPhase string

const (
	// TODO: set Pending when admission/resource checks block reconciliation (e.g. insufficient GPU quota). Currently unused.
	ModelPhasePending  ModelPhase = "Pending"
	ModelPhaseCreating ModelPhase = "Creating"
	ModelPhaseReady    ModelPhase = "Ready"
	ModelPhaseInactive ModelPhase = "Inactive"
	ModelPhaseFailed   ModelPhase = "Failed"
)

const (
	// The model is ready to serve traffic.
	ModelConditionReady = "Ready"
)

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

// HFWeightsSpec configures model weights sourced from Hugging Face.
type HFWeightsSpec struct {
	// RepoID is the Hugging Face repository ID, e.g. "openai/gpt-oss-120b".
	RepoID string `json:"repoId"`

	// TokenSecret references the key within a Kubernetes secret containing the Hugging Face token.
	// +kubebuilder:validation:Optional
	TokenSecret *corev1.SecretKeySelector `json:"tokenSecret,omitempty"`
}

// WeightsSpec defines where the model weights are sourced from.
// +kubebuilder:validation:XValidation:rule="self.type != 'hf' || has(self.hf)",message="spec.weights.hf is required when type is hf"
type WeightsSpec struct {
	// Type of the weights source.
	Type WeightsType `json:"type"`

	// HF configures weights sourced from Hugging Face.
	// +kubebuilder:validation:Optional
	HF *HFWeightsSpec `json:"hf,omitempty"`
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

	// Weights defines where the model weights are sourced from.
	Weights WeightsSpec `json:"weights"`

	// Aliases are the model names under which this model is exposed.
	// When empty, the base model name from the weights is used.
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

// ModelStatus defines the observed state of a Model.
type ModelStatus struct {
	// EngineType is the detected inference engine type, derived from the engine image.
	// +kubebuilder:validation:Optional
	EngineType EngineType `json:"engineType,omitempty"`

	// EPPType is the detected EPP type, derived from the EPP image.
	// +kubebuilder:validation:Optional
	EPPType EPPType `json:"eppType,omitempty"`

	// Phase summarizes the current lifecycle state of the model.
	// +kubebuilder:validation:Optional
	Phase ModelPhase `json:"phase,omitempty"`

	// Conditions represent the latest observations of the model's state.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mdl,categories=thalamus
// +kubebuilder:printcolumn:name="Backend",type="string",JSONPath=".spec.backend"
// +kubebuilder:printcolumn:name="Engine",type="string",JSONPath=".status.engineType"
// +kubebuilder:printcolumn:name="EPP",type="string",JSONPath=".status.eppType"
// +kubebuilder:printcolumn:name="Weights",type="string",JSONPath=".spec.weights.type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
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

// +kubebuilder:object:root=true

// BaseModelName returns the model identifier from the weights configuration.
func (m *Model) BaseModelName() string {
	if m.Spec.Weights.Type == WeightsTypeHF && m.Spec.Weights.HF != nil {
		return m.Spec.Weights.HF.RepoID
	}
	return ""
}

// ModelNames returns the model identifiers this Model exposes.
// It falls back to the base model name when no aliases are set.
func (m *Model) ModelNames() []string {
	if len(m.Spec.Aliases) > 0 {
		return m.Spec.Aliases
	}
	return []string{m.BaseModelName()}
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
