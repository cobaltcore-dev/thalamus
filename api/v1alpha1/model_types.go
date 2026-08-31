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
	TokenSecret corev1.SecretKeySelector `json:"tokenSecret"`
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

// EngineSpec configures the inference engine instance.
type EngineSpec struct {
	// Image is the container image for the inference engine.
	Image string `json:"image"`

	// Args are passed directly to the inference engine as CLI arguments.
	// +kubebuilder:validation:Optional
	Args []string `json:"args,omitempty"`

	// Env defines environment variables for the inference engine container.
	// +kubebuilder:validation:Optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources defines the compute resources required by the inference engine.
	// +kubebuilder:validation:Optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Cache defines the volume source for the vLLM cache directory (/root/.cache).
	// Defaults to an emptyDir. Accepts any Kubernetes volume source.
	// +kubebuilder:validation:Optional
	Cache *corev1.VolumeSource `json:"cache,omitempty"`
}

// EPPSpec configures the Endpoint Picker Proxy (EPP).
type EPPSpec struct {
	// Image is the container image for the EPP.
	Image string `json:"image"`

	// Args are passed directly to the EPP as CLI arguments.
	// +kubebuilder:validation:Optional
	Args []string `json:"args,omitempty"`

	// Env defines environment variables for the EPP container.
	// +kubebuilder:validation:Optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources defines the compute resources required by the EPP.
	// +kubebuilder:validation:Optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ServingSpec defines the serving configuration for the model instance.
type ServingSpec struct {
	// Engine configures the inference engine container.
	Engine EngineSpec `json:"engine"`

	// EPP configures the optional Endpoint Picker Proxy.
	// +kubebuilder:validation:Optional
	EPP *EPPSpec `json:"epp,omitempty"`
}

// SchedulingSpec defines scheduling constraints for the model's pods.
type SchedulingSpec struct {
	// NodeSelector constrains the nodes onto which the model's pods may be scheduled.
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// ModelSpec defines the configuration for the model instance.
// +kubebuilder:validation:XValidation:rule="self.backend != 'native' || has(self.serving.epp)",message="spec.serving.epp is required when backend is native"
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

	// Scheduling defines scheduling constraints for the model's pods.
	// +kubebuilder:validation:Optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
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

// EngineName returns the Kubernetes resource name for the model's inference engine.
func (m *Model) EngineName() string { return m.Name + "-engine" }

// EPPName returns the Kubernetes resource name for the model's Endpoint Picker Proxy.
func (m *Model) EPPName() string { return m.Name + "-epp" }

// DetectedEngineType derives the engine type from the engine container image.
func (m *Model) DetectedEngineType() EngineType {
	if strings.Contains(m.Spec.Serving.Engine.Image, "vllm") {
		return EngineTypeVLLM
	}
	return EngineTypeUnknown
}

// DetectedEPPType derives the EPP type from the EPP container image.
func (m *Model) DetectedEPPType() EPPType {
	if m.Spec.Serving.EPP == nil {
		return EPPTypeUnknown
	}
	if strings.Contains(m.Spec.Serving.EPP.Image, "llm-d") {
		return EPPTypeLLMD
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
