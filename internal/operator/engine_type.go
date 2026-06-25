// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"strings"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// engineTypeRules maps EngineType values to the image substrings that identify them.
var engineTypeRules = []struct {
	typ      v1alpha1.EngineType
	patterns []string
}{
	{v1alpha1.EngineTypeVLLM, []string{"vllm"}},
}

func parseEngineType(image string) v1alpha1.EngineType {
	for _, rule := range engineTypeRules {
		for _, p := range rule.patterns {
			if strings.Contains(image, p) {
				return rule.typ
			}
		}
	}
	return v1alpha1.EngineTypeUnknown
}
