// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
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
	} {
		if err := add(s); err != nil {
			t.Fatalf("scheme registration: %v", err)
		}
	}
	return s
}

// MustGet fetches obj by name/namespace from c, fataling the test if not found.
func MustGet(t *testing.T, c client.Client, name, namespace string, obj client.Object) {
	t.Helper()
	if err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		t.Fatalf("Get %T %q: %v", obj, name, err)
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
