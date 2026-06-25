// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

func TestParseEPPType(t *testing.T) {
	tests := []struct {
		name     string
		epp      *v1alpha1.EPPSpec
		expected v1alpha1.EPPType
	}{
		{
			name:     "llm-d image",
			epp:      &v1alpha1.EPPSpec{Image: "ghcr.io/llm-d/llm-d-inference-scheduler:v0.8.0"},
			expected: v1alpha1.EPPTypeLLMD,
		},
		{
			name:     "llm-d with different registry",
			epp:      &v1alpha1.EPPSpec{Image: "registry.example.com/llm-d/scheduler:latest"},
			expected: v1alpha1.EPPTypeLLMD,
		},
		{
			name:     "unknown image",
			epp:      &v1alpha1.EPPSpec{Image: "some-other-epp:v1.0.0"},
			expected: v1alpha1.EPPTypeUnknown,
		},
		{
			name:     "nil epp",
			epp:      nil,
			expected: v1alpha1.EPPTypeUnknown,
		},
		{
			name:     "empty image",
			epp:      &v1alpha1.EPPSpec{Image: ""},
			expected: v1alpha1.EPPTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEPPType(tt.epp)
			if got != tt.expected {
				t.Errorf("parseEPPType(%v) = %q, want %q", tt.epp, got, tt.expected)
			}
		})
	}
}
