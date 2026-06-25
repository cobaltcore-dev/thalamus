// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"strings"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// eppTypeRules maps EPPType values to the image substrings that identify them.
var eppTypeRules = []struct {
	typ      v1alpha1.EPPType
	patterns []string
}{
	{v1alpha1.EPPTypeLLMD, []string{"llm-d"}},
}

func parseEPPType(epp *v1alpha1.EPPSpec) v1alpha1.EPPType {
	if epp == nil {
		return v1alpha1.EPPTypeUnknown
	}
	for _, rule := range eppTypeRules {
		for _, p := range rule.patterns {
			if strings.Contains(epp.Image, p) {
				return rule.typ
			}
		}
	}
	return v1alpha1.EPPTypeUnknown
}
