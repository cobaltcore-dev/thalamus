// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Reasons for the Ready condition while a download Job progresses.
const (
	ModelArtifactReasonDownloadPending ModelArtifactReason = "DownloadPending"
	ModelArtifactReasonDownloading     ModelArtifactReason = "Downloading"
	ModelArtifactReasonDownloadFailed  ModelArtifactReason = "DownloadFailed"
)

// ModelArtifactVolume selects the PVC used for prefetching.
type ModelArtifactVolume struct {
	// ClaimName is an existing PVC in the artifact's namespace.
	// +kubebuilder:validation:Optional
	ClaimName string `json:"claimName,omitempty"`
}

// ModelArtifactStatus defines the observed state of an artifact.
type ModelArtifactStatus struct {
	// Conditions report artifact readiness and download progress.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ModelArtifactSpec describes remote model weights that an inference engine can load.
// +kubebuilder:validation:XValidation:rule="!has(self.draft) || self.base != self.draft",message="spec.draft must differ from spec.base"
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type ModelArtifactSpec struct {
	// Base is the remote location of the base model weights.
	// +kubebuilder:validation:XValidation:rule="self.matches('^(hf|modelpack)://.+$')",message="spec.base must be an hf:// or modelpack:// URL"
	Base string `json:"base"`

	// Draft is the optional remote location of the draft model weights.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self.matches('^(hf|modelpack)://.+$')",message="spec.draft must be an hf:// or modelpack:// URL"
	Draft *string `json:"draft,omitempty"`

	// Volume selects a PVC for prefetching. Without one the engine downloads weights.
	// +kubebuilder:validation:Optional
	Volume *ModelArtifactVolume `json:"volume,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=mart,categories=thalamus
// +kubebuilder:printcolumn:name="Base",type="string",JSONPath=".spec.base"
// +kubebuilder:printcolumn:name="Draft",type="string",JSONPath=".spec.draft"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// The "-download" suffix must fit in the 63-character Job name limit.
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 54",message="metadata.name must be at most 54 characters."

// ModelArtifact is the Schema for the modelartifacts API.
type ModelArtifact struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec ModelArtifactSpec `json:"spec"`

	// +optional
	Status ModelArtifactStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true
type ModelArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelArtifact `json:"items"`
}

// BaseRepoID returns the base model identifier without its scheme.
func (a *ModelArtifact) BaseRepoID() string {
	return stripScheme(a.Spec.Base)
}

// DraftRepoID returns the draft model identifier without its scheme,
// or an empty string when no draft is configured.
func (a *ModelArtifact) DraftRepoID() string {
	if a.Spec.Draft == nil {
		return ""
	}
	return stripScheme(*a.Spec.Draft)
}

// stripScheme removes the "<scheme>://" prefix from a weights URL, leaving the
// bare repository identifier shared by every source type.
func stripScheme(url string) string {
	if _, rest, ok := strings.Cut(url, "://"); ok {
		return rest
	}
	return url
}

// UsesPVC reports whether the artifact prefetches to a PVC.
func (a *ModelArtifact) UsesPVC() bool {
	return a != nil && a.Spec.Volume != nil && a.Spec.Volume.ClaimName != ""
}

// IsReady reports whether Ready is true for the current generation.
func (a *ModelArtifact) IsReady() bool {
	if a == nil {
		return false
	}
	condition := meta.FindStatusCondition(a.Status.Conditions, "Ready")
	return condition != nil && condition.Status == metav1.ConditionTrue && condition.ObservedGeneration >= a.Generation
}

// SetReady updates the Ready condition in place.
func (a *ModelArtifact) SetReady(status metav1.ConditionStatus, reason ModelArtifactReason, message string) {
	meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: a.Generation,
	})
}

// DownloadJobName returns the name of the artifact's download Job.
func (a *ModelArtifact) DownloadJobName() string { return a.Name + "-download" }

func init() {
	SchemeBuilder.Register(&ModelArtifact{}, &ModelArtifactList{})
}
