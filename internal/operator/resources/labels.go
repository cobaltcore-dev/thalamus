// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// Label keys the operator sets on the workloads and services it creates.
const (
	engineLabel = "thalamus.cloud/engine"
	eppLabel    = "thalamus.cloud/epp"
)

// engineLabels returns the labels identifying the model's engine pods.
func engineLabels(model *v1alpha1.Model) map[string]string {
	return map[string]string{engineLabel: model.EngineName()}
}

// eppLabels returns the labels identifying the model's router pods.
func eppLabels(model *v1alpha1.Model) map[string]string {
	return map[string]string{eppLabel: model.EPPName()}
}
