// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildInferencePool(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: "ghcr.io/llm-d/llm-d-router-endpoint-picker:v0.9.0"}
	pool := BuildInferencePool(model)

	if pool.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", pool.Name, model.EngineName())
	}
	if len(pool.Spec.TargetPorts) != 1 || pool.Spec.TargetPorts[0].Number != engineHTTPPort {
		t.Error("unexpected TargetPorts")
	}
	if string(pool.Spec.EndpointPickerRef.Name) != model.EPPName() {
		t.Errorf("EndpointPickerRef.Name:\ngot:  %q\nwant: %q", pool.Spec.EndpointPickerRef.Name, model.EPPName())
	}
	if pool.Spec.EndpointPickerRef.Port == nil || pool.Spec.EndpointPickerRef.Port.Number != inferencev1.PortNumber(eppGRPCExtProcPort) {
		t.Errorf("EndpointPickerRef.Port != %d", eppGRPCExtProcPort)
	}
}

func TestBuildHTTPRoute(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	route := BuildHTTPRoute(model)

	if route.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", route.Name, model.EngineName())
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Errorf("len(route.Spec.ParentRefs): %d, want 1", len(route.Spec.ParentRefs))
	}
	parentRef := route.Spec.ParentRefs[0]
	if name := parentRef.Name; name != defaultGatewayName {
		t.Errorf("parentRef.Name:\ngot:  %q\nwant: %q", name, defaultGatewayName)
	}
	if section := *parentRef.SectionName; section != defaultGatewaySectionName {
		t.Errorf("parentRef.SectionName: got: %q, want %q", section, defaultGatewaySectionName)
	}
	rule := route.Spec.Rules[0]
	if len(rule.Matches) != len(modelRoutes) {
		t.Fatalf("len(rule.Matches): %d, want %d", len(rule.Matches), len(modelRoutes))
	}
	for i, want := range modelRoutes {
		match := rule.Matches[i]
		if match.Path == nil || match.Path.Type == nil || *match.Path.Type != gatewayv1.PathMatchExact {
			t.Errorf("matches[%d].path: got %+v, want Exact match", i, match.Path)
			continue
		}
		if *match.Path.Value != want.path {
			t.Errorf("matches[%d].path.value:\ngot:  %q\nwant: %q", i, *match.Path.Value, want.path)
		}
		if len(match.Headers) != 1 || match.Headers[0].Name != gatewayBaseModelHeaderName || match.Headers[0].Value != "arnir0/Tiny-LLM" {
			t.Errorf("matches[%d].headers: got %+v", i, match.Headers)
		}
	}

	if len(rule.BackendRefs) != 1 {
		t.Fatalf("len(rule.BackendRefs): %d, want 1", len(rule.BackendRefs))
	}
	backendRef := rule.BackendRefs[0].BackendObjectReference
	if *backendRef.Group != agentgatewayv1alpha1.GroupName || *backendRef.Kind != "AgentgatewayBackend" {
		t.Errorf("backendRef group/kind:\ngot:  %s/%s\nwant: %s/AgentgatewayBackend", *backendRef.Group, *backendRef.Kind, agentgatewayv1alpha1.GroupName)
	}
	if backendRef.Name != gatewayv1.ObjectName(model.Name) {
		t.Errorf("backendRef.Name:\ngot:  %q\nwant: %q", backendRef.Name, model.Name)
	}
}
