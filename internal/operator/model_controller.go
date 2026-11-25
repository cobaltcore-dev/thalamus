// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"errors"
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources"
)

// ModelReconciler reconciles Model objects.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// APIReader reads directly from the API server.
	APIReader client.Reader
	// GatewayName is the name of the gateway the model routes attach to.
	GatewayName string
	// ListenerName is the gateway listener the model routes attach to.
	ListenerName string
	// RouterServiceAccountName is the ServiceAccount the router pods run as.
	RouterServiceAccountName string
}

// Reconcile applies the desired state and records the observed status.
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	model, err := getObject(ctx, r.Client, req.Namespace, req.Name, &v1alpha1.Model{})
	if err != nil || model == nil {
		return ctrl.Result{}, err
	}

	artifact, err := getObject(ctx, r.APIReader, model.Namespace, model.Spec.ArtifactRef.Name, &v1alpha1.ModelArtifact{})
	if err != nil {
		return ctrl.Result{}, err
	}

	before := model.DeepCopy()
	identifiers := modelIdentifiers(model, artifact)
	usable := isArtifactUsable(artifact) || model.Spec.Replicas == 0
	var applyErr error
	if usable {
		applyErr = r.apply(ctx, model, artifact, identifiers)
	}

	serving, observeErr := r.syncModelStatus(ctx, model, identifiers)

	reconcileErr := errors.Join(applyErr, observeErr)
	if reconcileErr != nil {
		reconcileErr = fmt.Errorf("reconcile model %s: %w", client.ObjectKeyFromObject(model), reconcileErr)
	}

	status := *before.Status.DeepCopy()
	// Publish identifiers only once routing serves them.
	switch {
	case serving.Status == metav1.ConditionTrue:
		status.Identifiers = identifiers
	case model.Spec.Replicas == 0:
		status.Identifiers = nil
	}
	meta.SetStatusCondition(&status.Conditions, artifactReadyCondition(model, artifact))

	ready := serving
	if !usable {
		ready.ObservedGeneration = lastAppliedGeneration(before.Status.Conditions)
	}
	if reconcileErr != nil {
		ready = modelCondition(model, v1alpha1.ModelConditionReady, metav1.ConditionFalse,
			v1alpha1.ModelReasonReconcileError, reconcileErr.Error())
	}
	meta.SetStatusCondition(&status.Conditions, ready)
	model.Status = status

	var patchErr error
	if !apiequality.Semantic.DeepEqual(before.Status, model.Status) {
		log.FromContext(ctx).V(levelInfo).Info("model status changed",
			"namespace", model.Namespace, "name", model.Name, "conditions", model.Status.Conditions)
		if err := r.Status().Patch(ctx, model, client.MergeFrom(before)); err != nil {
			patchErr = fmt.Errorf("patch model status %s: %w", client.ObjectKeyFromObject(model), err)
		}
	}
	log.FromContext(ctx).V(levelDebug).Info("reconciled model",
		"namespace", model.Namespace, "name", model.Name, "conditions", model.Status.Conditions)
	return ctrl.Result{}, errors.Join(reconcileErr, patchErr)
}

func modelIdentifiers(model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) []v1alpha1.Identifier {
	if isArtifactUsable(artifact) {
		return v1alpha1.Identifiers(model.Spec.Aliases, artifact.BaseRepoID())
	}
	if model.Spec.Replicas > 0 {
		return model.Status.Identifiers
	}
	return nil
}

func isArtifactUsable(artifact *v1alpha1.ModelArtifact) bool {
	return artifact != nil &&
		artifact.GetDeletionTimestamp() == nil &&
		artifact.IsReady()
}

// apply creates, updates, or deletes all child resources for the model.
func (r *ModelReconciler) apply(ctx context.Context, model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact, identifiers []v1alpha1.Identifier) error {
	engineDeployment, err := resources.BuildEngineDeployment(model, artifact)
	if err != nil {
		return fmt.Errorf("build engine deployment: %w", err)
	}

	// Engine stack.
	objs := []client.Object{
		engineDeployment,
		resources.BuildEngineService(model),
		resources.BuildInferencePool(model),
		resources.BuildAIBackend(model),
		resources.BuildHTTPRoute(model, identifiers, r.GatewayName, r.ListenerName),
	}

	eppDeployment, err := resources.BuildEPPDeployment(model, r.RouterServiceAccountName)
	if err != nil {
		return fmt.Errorf("build epp deployment: %w", err)
	}

	// EPP stack.
	objs = append(objs,
		resources.BuildEPPConfigMap(model),
		eppDeployment,
		resources.BuildEPPService(model),
	)

	if model.Spec.Replicas == 0 {
		// Delete in reverse order so consumers are removed before their dependencies.
		log.FromContext(ctx).V(levelDebug).Info("deleting owned resources", "namespace", model.Namespace, "name", model.Name, "resources", len(objs))
		for _, obj := range slices.Backward(objs) {
			if err := r.deleteOwned(ctx, model, obj); err != nil {
				return err
			}
		}
		return nil
	}

	for _, obj := range objs {
		if err := applyOwned(ctx, r.Client, r.Scheme, model, obj); err != nil {
			return err
		}
	}
	return nil
}

// deleteOwned removes desired if it exists and is controlled by owner.
func (r *ModelReconciler) deleteOwned(ctx context.Context, owner *v1alpha1.Model, desired client.Object) error {
	obj, err := getObject(ctx, r.Client, desired.GetNamespace(), desired.GetName(), desired)
	if err != nil || obj == nil {
		return err
	}
	if !metav1.IsControlledBy(obj, owner) {
		return fmt.Errorf("refusing to delete %T %s/%s: not owned by model %s", obj, obj.GetNamespace(), obj.GetName(), owner.Name)
	}
	if err := client.IgnoreNotFound(r.Delete(ctx, obj)); err != nil {
		return fmt.Errorf("delete %T %s/%s: %w", obj, obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

// mapArtifactToModels enqueues the models referencing an artifact.
func (r *ModelReconciler) mapArtifactToModels(ctx context.Context, obj client.Object) []reconcile.Request {
	var models v1alpha1.ModelList
	if err := r.List(ctx, &models, client.InNamespace(obj.GetNamespace())); err != nil {
		log.FromContext(ctx).Error(err, "failed to list models for artifact", "namespace", obj.GetNamespace(), "name", obj.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0)
	for i := range models.Items {
		model := &models.Items[i]
		if model.Spec.ArtifactRef.Name != obj.GetName() {
			continue
		}
		requests = append(requests, reconcile.Request{Name: model.Name, Namespace: model.Namespace})
	}
	return requests
}

func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Model{}).
		Watches(&v1alpha1.ModelArtifact{}, handler.EnqueueRequestsFromMapFunc(r.mapArtifactToModels)).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&inferencev1.InferencePool{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Owns(&agentgatewayv1alpha1.AgentgatewayBackend{}).
		Named("model").
		Complete(r)
}
