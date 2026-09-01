// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildAIBackend(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	backend := BuildAIBackend(model)

	if backend.Name != model.Name {
		t.Errorf("Name:\ngot:  %q\nwant: %q", backend.Name, model.Name)
	}
	if backend.Namespace != model.Namespace {
		t.Errorf("Namespace:\ngot:  %q\nwant: %q", backend.Namespace, model.Namespace)
	}

	ai := backend.Spec.AI
	if ai == nil || ai.LLM == nil || ai.LLM.Custom == nil {
		t.Fatal("spec.ai.provider.custom not set")
	}
	custom := ai.LLM.Custom
	if custom.BackendRef == nil {
		t.Fatal("spec.ai.provider.custom.backendRef not set")
	}
	if *custom.BackendRef.Group != inferencev1.GroupName ||
		*custom.BackendRef.Kind != "InferencePool" ||
		custom.BackendRef.Name != model.EngineName() {
		t.Errorf("unexpected backendRef: %+v", custom.BackendRef)
	}

	wantFormats := []agentgatewayv1alpha1.ProviderFormat{
		agentgatewayv1alpha1.ProviderFormatCompletions,
		agentgatewayv1alpha1.ProviderFormatMessages,
		agentgatewayv1alpha1.ProviderFormatResponses,
		agentgatewayv1alpha1.ProviderFormatEmbeddings,
		agentgatewayv1alpha1.ProviderFormatRerank,
		agentgatewayv1alpha1.ProviderFormatAnthropicTokenCount,
	}
	if len(custom.Formats) != len(wantFormats) {
		t.Fatalf("formats:\ngot:  %+v\nwant: %d entries", custom.Formats, len(wantFormats))
	}
	for i, want := range wantFormats {
		if custom.Formats[i].Type != want {
			t.Errorf("formats[%d]:\ngot:  %s\nwant: %s", i, custom.Formats[i].Type, want)
		}
	}

	policies := backend.Spec.Policies
	if policies == nil || policies.AI == nil {
		t.Fatal("spec.policies.ai not set")
	}
	wantRoutes := map[string]agentgatewayv1alpha1.RouteType{
		"/v1/chat/completions": agentgatewayv1alpha1.RouteTypeCompletions,
		"/v1/messages":         agentgatewayv1alpha1.RouteTypeMessages,
		"/v1/responses":        agentgatewayv1alpha1.RouteTypeResponses,
		"/v1/embeddings":       agentgatewayv1alpha1.RouteTypeEmbeddings,
		"/v1/rerank":           agentgatewayv1alpha1.RouteTypeRerank,
		"/v2/rerank":           agentgatewayv1alpha1.RouteTypeRerank,
		"/tokenize":            agentgatewayv1alpha1.RouteTypePassthrough,
		"/detokenize":          agentgatewayv1alpha1.RouteTypePassthrough,
	}
	if len(policies.AI.Routes) != len(wantRoutes) {
		t.Fatalf("routes:\ngot:  %+v\nwant: %d entries", policies.AI.Routes, len(wantRoutes))
	}
	for path, want := range wantRoutes {
		if got := policies.AI.Routes[path]; got != want {
			t.Errorf("routes[%q]:\ngot:  %s\nwant: %s", path, got, want)
		}
	}
}
