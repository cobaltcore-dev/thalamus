// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"strings"

	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	gatewayBaseModelHeaderName    = "X-Gateway-Base-Model-Name"
	BodyBasedRoutingPolicyName    = "body-based-routing"
	bodyBasedRoutingCELExpression = "json(request.body).model"
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

// BuildHTTPRoutes returns one HTTPRoute per served model name
func BuildHTTPRoutes(model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact, gatewayName, listenerName string) []*gatewayv1.HTTPRoute {
	modelNames := slices.Clone(model.ModelNames(artifact))
	slices.Sort(modelNames)

	routes := make([]*gatewayv1.HTTPRoute, 0, len(modelNames))
	for _, modelName := range modelNames {
		routes = append(routes, buildHTTPRoute(model, modelName, gatewayName, listenerName))
	}
	return routes
}

func buildHTTPRoute(model *v1alpha1.Model, modelName, gatewayName, listenerName string) *gatewayv1.HTTPRoute {
	matches := make([]gatewayv1.HTTPRouteMatch, 0, len(modelRoutes))
	for _, r := range modelRoutes {
		matches = append(matches, gatewayv1.HTTPRouteMatch{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  new(gatewayv1.PathMatchExact),
				Value: new(r.path),
			},
			Headers: []gatewayv1.HTTPHeaderMatch{{
				Type:  new(gatewayv1.HeaderMatchExact),
				Name:  gatewayBaseModelHeaderName,
				Value: modelName,
			}},
		})
	}

	return &gatewayv1.HTTPRoute{
		Name:      httpRouteName(model.Name, modelName),
		Namespace: model.Namespace,
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:        gatewayv1.ObjectName(gatewayName),
						Namespace:   new(gatewayv1.Namespace(model.Namespace)),
						SectionName: new(gatewayv1.SectionName(listenerName)),
					},
				},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: matches,
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

// httpRouteName builds a deterministic route name from the Model and served
// model name. Served names are arbitrary strings, so they are converted to a
// DNS-safe form.
func httpRouteName(modelName, servedName string) string {
	return modelName + "-" + dnsSafeName(servedName)
}

// dnsSafeName lowercases a served model name and replaces unsupported
// characters with dashes. The conversion is lossy; served names that differ
// only by case or punctuation can produce the same route name.
func dnsSafeName(name string) string {
	var safe strings.Builder
	lastWasDash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			safe.WriteRune(r)
			lastWasDash = false
		} else if safe.Len() > 0 && !lastWasDash {
			safe.WriteByte('-')
			lastWasDash = true
		}
	}
	if result := strings.Trim(safe.String(), "-"); result != "" {
		return result
	}
	return "model"
}

// BuildBodyBasedRoutingPolicy returns the policy setting the header the per-model routes match on.
func BuildBodyBasedRoutingPolicy(gateway *gatewayv1.Gateway, listenerName string) *agentgatewayv1alpha1.AgentgatewayPolicy {
	return &agentgatewayv1alpha1.AgentgatewayPolicy{
		Name:      BodyBasedRoutingPolicyName,
		Namespace: gateway.Namespace,
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			TargetRefs: []agentgatewayv1alpha1.LocalPolicyTargetReferenceWithSectionName{
				{
					Group:       gatewayv1.GroupName,
					Kind:        "Gateway",
					Name:        gatewayv1.ObjectName(gateway.Name),
					SectionName: new(gatewayv1.SectionName(listenerName)),
				},
			},
			Traffic: &agentgatewayv1alpha1.Traffic{
				Phase: new(agentgatewayv1alpha1.PolicyPhasePreRouting),
				Transformation: &agentgatewayv1alpha1.TransformationOrConditional{
					Request: &agentgatewayv1alpha1.Transform{
						Set: []agentgatewayv1alpha1.HeaderTransformation{
							{
								Name:  gatewayBaseModelHeaderName,
								Value: bodyBasedRoutingCELExpression,
							},
						},
					},
				},
			},
		},
	}
}
