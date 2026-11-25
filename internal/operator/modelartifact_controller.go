// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources"
)

// ModelArtifactFinalizer holds an artifact open until no Model references it.
const ModelArtifactFinalizer = "thalamus.cloud/artifact-protection"

// ModelArtifactReconciler fills PVC-backed artifacts and protects live ones from deletion.
type ModelArtifactReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	APIReader client.Reader
}

// Reconcile downloads PVC-backed artifacts and manages artifact protection.
func (r *ModelArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	artifact, err := getObject(ctx, r.Client, req.Namespace, req.Name, &v1alpha1.ModelArtifact{})
	if err != nil || artifact == nil {
		return ctrl.Result{}, err
	}

	before := artifact.DeepCopy()
	if artifact.IsReady() {
		switch {
		case artifact.GetDeletionTimestamp() != nil:
			artifact.SetReady(metav1.ConditionFalse, v1alpha1.ModelArtifactReasonTerminating, "model artifact is being deleted")
		case !controllerutil.ContainsFinalizer(artifact, ModelArtifactFinalizer):
			artifact.SetReady(metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotReady, "artifact protection finalizer is missing")
		}
		if !apiequality.Semantic.DeepEqual(before.Status, artifact.Status) {
			if err := r.Status().Patch(ctx, artifact, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, fmt.Errorf("patch model artifact %s/%s status: %w", artifact.Namespace, artifact.Name, err)
			}
			if artifact.GetDeletionTimestamp() == nil {
				return ctrl.Result{}, nil
			}
			before = artifact.DeepCopy()
		}
	}

	if err := r.syncFinalizer(ctx, artifact); err != nil {
		return ctrl.Result{}, err
	}
	if !apiequality.Semantic.DeepEqual(before.Finalizers, artifact.Finalizers) {
		if err := r.Patch(ctx, artifact, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch model artifact %s/%s finalizer: %w", artifact.Namespace, artifact.Name, err)
		}
		if artifact.GetDeletionTimestamp() != nil {
			log.FromContext(ctx).V(levelInfo).Info("releasing model artifact",
				"namespace", artifact.Namespace, "name", artifact.Name)
		}
		// Persist protection before creating resources.
		return ctrl.Result{}, nil
	}
	if artifact.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}

	status := artifact.DeepCopy().Status
	if !artifact.UsesPVC() {
		artifact.SetReady(metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady, "an init container downloads the weights when the Model pod starts")
	} else {
		if err := r.syncDownloadJob(ctx, artifact); err != nil {
			return ctrl.Result{}, err
		}
		job, err := getObject(ctx, r.Client, artifact.Namespace, artifact.DownloadJobName(), &batchv1.Job{})
		if err != nil {
			return ctrl.Result{}, err
		}
		artifactDownloadStatus(artifact, job)
	}

	if !apiequality.Semantic.DeepEqual(status, artifact.Status) {
		if err := r.Status().Patch(ctx, artifact, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch model artifact %s/%s status: %w", artifact.Namespace, artifact.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

// artifactDownloadStatus derives the Ready condition from the download Job.
func artifactDownloadStatus(artifact *v1alpha1.ModelArtifact, job *batchv1.Job) {
	switch {
	case job == nil:
		artifact.SetReady(metav1.ConditionFalse, v1alpha1.ModelArtifactReasonDownloadPending, "download Job has not started")
	case job.Status.Succeeded > 0:
		artifact.SetReady(metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady, "base and draft weights were downloaded to the PVC")
	case jobFailed(job):
		message := "download Job failed"
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue && condition.Message != "" {
				message = condition.Message
				break
			}
		}
		artifact.SetReady(metav1.ConditionFalse, v1alpha1.ModelArtifactReasonDownloadFailed, message)
	default:
		artifact.SetReady(metav1.ConditionFalse, v1alpha1.ModelArtifactReasonDownloading, "download Job is downloading model weights")
	}
}

// jobFailed reports whether the Job has a True JobFailed condition.
func jobFailed(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// syncDownloadJob applies the artifact's download Job.
func (r *ModelArtifactReconciler) syncDownloadJob(ctx context.Context, artifact *v1alpha1.ModelArtifact) error {
	job := resources.BuildArtifactDownloadJob(artifact, artifact.DownloadJobName())
	if err := applyOwned(ctx, r.Client, r.Scheme, artifact, job); err != nil {
		return fmt.Errorf("apply download Job for model artifact %s/%s: %w", artifact.Namespace, artifact.Name, err)
	}
	return nil
}

// syncFinalizer adds or removes the protection finalizer based on the artifact lifecycle.
func (r *ModelArtifactReconciler) syncFinalizer(ctx context.Context, artifact *v1alpha1.ModelArtifact) error {
	if artifact.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(artifact, ModelArtifactFinalizer)
		return nil
	}
	if !controllerutil.ContainsFinalizer(artifact, ModelArtifactFinalizer) {
		return nil
	}
	refs, err := countModelsReferencingArtifact(ctx, r.APIReader, artifact)
	if err != nil {
		return err
	}
	if refs > 0 {
		log.FromContext(ctx).V(levelInfo).Info("holding terminating model artifact",
			"namespace", artifact.Namespace, "name", artifact.Name, "models", refs)
		return nil
	}
	controllerutil.RemoveFinalizer(artifact, ModelArtifactFinalizer)
	return nil
}

// countModelsReferencingArtifact counts Models referencing the artifact; use an uncached reader.
func countModelsReferencingArtifact(ctx context.Context, reader client.Reader, artifact *v1alpha1.ModelArtifact) (int, error) {
	var models v1alpha1.ModelList
	if err := reader.List(ctx, &models, client.InNamespace(artifact.Namespace)); err != nil {
		return 0, fmt.Errorf("list models in namespace %s: %w", artifact.Namespace, err)
	}
	refs := 0
	for i := range models.Items {
		if models.Items[i].Spec.ArtifactRef.Name == artifact.Name {
			refs++
		}
	}
	return refs, nil
}

// mapModelToArtifact enqueues the ModelArtifact the object references.
func (r *ModelArtifactReconciler) mapModelToArtifact(_ context.Context, obj client.Object) []reconcile.Request {
	model, ok := obj.(*v1alpha1.Model)
	if !ok {
		return nil
	}
	return []reconcile.Request{{
		Name: model.Spec.ArtifactRef.Name, Namespace: model.Namespace,
	}}
}

func (r *ModelArtifactReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModelArtifact{}).
		Owns(&batchv1.Job{}).
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(r.mapModelToArtifact)).
		Named("model-artifact").
		Complete(r)
}
