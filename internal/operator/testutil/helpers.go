// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// NewScheme registers all schemes required by the thalamus operator.
func NewScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		v1alpha1.AddToScheme,
		appsv1.AddToScheme,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
		gatewayv1.Install,
		inferencev1.Install,
		agentgatewayv1alpha1.Install,
	} {
		if err := add(s); err != nil {
			t.Fatalf("scheme registration: %v", err)
		}
	}
	return s
}

// MustGet fetches obj by name/namespace from c, fataling the test if not found.
func MustGet[T client.Object](t *testing.T, c client.Client, name, namespace string, obj T) T {
	t.Helper()
	if err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		t.Fatalf("Get %T %q: %v", obj, name, err)
	}
	return obj
}

// UpdateStatus fetches obj, applies mutate, and updates its status subresource.
func UpdateStatus[T client.Object](t *testing.T, c client.Client, name, namespace string, obj T, mutate func(T)) {
	t.Helper()
	MustGet(t, c, name, namespace, obj)
	mutate(obj)
	if err := c.Status().Update(context.Background(), obj); err != nil {
		t.Fatalf("update %T status: %v", obj, err)
	}
}

// MustNotGet asserts that obj does not exist in c by name/namespace.
func MustNotGet(t *testing.T, c client.Client, name, namespace string, obj client.Object) {
	t.Helper()
	err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, obj)
	if err == nil {
		t.Fatalf("expected %T %q to be deleted, but it still exists", obj, name)
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("unexpected error fetching %T %q: %v", obj, name, err)
	}
}

// HTTPRouteHeaderValues returns the distinct values for headerName in the
// route's first rule, preserving their order.
func HTTPRouteHeaderValues(route gatewayv1.HTTPRoute, headerName gatewayv1.HTTPHeaderName) []string {
	if len(route.Spec.Rules) == 0 {
		return nil
	}

	var values []string
	for _, match := range route.Spec.Rules[0].Matches {
		for _, header := range match.Headers {
			if header.Name == headerName && !slices.Contains(values, header.Value) {
				values = append(values, header.Value)
			}
		}
	}
	return values
}
