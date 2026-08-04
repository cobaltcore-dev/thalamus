// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildInferencePool_NoEPP(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	pool := BuildInferencePool(model)

	if pool.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", pool.Name, model.EngineName())
	}
	if len(pool.Spec.TargetPorts) != 1 || pool.Spec.TargetPorts[0].Number != engineHTTPPort {
		t.Error("unexpected TargetPorts")
	}
	// No EPP configured — EndpointPickerRef must be zero value.
	if pool.Spec.EndpointPickerRef.Name != "" {
		t.Errorf("EndpointPickerRef should be empty without EPP, got %q", pool.Spec.EndpointPickerRef.Name)
	}
}

func TestBuildInferencePool_WithEPP(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: "ghcr.io/llm-d/llm-d-router-endpoint-picker:v0.9.0"}
	pool := BuildInferencePool(model)

	if string(pool.Spec.EndpointPickerRef.Name) != model.EPPName() {
		t.Errorf("EndpointPickerRef.Name:\ngot:  %q\nwant: %q", pool.Spec.EndpointPickerRef.Name, model.EPPName())
	}
	if pool.Spec.EndpointPickerRef.Port == nil || pool.Spec.EndpointPickerRef.Port.Number != 9002 {
		t.Error("EndpointPickerRef.Port != 9002")
	}
}

func TestBuildHTTPRoute(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	route := BuildHTTPRoute(model)

	if route.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", route.Name, model.EngineName())
	}
	if len(route.Spec.ParentRefs) != 1 || string(route.Spec.ParentRefs[0].Name) != defaultGatewayName {
		t.Error("unexpected ParentRefs")
	}
	rule := route.Spec.Rules[0]
	if len(rule.Matches) != 1 || rule.Matches[0].Headers[0].Value != "arnir0/Tiny-LLM" {
		t.Error("header match value mismatch")
	}
}
