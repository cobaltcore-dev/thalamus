// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	gatewayBaseModelHeaderName    = "X-Gateway-Base-Model-Name"
	BodyBasedRoutingPolicyName    = "body-based-routing"
	bodyBasedRoutingCELExpression = "json(request.body).model"

	// httpRouteNamePrefix namespaces routes that belong to a single served model
	// name. It keeps them distinct from other routes the operator manages, such
	// as ModelListRouteName.
	httpRouteNamePrefix = "model-route-"
	// httpRouteNameHashLength is the number of hex characters of the digest
	// appended to every route name. 40 bits make collisions negligible for the
	// number of routes a namespace can hold.
	httpRouteNameHashLength = 10
	// httpRouteNameReadableMaxLength bounds the human-readable part so that the
	// full name always fits within the DNS subdomain limit.
	httpRouteNameReadableMaxLength = validation.DNS1123SubdomainMaxLength -
		len(httpRouteNamePrefix) - len("-") - httpRouteNameHashLength
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

// httpRouteName returns a deterministic, collision-safe and length-bounded
// name for the HTTPRoute of one served model name.
//
// The readable part is best effort only: served names are arbitrary strings,
// so the DNS-safe conversion is lossy and different (Model, served name) pairs
// can produce the same readable text. Identity therefore comes from a digest
// over an unambiguous, length-prefixed encoding of the original pair.
func httpRouteName(modelName, servedName string) string {
	readable := slugPrefix(modelName+"-"+servedName, httpRouteNameReadableMaxLength)
	return httpRouteNamePrefix + readable + "-" + httpRouteNameDigest(modelName, servedName)
}

// httpRouteNameDigest returns a short, stable digest of the original Model and
// served name. The length prefixes make the encoding unambiguous, so pairs
// such as ("a/b", "c") and ("a", "b/c") do not collide.
func httpRouteNameDigest(modelName, servedName string) string {
	encoded := fmt.Sprintf("%d:%s%d:%s", len(modelName), modelName, len(servedName), servedName)
	sum := sha256.Sum256([]byte(encoded))
	return hex.EncodeToString(sum[:])[:httpRouteNameHashLength]
}

// slugPrefix lowercases s, replaces unsupported characters with dashes and
// returns at most n bytes of the result. It only contributes readability; the
// digest in httpRouteName provides uniqueness.
func slugPrefix(s string, n int) string {
	var b strings.Builder
	lastWasDash := true // start true to suppress leading dash
	for _, r := range strings.ToLower(s) {
		if b.Len() >= n {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastWasDash = false
		} else if !lastWasDash {
			b.WriteByte('-')
			lastWasDash = true
		}
	}
	if slug := strings.TrimRight(b.String(), "-"); slug != "" {
		return slug
	}
	return "x"
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
