// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

func TestParseEngineType(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected v1alpha1.EngineType
	}{
		{
			name:     "vllm image",
			image:    "vllm/vllm-openai:v0.23.0",
			expected: v1alpha1.EngineTypeVLLM,
		},
		{
			name:     "vllm cpu image",
			image:    "vllm/vllm-openai-cpu:v0.23.0",
			expected: v1alpha1.EngineTypeVLLM,
		},
		{
			name:     "vllm with registry prefix",
			image:    "registry.example.com/vllm/vllm-openai:latest",
			expected: v1alpha1.EngineTypeVLLM,
		},
		{
			name:     "unknown image",
			image:    "some-other-engine:v1.0.0",
			expected: v1alpha1.EngineTypeUnknown,
		},
		{
			name:     "empty image",
			image:    "",
			expected: v1alpha1.EngineTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEngineType(tt.image)
			if got != tt.expected {
				t.Errorf("parseEngineType(%q) = %q, want %q", tt.image, got, tt.expected)
			}
		})
	}
}
