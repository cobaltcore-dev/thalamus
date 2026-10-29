// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

// newTestModelArtifactReconciler returns a reconciler backed by a fake client.
func newTestModelArtifactReconciler(t *testing.T, objs ...client.Object) (*ModelArtifactReconciler, client.Client) {
	t.Helper()
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return &ModelArtifactReconciler{Client: c, Scheme: s, APIReader: c}, c
}

// reconcileModelArtifactOnce reconciles the named artifact once.
func reconcileModelArtifactOnce(t *testing.T, r *ModelArtifactReconciler, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{Name: name, Namespace: testNamespace}); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
}

// artifactHasFinalizer reports whether the artifact carries the protection finalizer.
func artifactHasFinalizer(t *testing.T, c client.Client, name string) bool {
	t.Helper()
	artifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, name, testNamespace, artifact)
	return controllerutil.ContainsFinalizer(artifact, ModelArtifactFinalizer)
}

// deleteArtifact deletes the named artifact via the client.
func deleteArtifact(t *testing.T, c client.Client, name string) {
	t.Helper()
	artifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, name, testNamespace, artifact)
	if err := c.Delete(context.Background(), artifact); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}
}

func TestModelArtifactReconcile_ProtectsUnreferencedArtifact(t *testing.T) {
	artifact := testutil.NewModelArtifact("weights", testNamespace)
	r, c := newTestModelArtifactReconciler(t, artifact)

	reconcileModelArtifactOnce(t, r, "weights")

	if !artifactHasFinalizer(t, c, "weights") {
		t.Error("live artifacts must carry the protection finalizer")
	}
}

func TestModelArtifactReconcile_ProtectsReferencedArtifact(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	artifact := testutil.NewModelArtifact(model.Spec.ArtifactRef.Name, testNamespace)
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)

	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Error("referenced artifact missing the protection finalizer")
	}
}

func TestModelArtifactReconcile_HoldsTerminatingArtifact(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	artifact := testutil.NewModelArtifact(model.Spec.ArtifactRef.Name, testNamespace)
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)
	deleteArtifact(t, c, artifact.Name)

	reconcileModelArtifactOnce(t, r, artifact.Name)

	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Error("terminating artifact must be held by the protection finalizer while referenced")
	}
}

func TestModelArtifactReconcile_ReleasesWhenModelDeleted(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	artifact := testutil.NewModelArtifact(model.Spec.ArtifactRef.Name, testNamespace)
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)
	deleteArtifact(t, c, artifact.Name)
	if err := c.Delete(context.Background(), model); err != nil {
		t.Fatalf("delete model: %v", err)
	}

	reconcileModelArtifactOnce(t, r, artifact.Name)

	testutil.MustNotGet(t, c, artifact.Name, testNamespace, &v1alpha1.ModelArtifact{})
}

func TestModelArtifactReconcile_RetainsProtectionWhenReferenceChanges(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	artifact := testutil.NewModelArtifact(model.Spec.ArtifactRef.Name, testNamespace)
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)
	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Fatal("referenced artifact missing the protection finalizer")
	}

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	updated.Spec.ArtifactRef.Name = "other-artifact"
	if err := c.Update(context.Background(), updated); err != nil {
		t.Fatalf("update model artifactRef: %v", err)
	}

	reconcileModelArtifactOnce(t, r, artifact.Name)
	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Error("live artifacts must retain the protection finalizer")
	}

	deleteArtifact(t, c, artifact.Name)
	reconcileModelArtifactOnce(t, r, artifact.Name)
	testutil.MustNotGet(t, c, artifact.Name, testNamespace, &v1alpha1.ModelArtifact{})
}

func TestModelArtifactReconcile_CompletesUnreferencedDeletion(t *testing.T) {
	artifact := testutil.NewModelArtifact("weights", testNamespace)
	r, c := newTestModelArtifactReconciler(t, artifact)

	reconcileModelArtifactOnce(t, r, "weights")
	deleteArtifact(t, c, "weights")
	reconcileModelArtifactOnce(t, r, "weights")

	testutil.MustNotGet(t, c, "weights", testNamespace, &v1alpha1.ModelArtifact{})
}

func TestModelArtifactReconcile_ConfirmsZeroReferencesWithAPIReader(t *testing.T) {
	s := testutil.NewScheme(t)
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	controllerutil.AddFinalizer(artifact, ModelArtifactFinalizer)

	// The cached client has not observed the Model yet.
	cached := fake.NewClientBuilder().WithScheme(s).WithObjects(artifact).Build()
	apiReader := fake.NewClientBuilder().WithScheme(s).WithObjects(model, artifact).Build()
	if err := cached.Delete(context.Background(), artifact); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}

	r := &ModelArtifactReconciler{Client: cached, APIReader: apiReader, Scheme: s}
	reconcileModelArtifactOnce(t, r, artifact.Name)

	if !artifactHasFinalizer(t, cached, artifact.Name) {
		t.Error("artifact referenced only in the API reader must retain protection")
	}
}

func TestModelArtifactEventHandler_UpdateEnqueuesOldAndNewReferences(t *testing.T) {
	r, _ := newTestModelArtifactReconciler(t)

	oldModel, _ := testutil.Fixture("tiny-llm", testNamespace).Build()
	newModel, _ := testutil.Fixture("tiny-llm", testNamespace).Build()
	newModel.Spec.ArtifactRef.Name = "new-artifact"

	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	handler.EnqueueRequestsFromMapFunc(r.mapModelToArtifact).Update(context.Background(), event.UpdateEvent{
		ObjectOld: oldModel,
		ObjectNew: newModel,
	}, q)

	got := map[string]bool{}
	for q.Len() > 0 {
		req, shutdown := q.Get()
		if shutdown {
			break
		}
		got[req.Name] = true
		q.Done(req)
	}

	for _, name := range []string{oldModel.Spec.ArtifactRef.Name, newModel.Spec.ArtifactRef.Name} {
		if !got[name] {
			t.Errorf("artifact %q was not enqueued; got %v", name, got)
		}
	}
}
