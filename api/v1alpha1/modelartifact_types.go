// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelArtifactSpec describes remote model weights that an inference engine can load.
// +kubebuilder:validation:XValidation:rule="!has(self.draft) || self.base != self.draft",message="spec.draft must differ from spec.base"
type ModelArtifactSpec struct {
	// Base is the remote location of the base model weights.
	// +kubebuilder:validation:Pattern=`^hf://.+`
	Base string `json:"base"`

	// Draft is the optional remote location of the draft model weights.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^hf://.+`
	Draft *string `json:"draft,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=mart,categories=thalamus
// +kubebuilder:printcolumn:name="Base",type="string",JSONPath=".spec.base"
// +kubebuilder:printcolumn:name="Draft",type="string",JSONPath=".spec.draft"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ModelArtifact struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec ModelArtifactSpec `json:"spec"`
}

// +kubebuilder:object:root=true
type ModelArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelArtifact `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModelArtifact{}, &ModelArtifactList{})
}
