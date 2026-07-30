// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const defaultGatewayName = "inference-gateway"

// BuildInferencePool returns the InferencePool for the model.
func BuildInferencePool(model *v1alpha1.Model) *inferencev1.InferencePool {
	spec := inferencev1.InferencePoolSpec{
		Selector: inferencev1.LabelSelector{
			MatchLabels: map[inferencev1.LabelKey]inferencev1.LabelValue{
				"app": inferencev1.LabelValue(model.EngineName()),
			},
		},
		TargetPorts: []inferencev1.Port{{Number: 8000}},
	}

	if model.Spec.Serving.EPP != nil {
		eppPort := inferencev1.Port{Number: 9002}
		spec.EndpointPickerRef = inferencev1.EndpointPickerRef{
			Name: inferencev1.ObjectName(model.EPPName()),
			Port: &eppPort,
		}
	}

	return &inferencev1.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EngineName(),
			Namespace: model.Namespace,
		},
		Spec: spec,
	}
}

// BuildHTTPRoute returns the HTTPRoute that routes requests to the model's InferencePool
// based on the X-Gateway-Base-Model-Name header.
func BuildHTTPRoute(model *v1alpha1.Model) *gatewayv1.HTTPRoute {
	ns := gatewayv1.Namespace(model.Namespace)
	modelName := ""
	if model.Spec.Weights.Type == v1alpha1.WeightsTypeHF && model.Spec.Weights.HF != nil {
		modelName = model.Spec.Weights.HF.RepoID
	}
	headerValue := gatewayv1.HTTPHeaderMatch{
		Type:  ptr.To(gatewayv1.HeaderMatchExact),
		Name:  "X-Gateway-Base-Model-Name",
		Value: modelName,
	}

	group := gatewayv1.Group("inference.networking.k8s.io")
	kind := gatewayv1.Kind("InferencePool")

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.EngineName(),
			Namespace: model.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      gatewayv1.ObjectName(defaultGatewayName),
						Namespace: &ns,
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
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Group: &group,
									Kind:  &kind,
									Name:  gatewayv1.ObjectName(model.EngineName()),
								},
							},
						},
					},
				},
			},
		},
	}
}
