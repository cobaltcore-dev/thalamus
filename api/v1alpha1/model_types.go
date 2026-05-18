// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=ceph
type WeightsType string

const WeightsTypeCeph WeightsType = "ceph"

// +kubebuilder:validation:Enum=vllm;tensorrt
type ServingEngine string

const (
	ServingEngineVLLM      ServingEngine = "vllm"
	ServingEngineTensorRT  ServingEngine = "tensorrt"
)

// +kubebuilder:validation:Enum=cortex;grove
type ModelScheduler string

const (
	ModelSchedulerCortex ModelScheduler = "cortex"
	ModelSchedulerGrove  ModelScheduler = "grove"
)

type ModelPhase string

const (
	ModelPhasePending   ModelPhase = "Pending"
	ModelPhaseDeploying ModelPhase = "Deploying"
	ModelPhaseReady     ModelPhase = "Ready"
	ModelPhaseFailed    ModelPhase = "Failed"
)

const ModelConditionReady = "Ready"

type ImageSpec struct {
	// +kubebuilder:validation:Required
	Ref string `json:"ref"`
}

type CephWeightsSpec struct {
	// +kubebuilder:validation:Required
	Pool string `json:"pool"`
	// +kubebuilder:validation:Required
	Path string `json:"path"`
}

type WeightsSpec struct {
	// +kubebuilder:validation:Required
	Type WeightsType      `json:"type"`
	Ceph *CephWeightsSpec `json:"ceph,omitempty"`
}

type ServingResourcesSpec struct {
	GPU    int               `json:"gpu,omitempty"`
	Memory resource.Quantity `json:"memory,omitempty"`
}

type ServingSpec struct {
	// +kubebuilder:validation:Required
	Engine    ServingEngine         `json:"engine"`
	// Args are passed directly to the inference engine container as CLI arguments.
	Args      []string              `json:"args,omitempty"`
	Resources *ServingResourcesSpec `json:"resources,omitempty"`
}

type CortexSchedulingSpec struct {
	// +kubebuilder:validation:Required
	NodePool      string            `json:"nodePool"`
	ResourceClass string            `json:"resourceClass,omitempty"`
	NodeSelector  map[string]string `json:"nodeSelector,omitempty"`
}

type GroveSchedulingSpec struct {
	// +kubebuilder:validation:Required
	NodePool     string            `json:"nodePool"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

type SchedulingSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:default=cortex
	Scheduler ModelScheduler        `json:"scheduler"`
	Cortex    *CortexSchedulingSpec `json:"cortex,omitempty"`
	Grove     *GroveSchedulingSpec  `json:"grove,omitempty"`
}

type AutoscalingMetricsSpec struct {
	TTFTp95Ms            int `json:"ttftP95Ms,omitempty"`
	QueueDepthPerReplica int `json:"queueDepthPerReplica,omitempty"`
}

type AutoscalingSpec struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	MinReplicas int `json:"minReplicas"`
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int                     `json:"maxReplicas"`
	Metrics     *AutoscalingMetricsSpec `json:"metrics,omitempty"`
}

type AccessSpec struct {
	// +kubebuilder:validation:Minimum=0
	RateLimitRpm   int      `json:"rateLimitRpm,omitempty"`
	AllowedTenants []string `json:"allowedTenants,omitempty"`
}

type ModelSpec struct {
	DisplayName string        `json:"displayName,omitempty"`
	// +kubebuilder:validation:Required
	Image       ImageSpec     `json:"image"`
	// +kubebuilder:validation:Required
	Weights     WeightsSpec   `json:"weights"`
	// +kubebuilder:validation:Required
	Serving     ServingSpec   `json:"serving"`
	Scheduling  *SchedulingSpec  `json:"scheduling,omitempty"`
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
	Access      *AccessSpec      `json:"access,omitempty"`
}

type ModelStatus struct {
	Phase         ModelPhase       `json:"phase,omitempty"`
	ReadyReplicas int              `json:"readyReplicas,omitempty"`
	Conditions    []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mdl,categories=thalamus
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Engine",type="string",JSONPath=".spec.serving.engine"
// +kubebuilder:printcolumn:name="GPU",type="integer",JSONPath=".spec.serving.resources.gpu"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".spec.autoscaling.minReplicas",priority=0
// +kubebuilder:printcolumn:name="Max",type="integer",JSONPath=".spec.autoscaling.maxReplicas"
// +kubebuilder:printcolumn:name="Scheduler",type="string",JSONPath=".spec.scheduling.scheduler"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Model is the Schema for the models API.
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ModelSpec   `json:"spec"`
	// +optional
	Status ModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
