// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestGetGateway(t *testing.T) {
	s := testutil.NewScheme(t)
	nn := types.NamespacedName{Name: testGatewayName, Namespace: testNamespace}

	t.Run("returns the gateway", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(newGateway()).Build()

		got, err := getGateway(context.Background(), c, nn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.Name != testGatewayName {
			t.Fatalf("expected gateway %q, got %v", testGatewayName, got)
		}
	})

	t.Run("nil when missing", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(s).Build()

		got, err := getGateway(context.Background(), c, nn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil gateway, got %v", got)
		}
	})

	t.Run("nil when deleting", func(t *testing.T) {
		gateway := newGateway()
		now := metav1.Now()
		gateway.DeletionTimestamp = &now
		gateway.Finalizers = []string{"example.com/test"}
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway).Build()

		got, err := getGateway(context.Background(), c, nn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil gateway, got %v", got)
		}
	})

	t.Run("errors are passed through", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()

		got, err := getGateway(context.Background(), c, nn)
		if err == nil {
			t.Fatal("expected error for unregistered type")
		}
		if got != nil {
			t.Fatalf("expected nil gateway, got %v", got)
		}
	})
}
