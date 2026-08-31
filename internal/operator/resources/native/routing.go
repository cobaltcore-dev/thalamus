// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	defaultGatewayName         = "inference-gateway"
	defaultGatewaySectionName  = "api"
	gatewayBaseModelHeaderName = "X-Gateway-Base-Model-Name"
)

// BuildEPPExtProcPolicy returns the AgentgatewayPolicy that wires the model's
// HTTPRoute to the EPP as an ext-proc service, following the llm-d router
// standalone mode where the proxy consults the EPP for endpoint selection.
func BuildEPPExtProcPolicy(model *v1alpha1.Model) *agentgatewayv1alpha1.AgentgatewayPolicy {
	port := eppGRPCExtProcPort
	return &agentgatewayv1alpha1.AgentgatewayPolicy{
		Name:      model.Name + "-extproc",
		Namespace: model.Namespace,
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			TargetRefs: []agentgatewayv1alpha1.LocalPolicyTargetReferenceWithSectionName{
				{
					Group: gatewayv1.Group(gatewayv1.GroupName),
					Kind:  gatewayv1.Kind("HTTPRoute"),
					Name:  gatewayv1.ObjectName(model.EngineName()),
				},
			},
			Traffic: &agentgatewayv1alpha1.Traffic{
				ExtProc: &agentgatewayv1alpha1.ExtProcOrConditional{
					BackendRef: &gatewayv1.BackendObjectReference{
						Name: gatewayv1.ObjectName(model.EPPName()),
						Port: &port,
					},
				},
			},
		},
	}
}

// BuildHTTPRoute returns the HTTPRoute that routes requests to the model's
// AgentgatewayBackend, so traffic is token-metered by the LLM pipeline.
func BuildHTTPRoute(model *v1alpha1.Model) *gatewayv1.HTTPRoute {
	modelName := ""
	if model.Spec.Weights.Type == v1alpha1.WeightsTypeHF && model.Spec.Weights.HF != nil {
		modelName = model.Spec.Weights.HF.RepoID
	}
	headerValue := gatewayv1.HTTPHeaderMatch{
		Type:  new(gatewayv1.HeaderMatchExact),
		Name:  gatewayBaseModelHeaderName,
		Value: modelName,
	}

	return &gatewayv1.HTTPRoute{
		Name:      model.EngineName(),
		Namespace: model.Namespace,
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:        defaultGatewayName,
						Namespace:   new(gatewayv1.Namespace(model.Namespace)),
						SectionName: new(gatewayv1.SectionName(defaultGatewaySectionName)),
					},
				},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Headers: []gatewayv1.HTTPHeaderMatch{headerValue}},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							Group: new(gatewayv1.Group(agentgatewayv1alpha1.GroupName)),
							Kind:  new(gatewayv1.Kind("AgentgatewayBackend")),
							Name:  gatewayv1.ObjectName(model.Name),
						},
					},
				},
			},
		},
	}
}
