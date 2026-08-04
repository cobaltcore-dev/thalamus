// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const testNamespace = "default"

func reconcileModelOnce(t *testing.T, r *ModelReconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: testNamespace},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	return res
}

func TestReconcile_NotFound(t *testing.T) {
	s := testutil.NewScheme(t)
	r := &ModelReconciler{Client: fake.NewClientBuilder().WithScheme(s).Build(), Scheme: s}
	reconcileModelOnce(t, r, "missing")
}

func TestReconcile_Native(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &corev1.Service{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})

	// Deployment must have owner reference pointing at the Model.
	if len(dep.OwnerReferences) == 0 || dep.OwnerReferences[0].Name != "tiny-llm" {
		t.Error("Deployment missing owner reference to Model")
	}
}

func TestReconcile_NativeWithEPP(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: "test/epp:latest"}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	reconcileModelOnce(t, r, "tiny-llm")

	epp := testutil.NewModel("tiny-llm", testNamespace).EPPName()
	for _, check := range []struct {
		name string
		obj  client.Object
	}{
		{"EPP ServiceAccount", &corev1.ServiceAccount{}},
		{"EPP ConfigMap", &corev1.ConfigMap{}},
		{"EPP Service", &corev1.Service{}},
		{"EPP Role", &rbacv1.Role{}},
		{"EPP RoleBinding", &rbacv1.RoleBinding{}},
		{"EPP Deployment", &appsv1.Deployment{}},
	} {
		if err := c.Get(context.Background(), types.NamespacedName{Name: epp, Namespace: testNamespace}, check.obj); err != nil {
			t.Errorf("%s missing: %v", check.name, err)
		}
	}
}

func TestReconcile_ReadyWhenDeploymentReady(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	reconcileModelOnce(t, r, "tiny-llm")

	// Simulate engine pod becoming ready.
	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	dep.Status.ReadyReplicas = 1
	if err := c.Status().Update(context.Background(), dep); err != nil {
		t.Fatalf("status update: %v", err)
	}

	reconcileModelOnce(t, r, "tiny-llm")

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseReady {
		t.Errorf("Phase:\ngot:  %q\nwant: Ready", updated.Status.Phase)
	}
}
