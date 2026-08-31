// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// BuildAIBackend returns the AgentgatewayBackend that routes the model's
// InferencePool through the gateway's LLM pipeline for token-usage metrics.
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
		},
	}
}
