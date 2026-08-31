// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// reconcileModelOnce runs a single reconciliation for the named Model and fails
// the test if Reconcile returns an error.
func reconcileModelOnce(t *testing.T, r *ModelReconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: name, Namespace: testNamespace,
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	return res
}

// newTestReconciler returns a ModelReconciler backed by a fake client seeded
// with the given objects.
func newTestReconciler(t *testing.T, objs ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.Model{}, &appsv1.Deployment{}, &inferencev1.InferencePool{}, &gatewayv1.HTTPRoute{}).
		Build()
	return &ModelReconciler{Client: c, Scheme: s}, c
}

func TestReconcile_NotFound(t *testing.T) {
	s := testutil.NewScheme(t)
	r := &ModelReconciler{Client: fake.NewClientBuilder().WithScheme(s).Build(), Scheme: s}
	reconcileModelOnce(t, r, "missing")
}

func TestReconcile_Native(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &corev1.Service{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})

	if len(dep.OwnerReferences) == 0 || dep.OwnerReferences[0].Name != "tiny-llm" {
		t.Error("Deployment missing owner reference to Model")
	}
}

func TestReconcile_NativeWithEPP(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: "test/epp:latest"}
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	epp := model.EPPName()
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

func TestReconcile_NativeMultipleReplicas(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 3
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("engine replicas:\ngot:  %v\nwant: 3", dep.Spec.Replicas)
	}

	// Singleton resources should still exist exactly once.
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &corev1.Service{})
}

func TestReconcile_ScaleToZero(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 0
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	reconcileModelOnce(t, r, "tiny-llm")

	// All child resources should be deleted when scaled to zero.
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &corev1.ServiceAccount{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &rbacv1.Role{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &rbacv1.RoleBinding{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &corev1.ConfigMap{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &corev1.Service{})

	// Model CR should remain and be reported as Inactive.
	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseInactive {
		t.Errorf("Phase:\ngot:  %q\nwant: Inactive", updated.Status.Phase)
	}
}

func TestReconcile_ScaleDownFromOne(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	// Start with replicas=1 and resources created.
	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	// Scale to zero and reconcile again.
	updatedModel := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updatedModel)
	updatedModel.Spec.Replicas = 0
	if err := c.Update(context.Background(), updatedModel); err != nil {
		t.Fatalf("update model replicas: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")

	// Child resources should be gone.
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseInactive {
		t.Errorf("Phase:\ngot:  %q\nwant: Inactive", updated.Status.Phase)
	}
}

func TestReconcile_ScaleToZero_KeepsForeignOwnedObject(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 0

	// A pre-existing engine deployment that is owned by something else.
	controller := true
	foreign := &appsv1.Deployment{
		Name:      "tiny-llm-engine",
		Namespace: testNamespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "foreign-owner",
			UID:        "foreign-uid",
			Controller: &controller,
		}},
	}

	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model, foreign).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: "tiny-llm", Namespace: testNamespace,
	})
	if err == nil {
		t.Fatal("expected error for foreign-owned colliding resource, got nil")
	}

	// The foreign-owned deployment must not be deleted.
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
}
