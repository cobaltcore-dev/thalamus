// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	"k8s.io/utils/ptr"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

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
	if len(route.Spec.ParentRefs) != 1 || string(route.Spec.ParentRefs[0].Name) != defaultGatewayName {
		t.Error("unexpected ParentRefs")
	}
	if route.Spec.ParentRefs[0].Namespace != nil {
		t.Error("Namespace should be nil to default to local namespace")
	}
	if route.Spec.ParentRefs[0].SectionName != nil {
		t.Error("SectionName should be nil when routing is not set")
	}
	rule := route.Spec.Rules[0]
	if len(rule.Matches) != 1 || rule.Matches[0].Headers[0].Value != "arnir0/Tiny-LLM" {
		t.Error("header match value mismatch")
	}
}

func TestBuildHTTPRouteWithRoutingSectionOnly(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	section := gatewayv1.SectionName("api-key-auth")
	model.Spec.Routing = []gatewayv1.ParentReference{
		{SectionName: &section},
	}
	route := BuildHTTPRoute(model)

	ref := route.Spec.ParentRefs[0]
	if string(ref.Name) != defaultGatewayName {
		t.Errorf("Name:\ngot:  %q\nwant: %q", ref.Name, defaultGatewayName)
	}
	if ref.Namespace != nil {
		t.Error("Namespace should be nil to default to local namespace")
	}
	if ref.SectionName == nil || *ref.SectionName != section {
		t.Errorf("SectionName:\ngot:  %v\nwant: %q", ref.SectionName, section)
	}
}

func TestBuildHTTPRouteWithRouting(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	section := gatewayv1.SectionName("api-key-auth")
	model.Spec.Routing = []gatewayv1.ParentReference{
		{
			Name:        "my-custom-gateway",
			SectionName: &section,
		},
		{
			Name:      "another-gateway",
			Namespace: ptr.To(gatewayv1.Namespace("other-ns")),
		},
	}
	route := BuildHTTPRoute(model)

	if len(route.Spec.ParentRefs) != 2 {
		t.Fatalf("expected 2 parentRefs, got %d", len(route.Spec.ParentRefs))
	}

	ref0 := route.Spec.ParentRefs[0]
	if string(ref0.Name) != "my-custom-gateway" {
		t.Errorf("ParentRefs[0].Name:\ngot:  %q\nwant: %q", ref0.Name, "my-custom-gateway")
	}
	if ref0.SectionName == nil || *ref0.SectionName != section {
		t.Errorf("ParentRefs[0].SectionName:\ngot:  %v\nwant: %q", ref0.SectionName, section)
	}
	if ref0.Namespace != nil {
		t.Error("ParentRefs[0].Namespace should be nil to default to local namespace")
	}

	ref1 := route.Spec.ParentRefs[1]
	if ref1.Namespace == nil || *ref1.Namespace != "other-ns" {
		t.Errorf("ParentRefs[1].Namespace should be preserved as %q", "other-ns")
	}
}
