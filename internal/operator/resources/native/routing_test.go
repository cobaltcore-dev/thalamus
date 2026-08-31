// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildEPPExtProcPolicy(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	policy := BuildEPPExtProcPolicy(model)

	if policy.Name != model.Name+"-extproc" {
		t.Errorf("Name:\ngot:  %q\nwant: %q", policy.Name, model.Name+"-extproc")
	}
	if len(policy.Spec.TargetRefs) != 1 {
		t.Fatalf("len(policy.Spec.TargetRefs): %d, want 1", len(policy.Spec.TargetRefs))
	}
	target := policy.Spec.TargetRefs[0]
	if target.Group != gatewayv1.Group(gatewayv1.GroupName) || target.Kind != "HTTPRoute" || target.Name != gatewayv1.ObjectName(model.EngineName()) {
		t.Errorf("unexpected targetRef: %+v", target)
	}

	extProc := policy.Spec.Traffic.ExtProc
	if extProc == nil || extProc.BackendRef == nil {
		t.Fatal("traffic.extProc.backendRef not set")
	}
	if extProc.BackendRef.Name != gatewayv1.ObjectName(model.EPPName()) {
		t.Errorf("backendRef.Name:\ngot:  %q\nwant: %q", extProc.BackendRef.Name, model.EPPName())
	}
	if extProc.BackendRef.Port == nil || *extProc.BackendRef.Port != eppGRPCExtProcPort {
		t.Errorf("backendRef.Port: %+v, want %d", extProc.BackendRef.Port, eppGRPCExtProcPort)
	}
}

func TestBuildHTTPRoute(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	route := BuildHTTPRoute(model)

	if route.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", route.Name, model.EngineName())
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("len(route.Spec.ParentRefs): %d, want 1", len(route.Spec.ParentRefs))
	}
	parentRef := route.Spec.ParentRefs[0]
	if name := parentRef.Name; name != defaultGatewayName {
		t.Errorf("parentRef.Name:\ngot:  %q\nwant: %q", name, defaultGatewayName)
	}
	if section := *parentRef.SectionName; section != defaultGatewaySectionName {
		t.Errorf("parentRef.SectionName: got: %q, want %q", section, defaultGatewaySectionName)
	}
	rule := route.Spec.Rules[0]
	if len(rule.Matches) != 1 || rule.Matches[0].Headers[0].Value != "arnir0/Tiny-LLM" {
		t.Error("header match value mismatch")
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
