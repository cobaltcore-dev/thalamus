// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	defaultGatewayName         = "inference-gateway"
	defaultGatewaySectionName  = "api"
	gatewayBaseModelHeaderName = "X-Gateway-Base-Model-Name"
)

// BuildInferencePool returns the InferencePool for the model.
func BuildInferencePool(model *v1alpha1.Model) *inferencev1.InferencePool {
	spec := inferencev1.InferencePoolSpec{
		Selector: inferencev1.LabelSelector{
			MatchLabels: map[inferencev1.LabelKey]inferencev1.LabelValue{
				"thalamus.cloud/engine": inferencev1.LabelValue(model.EngineName()),
			},
		},
		TargetPorts: []inferencev1.Port{{Number: engineHTTPPort}},
		EndpointPickerRef: &inferencev1.EndpointPickerRef{
			Name: inferencev1.ObjectName(model.EPPName()),
			Port: &inferencev1.Port{Number: inferencev1.PortNumber(eppGRPCExtProcPort)},
		},
	}

	return &inferencev1.InferencePool{
		Name:      model.EngineName(),
		Namespace: model.Namespace,
		Spec:      spec,
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
