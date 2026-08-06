// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func newModelListPolicy() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(agentgatewayPolicyGVK)
	u.SetName(native.ModelListPolicyName)
	u.SetNamespace(testNamespace)
	if err := unstructured.SetNestedField(u.Object, int64(modelListDirectStatus),
		"spec", "traffic", "directResponse", "status"); err != nil {
		panic(err)
	}
	if err := unstructured.SetNestedField(u.Object, `{"object":"list","data":[]}`,
		"spec", "traffic", "directResponse", "body"); err != nil {
		panic(err)
	}
	if err := unstructured.SetNestedSlice(u.Object, []any{
		map[string]any{"name": modelListContentTypeHeader, "value": modelListContentTypeValue},
	}, "spec", "traffic", "directResponse", "headers"); err != nil {
		panic(err)
	}
	return u
}

func reconcileModelListOnce(t *testing.T, r *ModelListReconciler) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: native.ModelListPolicyName, Namespace: testNamespace},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	return res
}

func mustGetPolicyBody(t *testing.T, r *ModelListReconciler) string {
	t.Helper()
	policy := &unstructured.Unstructured{}
	policy.SetGroupVersionKind(agentgatewayPolicyGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Name: native.ModelListPolicyName, Namespace: testNamespace}, policy); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	body, _, err := unstructured.NestedString(policy.Object, "spec", "traffic", "directResponse", "body")
	if err != nil {
		t.Fatalf("NestedString: %v", err)
	}
	return body
}

func mustAssertContentType(t *testing.T, r *ModelListReconciler) {
	t.Helper()
	policy := &unstructured.Unstructured{}
	policy.SetGroupVersionKind(agentgatewayPolicyGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Name: native.ModelListPolicyName, Namespace: testNamespace}, policy); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	value, found, err := directResponseHeaderValue(policy.Object, modelListContentTypeHeader)
	if err != nil {
		t.Fatalf("directResponseHeaderValue: %v", err)
	}
	if !found {
		t.Fatal("Content-Type header not found")
	}
	if value != modelListContentTypeValue {
		t.Errorf("Content-Type header:\ngot:  %q\nwant: %q", value, modelListContentTypeValue)
	}
	status, _, err := unstructured.NestedInt64(policy.Object, "spec", "traffic", "directResponse", "status")
	if err != nil {
		t.Fatalf("NestedInt64: %v", err)
	}
	if status != modelListDirectStatus {
		t.Errorf("directResponse status:\ngot:  %d\nwant: %d", status, modelListDirectStatus)
	}
}

func TestModelListReconcile_PolicyNotFound(t *testing.T) {
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: native.ModelListPolicyName, Namespace: testNamespace},
	})
	if err == nil {
		t.Fatal("expected error when policy is not found")
	}
}

func TestModelListReconcile_NoModels(t *testing.T) {
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(newModelListPolicy()).Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	reconcileModelListOnce(t, r)

	if body := mustGetPolicyBody(t, r); body != `{"object":"list","data":[]}` {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, `{"object":"list","data":[]}`)
	}
	mustAssertContentType(t, r)
}

func TestModelListReconcile_OneReadyModel(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Status.Phase = v1alpha1.ModelPhaseReady
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(model).WithStatusSubresource(model).
		WithObjects(newModelListPolicy()).
		Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	reconcileModelListOnce(t, r)

	body := mustGetPolicyBody(t, r)
	expected := `{"object":"list","data":[{"id":"arnir0/Tiny-LLM","object":"model","owned_by":"arnir0"}]}`
	if body != expected {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, expected)
	}
	mustAssertContentType(t, r)
}

func TestModelListReconcile_OnlyReadyModelsListed(t *testing.T) {
	s := testutil.NewScheme(t)
	ready := testutil.NewModel("ready", testNamespace)
	ready.Spec.Weights.HF.RepoID = "org/ready-model"
	ready.Status.Phase = v1alpha1.ModelPhaseReady
	creating := testutil.NewModel("creating", testNamespace)
	creating.Spec.Weights.HF.RepoID = "org/creating-model"
	creating.Status.Phase = v1alpha1.ModelPhaseCreating
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(ready, creating).WithStatusSubresource(ready, creating).
		WithObjects(newModelListPolicy()).
		Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	reconcileModelListOnce(t, r)

	body := mustGetPolicyBody(t, r)
	expected := `{"object":"list","data":[{"id":"org/ready-model","object":"model","owned_by":"org"}]}`
	if body != expected {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, expected)
	}
	mustAssertContentType(t, r)
}

func TestModelListReconcile_SkipsUpdateWhenBodyUnchanged(t *testing.T) {
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(newModelListPolicy()).Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	reconcileModelListOnce(t, r)

	if body := mustGetPolicyBody(t, r); body != `{"object":"list","data":[]}` {
		t.Errorf("model list policy body:\ngot:  %s\nwant: %s", body, `{"object":"list","data":[]}`)
	}
	mustAssertContentType(t, r)
}

func TestModelListReconcile_AppliesWhenContentTypeMissing(t *testing.T) {
	s := testutil.NewScheme(t)
	u := newModelListPolicy()
	unstructured.RemoveNestedField(u.Object, "spec", "traffic", "directResponse", "headers")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(u).Build()
	r := &ModelListReconciler{Client: c, Scheme: s}

	reconcileModelListOnce(t, r)

	mustAssertContentType(t, r)
}

func TestPhaseChangedPredicate(t *testing.T) {
	pred := phaseChangedPredicate{}

	cases := []struct {
		name     string
		oldPhase v1alpha1.ModelPhase
		newPhase v1alpha1.ModelPhase
		want     bool
	}{
		{"creating→ready fires", v1alpha1.ModelPhaseCreating, v1alpha1.ModelPhaseReady, true},
		{"ready→failed fires", v1alpha1.ModelPhaseReady, v1alpha1.ModelPhaseFailed, true},
		{"ready→ready no-op", v1alpha1.ModelPhaseReady, v1alpha1.ModelPhaseReady, false},
		{"creating→failed no-op", v1alpha1.ModelPhaseCreating, v1alpha1.ModelPhaseFailed, false},
		{"pending→creating no-op", v1alpha1.ModelPhasePending, v1alpha1.ModelPhaseCreating, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := &v1alpha1.Model{Status: v1alpha1.ModelStatus{Phase: tc.oldPhase}}
			cur := &v1alpha1.Model{Status: v1alpha1.ModelStatus{Phase: tc.newPhase}}
			got := pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: cur})
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
