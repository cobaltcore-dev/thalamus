// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// modelRoutes lists the paths a model exposes; anything else never reaches vLLM.
var modelRoutes = []struct {
	path  string
	rType agentgatewayv1alpha1.RouteType
}{
	{path: "/v1/chat/completions", rType: agentgatewayv1alpha1.RouteTypeCompletions},
	{path: "/v1/messages", rType: agentgatewayv1alpha1.RouteTypeMessages},
	{path: "/v1/responses", rType: agentgatewayv1alpha1.RouteTypeResponses},
	{path: "/v1/embeddings", rType: agentgatewayv1alpha1.RouteTypeEmbeddings},
	{path: "/v1/rerank", rType: agentgatewayv1alpha1.RouteTypeRerank},
	{path: "/v2/rerank", rType: agentgatewayv1alpha1.RouteTypeRerank},
	{path: "/tokenize", rType: agentgatewayv1alpha1.RouteTypePassthrough},
	{path: "/detokenize", rType: agentgatewayv1alpha1.RouteTypePassthrough},
}

// BuildAIBackend builds the AgentgatewayBackend for the model.
func BuildAIBackend(model *v1alpha1.Model) *agentgatewayv1alpha1.AgentgatewayBackend {
	settings := agentgatewayv1alpha1.CustomProviderSettings{
		BackendRef: &agentgatewayv1alpha1.LocalBackendObjectReference{
			Group: new(inferencev1.GroupName),
			Kind:  new("InferencePool"),
			Name:  model.EngineName(),
		},
		Formats: []agentgatewayv1alpha1.ProviderFormatConfig{
			{Type: agentgatewayv1alpha1.ProviderFormatCompletions},
			{Type: agentgatewayv1alpha1.ProviderFormatMessages},
			{Type: agentgatewayv1alpha1.ProviderFormatResponses},
			{Type: agentgatewayv1alpha1.ProviderFormatEmbeddings},
			{Type: agentgatewayv1alpha1.ProviderFormatRerank},
			{Type: agentgatewayv1alpha1.ProviderFormatAnthropicTokenCount},
		},
	}
	routes := make(map[string]agentgatewayv1alpha1.RouteType, len(modelRoutes))
	for _, r := range modelRoutes {
		routes[r.path] = r.rType
	}
	return &agentgatewayv1alpha1.AgentgatewayBackend{
		Name:      model.Name,
		Namespace: model.Namespace,
		Spec: agentgatewayv1alpha1.AgentgatewayBackendSpec{
			AI: &agentgatewayv1alpha1.AIBackend{
				LLM: &agentgatewayv1alpha1.LLMProvider{
					Custom: &agentgatewayv1alpha1.CustomProvider{
						CustomProviderSettings: settings,
					},
				},
			},
			Policies: &agentgatewayv1alpha1.BackendFull{
				AI: &agentgatewayv1alpha1.BackendAI{
					Routes: routes,
				},
			},
		},
	}
}
