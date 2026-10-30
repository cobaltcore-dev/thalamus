// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestGetObject(t *testing.T) {
	s := testutil.NewScheme(t)

	t.Run("returns the object", func(t *testing.T) {
		gateway := newGateway()
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway).Build()

		got, err := getObject(context.Background(), c, gateway.Namespace, gateway.Name, &gatewayv1.Gateway{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.Name != testGatewayName {
			t.Fatalf("expected gateway %q, got %v", testGatewayName, got)
		}
	})

	t.Run("nil when missing", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(s).Build()

		got, err := getObject(context.Background(), c, testNamespace, testGatewayName, &gatewayv1.Gateway{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil gateway, got %v", got)
		}
	})

	t.Run("errors are passed through", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()

		got, err := getObject(context.Background(), c, testNamespace, testGatewayName, &gatewayv1.Gateway{})
		if err == nil {
			t.Fatal("expected error for unregistered type")
		}
		if got != nil {
			t.Fatalf("expected nil gateway, got %v", got)
		}
	})
}
