// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const (
	testGatewayName     = "test-gateway"
	testGatewayListener = "custom-listener"
)

func routeServedName(route gatewayv1.HTTPRoute) string {
	if len(route.Spec.Rules) == 0 {
		return ""
	}
	for _, match := range route.Spec.Rules[0].Matches {
		for _, header := range match.Headers {
			if header.Name == gatewayBaseModelHeaderName {
				return header.Value
			}
		}
	}
	return ""
}

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

func TestBuildHTTPRoutes(t *testing.T) {
	model, artifact := testutil.NewModelWithArtifact("tiny-llm", "default")
	routes := BuildHTTPRoutes(model, artifact, "custom-gateway", testGatewayListener)

	if len(routes) != 1 {
		t.Fatalf("len(routes): %d, want 1", len(routes))
	}
	route := routes[0]
	if route.Name == model.EngineName() {
		t.Errorf("route.Name: got %q, want a dedicated per-name route name", route.Name)
	}
	if !strings.HasPrefix(route.Name, "tiny-llm-arnir0-tiny-llm") {
		t.Errorf("route.Name:\ngot:  %q\nwant prefix: %q", route.Name, "tiny-llm-arnir0-tiny-llm")
	}
	if got := routeServedName(*route); got != "arnir0/Tiny-LLM" {
		t.Errorf("served name:\ngot:  %q\nwant: %q", got, "arnir0/Tiny-LLM")
	}
	if route.Namespace != "default" {
		t.Errorf("Namespace:\ngot:  %q\nwant: %q", route.Namespace, "default")
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Errorf("len(route.Spec.ParentRefs): %d, want 1", len(route.Spec.ParentRefs))
	}
	parentRef := route.Spec.ParentRefs[0]
	if name := parentRef.Name; name != "custom-gateway" {
		t.Errorf("parentRef.Name:\ngot:  %q\nwant: %q", name, "custom-gateway")
	}
	if section := *parentRef.SectionName; section != testGatewayListener {
		t.Errorf("parentRef.SectionName: got: %q, want %q", section, testGatewayListener)
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

func TestBuildHTTPRoutes_Aliases(t *testing.T) {
	model, artifact := testutil.NewModelWithArtifact("tiny-llm", "default")
	model.Spec.Aliases = []string{"tiny", "tiny-latest"}
	routes := BuildHTTPRoutes(model, artifact, "custom-gateway", testGatewayListener)

	if len(routes) != len(model.Spec.Aliases) {
		t.Fatalf("len(routes): %d, want %d", len(routes), len(model.Spec.Aliases))
	}

	seenNames := make(map[string]bool)
	seenRouteNames := make(map[string]bool)
	for _, route := range routes {
		servedName := routeServedName(*route)
		if !slices.Contains(model.Spec.Aliases, servedName) {
			t.Errorf("unexpected served name: %q", servedName)
		}
		if seenNames[servedName] {
			t.Errorf("duplicate route for served name %q", servedName)
		}
		seenNames[servedName] = true
		if seenRouteNames[route.Name] {
			t.Errorf("duplicate route name %q", route.Name)
		}
		seenRouteNames[route.Name] = true
		if route.Name == model.EngineName() {
			t.Errorf("route.Name: got %q, want a dedicated per-name route name", route.Name)
		}

		rule := route.Spec.Rules[0]
		if len(rule.Matches) != len(modelRoutes) {
			t.Fatalf("route %q len(rule.Matches): %d, want %d", route.Name, len(rule.Matches), len(modelRoutes))
		}
		seenPaths := make(map[string]bool)
		for _, match := range rule.Matches {
			if len(match.Headers) != 1 || match.Headers[0].Name != gatewayBaseModelHeaderName {
				t.Fatalf("unexpected match headers: %+v", match.Headers)
			}
			if match.Headers[0].Value != servedName {
				t.Errorf("route %q header value: got %q, want %q", route.Name, match.Headers[0].Value, servedName)
			}
			seenPaths[*match.Path.Value] = true
		}
		for _, r := range modelRoutes {
			if !seenPaths[r.path] {
				t.Errorf("route %q missing path match %q", route.Name, r.path)
			}
		}
	}
	for _, alias := range model.Spec.Aliases {
		if !seenNames[alias] {
			t.Errorf("missing route for alias %q", alias)
		}
	}
}

func TestHTTPRouteName(t *testing.T) {
	modelName := "tiny-llm"
	name := httpRouteName(modelName, "arnir0/Tiny-LLM")

	if want := "tiny-llm-arnir0-tiny-llm"; name != want {
		t.Errorf("route name:\ngot:  %q\nwant: %q", name, want)
	}
	if name != httpRouteName(modelName, "arnir0/Tiny-LLM") {
		t.Error("httpRouteName is not deterministic")
	}
	if name == httpRouteName(modelName, "arnir0/Tiny-Other") {
		t.Error("different served names produced the same route name")
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		t.Errorf("route name %q is not a valid DNS subdomain: %v", name, errs)
	}
	if got, want := httpRouteName(modelName, "///"), "tiny-llm-model"; got != want {
		t.Errorf("fallback route name:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildBodyBasedRoutingPolicy(t *testing.T) {
	gateway := &gatewayv1.Gateway{Name: testGatewayName, Namespace: "thalamus"}

	policy := BuildBodyBasedRoutingPolicy(gateway, testGatewayListener)

	if policy.Name != BodyBasedRoutingPolicyName {
		t.Errorf("policy.Name:\ngot:  %q\nwant: %q", policy.Name, BodyBasedRoutingPolicyName)
	}
	if policy.Namespace != "thalamus" {
		t.Errorf("policy.Namespace:\ngot:  %q\nwant: %q", policy.Namespace, "thalamus")
	}
	if len(policy.Spec.TargetRefs) != 1 {
		t.Fatalf("len(policy.Spec.TargetRefs): %d, want 1", len(policy.Spec.TargetRefs))
	}
	ref := policy.Spec.TargetRefs[0]
	if string(ref.Group) != gatewayv1.GroupName || ref.Kind != "Gateway" || ref.Name != testGatewayName {
		t.Errorf("targetRef:\ngot:  %s/%s %s\nwant: %s/Gateway %s",
			ref.Group, ref.Kind, ref.Name, gatewayv1.GroupName, testGatewayName)
	}
	if ref.SectionName == nil || *ref.SectionName != testGatewayListener {
		t.Errorf("targetRef.SectionName:\ngot:  %v\nwant: %q", ref.SectionName, testGatewayListener)
	}

	traffic := policy.Spec.Traffic
	if traffic == nil || traffic.Phase == nil || *traffic.Phase != agentgatewayv1alpha1.PolicyPhasePreRouting {
		t.Fatalf("traffic.phase:\ngot:  %v\nwant: PreRouting", traffic)
	}
	if traffic.Transformation == nil || traffic.Transformation.Request == nil ||
		len(traffic.Transformation.Request.Set) != 1 {
		t.Fatal("expected one request transformation set entry")
	}
	set := traffic.Transformation.Request.Set[0]
	if set.Name != gatewayBaseModelHeaderName {
		t.Errorf("transformation header name:\ngot:  %q\nwant: %q", set.Name, gatewayBaseModelHeaderName)
	}
	if set.Value != bodyBasedRoutingCELExpression {
		t.Errorf("transformation CEL expression:\ngot:  %q\nwant: %q", set.Value, bodyBasedRoutingCELExpression)
	}
}
