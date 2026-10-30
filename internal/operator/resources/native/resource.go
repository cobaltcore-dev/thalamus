// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// Resource is a desired native resource. Object identifies it; Body renders it
// when the resource is applied.
type Resource struct {
	Object client.Object
	Body   func() (client.Object, error)
}

// Resources returns the native serving resources in apply order.
func Resources(model *v1alpha1.Model, gatewayName, listenerName, routerServiceAccountName string) []Resource {
	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: model.Namespace}
	}
	engine, epp := model.EngineName(), model.EPPName()

	return []Resource{
		{
			Object: &appsv1.Deployment{ObjectMeta: meta(engine)},
			Body: func() (client.Object, error) {
				return BuildEngineDeployment(model)
			},
		},
		{
			Object: &corev1.Service{ObjectMeta: meta(engine)},
			Body: func() (client.Object, error) {
				return BuildEngineService(model), nil
			},
		},
		{
			Object: &inferencev1.InferencePool{ObjectMeta: meta(engine)},
			Body: func() (client.Object, error) {
				return BuildInferencePool(model), nil
			},
		},
		{
			Object: &agentgatewayv1alpha1.AgentgatewayBackend{ObjectMeta: meta(model.Name)},
			Body: func() (client.Object, error) {
				return BuildAIBackend(model), nil
			},
		},
		{
			Object: &gatewayv1.HTTPRoute{ObjectMeta: meta(engine)},
			Body: func() (client.Object, error) {
				return BuildHTTPRoute(model, gatewayName, listenerName), nil
			},
		},
		{
			Object: &corev1.ConfigMap{ObjectMeta: meta(epp)},
			Body: func() (client.Object, error) {
				return BuildEPPConfigMap(model), nil
			},
		},
		{
			Object: &appsv1.Deployment{ObjectMeta: meta(epp)},
			Body: func() (client.Object, error) {
				return BuildEPPDeployment(model, routerServiceAccountName)
			},
		},
		{
			Object: &corev1.Service{ObjectMeta: meta(epp)},
			Body: func() (client.Object, error) {
				return BuildEPPService(model), nil
			},
		},
	}
}
