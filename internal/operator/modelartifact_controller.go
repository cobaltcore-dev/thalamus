// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// ModelArtifactFinalizer blocks deletion of artifacts referenced by a Model.
const ModelArtifactFinalizer = "thalamus.cloud/artifact-protection"

// ModelArtifactReconciler protects ModelArtifacts that are referenced by Models.
type ModelArtifactReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile keeps every live artifact protected and releases it once deletion is unreferenced.
func (r *ModelArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	artifact := &v1alpha1.ModelArtifact{}
	if err := r.Get(ctx, req.NamespacedName, artifact); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, fmt.Errorf("get model artifact %s: %w", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	if artifact.GetDeletionTimestamp() == nil {
		return ctrl.Result{}, ensureArtifactFinalizer(ctx, r.Client, artifact)
	}

	if !controllerutil.ContainsFinalizer(artifact, ModelArtifactFinalizer) {
		return ctrl.Result{}, nil
	}
	refs, err := r.countReferencingModels(ctx, artifact)
	if err != nil {
		return ctrl.Result{}, err
	}
	if refs > 0 {
		log.FromContext(ctx).V(levelInfo).Info("holding terminating model artifact",
			"namespace", artifact.Namespace, "name", artifact.Name, "models", refs)
		return ctrl.Result{}, nil
	}

	before := artifact.DeepCopy()
	controllerutil.RemoveFinalizer(artifact, ModelArtifactFinalizer)
	if err := r.Patch(ctx, artifact, client.MergeFrom(before)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch model artifact %s finalizer: %w", req.NamespacedName, err)
	}
	log.FromContext(ctx).V(levelInfo).Info("releasing model artifact",
		"namespace", artifact.Namespace, "name", artifact.Name)
	return ctrl.Result{}, nil
}

// ensureArtifactFinalizer adds the protection finalizer to a live artifact.
func ensureArtifactFinalizer(ctx context.Context, c client.Client, artifact *v1alpha1.ModelArtifact) error {
	before := artifact.DeepCopy()
	if !controllerutil.AddFinalizer(artifact, ModelArtifactFinalizer) {
		return nil
	}
	if err := c.Patch(ctx, artifact, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch model artifact %s/%s finalizer: %w", artifact.Namespace, artifact.Name, err)
	}
	return nil
}

// countReferencingModels counts the Models that reference the artifact.
func (r *ModelArtifactReconciler) countReferencingModels(ctx context.Context, artifact *v1alpha1.ModelArtifact) (int, error) {
	var models v1alpha1.ModelList
	if err := r.List(ctx, &models, client.InNamespace(artifact.Namespace)); err != nil {
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
