// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.ModelArtifact{}, &batchv1.Job{}).Build()
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

func TestModelArtifactReconcile_NodeLocalIsReady(t *testing.T) {
	artifact := testutil.NewModelArtifact("weights", testNamespace)
	artifact.Spec.Volume = &v1alpha1.ModelArtifactVolume{}
	r, c := newTestModelArtifactReconciler(t, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)
	reconcileModelArtifactOnce(t, r, artifact.Name)

	updated := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if !updated.IsReady() {
		t.Fatalf("node-local artifact status:\ngot:  %+v", updated.Status)
	}
	if got := updated.Status.Conditions; len(got) != 1 || got[0].Type != "Ready" {
		t.Errorf("artifact must expose only the Ready condition, got %+v", got)
	}
}

func TestModelArtifactReconcile_PrefetchesPVCAndTracksJob(t *testing.T) {
	artifact := testutil.NewModelArtifact("weights", testNamespace)
	artifact.Spec.Volume = &v1alpha1.ModelArtifactVolume{ClaimName: "model-data"}
	artifact.Spec.Draft = new("hf://org/Tiny-Draft")
	r, c := newTestModelArtifactReconciler(t, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name) // persist finalizer before creating resources
	reconcileModelArtifactOnce(t, r, artifact.Name)

	job := &batchv1.Job{}
	testutil.MustGet(t, c, artifact.Name+"-download", testNamespace, job)
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 3 {
		t.Errorf("job backoff limit:\ngot:  %+v\nwant: 3", job.Spec.BackoffLimit)
	}
	container := job.Spec.Template.Spec.Containers[0]
	if !strings.Contains(container.Args[0], "hf download arnir0/Tiny-LLM --local-dir /models/base") {
		t.Errorf("download command:\ngot:  %q", container.Args[0])
	}
	if !strings.Contains(container.Args[0], "hf download org/Tiny-Draft --local-dir /models/draft") {
		t.Errorf("draft download command:\ngot:  %q", container.Args[0])
	}
	mount := container.VolumeMounts[0]
	if mount.Name != "model" || mount.MountPath != "/models" {
		t.Errorf("PVC mount:\ngot:  %+v", mount)
	}
	if job.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName != "model-data" {
		t.Errorf("PVC claim:\ngot:  %+v", job.Spec.Template.Spec.Volumes[0])
	}
	updated := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if updated.IsReady() || updated.Status.Conditions[0].Reason != "Downloading" {
		t.Errorf("downloading artifact status:\ngot:  %+v", updated.Status)
	}
	job.Status.Failed = 1 // A failed attempt is still retryable until JobFailed is True.
	if err := c.Status().Update(context.Background(), job); err != nil {
		t.Fatalf("record retryable download failure: %v", err)
	}
	reconcileModelArtifactOnce(t, r, artifact.Name)
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if updated.IsReady() || updated.Status.Conditions[0].Reason != "Downloading" {
		t.Errorf("retrying artifact status:\ngot:  %+v", updated.Status)
	}

	testutil.MustGet(t, c, artifact.Name+"-download", testNamespace, job)
	job.Status.Succeeded = 1
	if err := c.Status().Update(context.Background(), job); err != nil {
		t.Fatalf("mark download Job succeeded: %v", err)
	}
	reconcileModelArtifactOnce(t, r, artifact.Name)
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if !updated.IsReady() {
		t.Errorf("completed artifact status:\ngot:  %+v", updated.Status)
	}
}

func TestModelArtifactReconcile_PrefetchFailureIsReported(t *testing.T) {
	artifact := testutil.NewModelArtifact("weights", testNamespace)
	artifact.Spec.Volume = &v1alpha1.ModelArtifactVolume{ClaimName: "model-data"}
	r, c := newTestModelArtifactReconciler(t, artifact)
	reconcileModelArtifactOnce(t, r, artifact.Name)
	reconcileModelArtifactOnce(t, r, artifact.Name)

	job := &batchv1.Job{}
	testutil.MustGet(t, c, artifact.Name+"-download", testNamespace, job)
	job.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "download exited with status 1",
	}}
	if err := c.Status().Update(context.Background(), job); err != nil {
		t.Fatalf("mark download Job failed: %v", err)
	}
	reconcileModelArtifactOnce(t, r, artifact.Name)
	updated := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if updated.IsReady() {
		t.Errorf("failed artifact status:\ngot:  %+v", updated.Status)
	}
	condition := updated.Status.Conditions[0]
	if condition.Reason != "DownloadFailed" || condition.Message != "download exited with status 1" {
		t.Errorf("failure condition:\ngot:  %+v", condition)
	}
}

func TestModelArtifactReconcile_ProtectsReferencedArtifact(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)

	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Error("referenced artifact missing the protection finalizer")
	}
}

func TestModelArtifactReconcile_HoldsTerminatingArtifact(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestModelArtifactReconciler(t, model, artifact)

	reconcileModelArtifactOnce(t, r, artifact.Name)
	deleteArtifact(t, c, artifact.Name)

	reconcileModelArtifactOnce(t, r, artifact.Name)

	if !artifactHasFinalizer(t, c, artifact.Name) {
		t.Error("terminating artifact must be held by the protection finalizer while referenced")
	}
}

func TestModelArtifactReconcile_ReleasesWhenModelDeleted(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
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
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
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

	oldModel := testutil.Fixture("tiny-llm", testNamespace).BuildModel()
	newModel := testutil.Fixture("tiny-llm", testNamespace).BuildModel()
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
