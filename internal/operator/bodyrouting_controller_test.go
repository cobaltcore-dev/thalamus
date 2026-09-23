// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func reconcileBodyRoutingOnce(t *testing.T, r *BodyRoutingReconciler) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: testGatewayName, Namespace: testNamespace,
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
}

func mustGetBodyRoutingPolicy(t *testing.T, r *BodyRoutingReconciler) *agentgatewayv1alpha1.AgentgatewayPolicy {
	t.Helper()
	policy := &agentgatewayv1alpha1.AgentgatewayPolicy{}
	testutil.MustGet(t, r.Client, native.BodyBasedRoutingPolicyName, testNamespace, policy)
	return policy
}

func TestBodyRoutingReconcile_NoGateway(t *testing.T) {
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	r := &BodyRoutingReconciler{Client: c, Scheme: s, GatewayName: testGatewayName}

	reconcileBodyRoutingOnce(t, r)

	testutil.MustNotGet(t, c, native.BodyBasedRoutingPolicyName, testNamespace, &agentgatewayv1alpha1.AgentgatewayPolicy{})
}

func TestBodyRoutingReconcile_CreatesPolicy(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway).Build()
	r := &BodyRoutingReconciler{Client: c, Scheme: s, GatewayName: testGatewayName}

	reconcileBodyRoutingOnce(t, r)

	policy := mustGetBodyRoutingPolicy(t, r)
	mustAssertOwnedByGateway(t, policy, gateway)
}

func TestBodyRoutingReconcile_ReappliesModifiedPolicy(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	policy := native.BuildBodyBasedRoutingPolicy(gateway)
	policy.Spec.Traffic.Transformation = nil
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway, policy).Build()
	r := &BodyRoutingReconciler{Client: c, Scheme: s, GatewayName: testGatewayName}

	reconcileBodyRoutingOnce(t, r)

	fixed := mustGetBodyRoutingPolicy(t, r)
	if fixed.Spec.Traffic.Transformation == nil ||
		fixed.Spec.Traffic.Transformation.Request == nil ||
		len(fixed.Spec.Traffic.Transformation.Request.Set) != 1 {
		t.Fatalf("transformation not restored:\ngot:  %+v", fixed.Spec.Traffic.Transformation)
	}
	set := fixed.Spec.Traffic.Transformation.Request.Set[0]
	if set.Name != "X-Gateway-Base-Model-Name" {
		t.Errorf("transformation header name:\ngot:  %q\nwant: %q", set.Name, "X-Gateway-Base-Model-Name")
	}
}
