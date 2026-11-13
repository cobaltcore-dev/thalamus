// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// ModelArtifactFinalizer holds an artifact open until no Model references it.
const ModelArtifactFinalizer = "thalamus.cloud/artifact-protection"

// ModelArtifactReconciler keeps live ModelArtifacts protected from deletion.
type ModelArtifactReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	APIReader client.Reader
}

// Reconcile adds protection to live artifacts and removes it when deletion is unreferenced.
func (r *ModelArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	artifact, err := getObject(ctx, r.Client, req.Namespace, req.Name, &v1alpha1.ModelArtifact{})
	if err != nil || artifact == nil {
		return ctrl.Result{}, err
	}

	before := artifact.DeepCopy()
	if err := r.syncFinalizer(ctx, artifact); err != nil {
		return ctrl.Result{}, err
	}
	if apiequality.Semantic.DeepEqual(before.Finalizers, artifact.Finalizers) {
		return ctrl.Result{}, nil
	}
	if err := r.Patch(ctx, artifact, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch model artifact %s/%s finalizer: %w", artifact.Namespace, artifact.Name, err)
	}
	if artifact.GetDeletionTimestamp() != nil {
		log.FromContext(ctx).V(levelInfo).Info("releasing model artifact",
			"namespace", artifact.Namespace, "name", artifact.Name)
	}
	return ctrl.Result{}, nil
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
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(r.mapModelToArtifact)).
		Named("model-artifact").
		Complete(r)
}
