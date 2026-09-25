// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func newGateway() *gatewayv1.Gateway {
	return &gatewayv1.Gateway{
		Name:      testGatewayName,
		Namespace: testNamespace,
		Spec:      gatewayv1.GatewaySpec{GatewayClassName: "agentgateway"},
	}
}

func newModelListPolicy() *agentgatewayv1alpha1.AgentgatewayPolicy {
	return native.BuildModelListPolicy(testNamespace, `{"object":"list","data":[]}`)
}

func reconcileModelListOnce(t *testing.T, r *ModelListReconciler) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: testGatewayName, Namespace: testNamespace,
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	return res
}

func mustGetPolicy(t *testing.T, r *ModelListReconciler) *agentgatewayv1alpha1.AgentgatewayPolicy {
	t.Helper()
	policy := &agentgatewayv1alpha1.AgentgatewayPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: native.ModelListPolicyName, Namespace: testNamespace}, policy); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	return policy
}

func mustGetRoute(t *testing.T, r *ModelListReconciler) *gatewayv1.HTTPRoute {
	t.Helper()
	route := &gatewayv1.HTTPRoute{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: native.ModelListRouteName, Namespace: testNamespace}, route); err != nil {
		t.Fatalf("get route: %v", err)
	}
	return route
}

func mustGetPolicyBody(t *testing.T, r *ModelListReconciler) string {
	t.Helper()
	policy := mustGetPolicy(t, r)
	if policy.Spec.Traffic == nil || policy.Spec.Traffic.DirectResponse == nil {
		return ""
	}
	if dr := policy.Spec.Traffic.DirectResponse; dr.Body != nil {
		return *dr.Body
	}
	return ""
}

func mustAssertOwnedByGateway(t *testing.T, obj client.Object, gateway *gatewayv1.Gateway) {
	t.Helper()
	ref := metav1.GetControllerOf(obj)
	if ref == nil {
		t.Fatalf("%T %q has no controller owner reference", obj, obj.GetName())
	}
	if ref.UID != gateway.UID {
		t.Errorf("%T %q controller owner UID:\ngot:  %s\nwant: %s", obj, obj.GetName(), ref.UID, gateway.UID)
	}
	if ref.Name != gateway.Name {
		t.Errorf("%T %q controller owner name:\ngot:  %s\nwant: %s", obj, obj.GetName(), ref.Name, gateway.Name)
	}
	if ref.BlockOwnerDeletion == nil || *ref.BlockOwnerDeletion {
		t.Errorf("%T %q owner reference must not block owner deletion", obj, obj.GetName())
	}
}

func TestModelListReconcile_NoGateway(t *testing.T) {
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)

	testutil.MustNotGet(t, c, native.ModelListPolicyName, testNamespace, &agentgatewayv1alpha1.AgentgatewayPolicy{})
	testutil.MustNotGet(t, c, native.ModelListRouteName, testNamespace, &gatewayv1.HTTPRoute{})
}

func TestModelListReconcile_NoModels(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway).Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)

	if body := mustGetPolicyBody(t, r); body != `{"object":"list","data":[]}` {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, `{"object":"list","data":[]}`)
	}
	route := mustGetRoute(t, r)
	mustAssertOwnedByGateway(t, route, gateway)
	mustAssertOwnedByGateway(t, mustGetPolicy(t, r), gateway)
}

func TestModelListReconcile_OneReadyModel(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Status.Phase = v1alpha1.ModelPhaseReady
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(gateway).
		WithObjects(model).WithStatusSubresource(model).
		Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)

	body := mustGetPolicyBody(t, r)
	expected := `{"object":"list","data":[{"id":"arnir0/Tiny-LLM","object":"model","owned_by":"arnir0"}]}`
	if body != expected {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, expected)
	}
}

func TestModelListReconcile_OnlyReadyModelsListed(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	ready := testutil.NewModel("ready", testNamespace)
	ready.Spec.Weights.HF.RepoID = "org/ready-model"
	ready.Status.Phase = v1alpha1.ModelPhaseReady
	creating := testutil.NewModel("creating", testNamespace)
	creating.Spec.Weights.HF.RepoID = "org/creating-model"
	creating.Status.Phase = v1alpha1.ModelPhaseCreating
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(gateway).
		WithObjects(ready, creating).WithStatusSubresource(ready, creating).
		Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)

	body := mustGetPolicyBody(t, r)
	expected := `{"object":"list","data":[{"id":"org/ready-model","object":"model","owned_by":"org"}]}`
	if body != expected {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, expected)
	}
}

func TestModelListReconcile_ReappliesModifiedPolicy(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	policy := newModelListPolicy()
	policy.Spec.Traffic.DirectResponse.Headers = nil
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway, policy).Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)

	fixed := mustGetPolicy(t, r)
	dr := fixed.Spec.Traffic.DirectResponse
	var header *agentgatewayv1alpha1.DirectResponseHeader
	for i := range dr.Headers {
		if dr.Headers[i].Name == "Content-Type" {
			header = &dr.Headers[i]
		}
	}
	if header == nil {
		t.Fatal("Content-Type header not found after reconcile")
	}
	if header.Value != "'application/json'" {
		t.Errorf("Content-Type header value:\ngot:  %q\nwant: %q", header.Value, "'application/json'")
	}
	if status := dr.StatusCode; status == nil || *status != int32(200) {
		t.Errorf("directResponse status:\ngot:  %v\nwant: 200", status)
	}
}

func TestModelListReconcile_RecreatesDeletedRoute(t *testing.T) {
	s := testutil.NewScheme(t)
	gateway := newGateway()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gateway).Build()
	r := &ModelListReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelListOnce(t, r)
	if err := r.Delete(context.Background(), mustGetRoute(t, r)); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	reconcileModelListOnce(t, r)

	route := mustGetRoute(t, r)
	mustAssertOwnedByGateway(t, route, gateway)
}
