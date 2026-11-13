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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// ModelReconciler reconciles Model objects.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// APIReader reads directly from the API server instead of the cache.
	APIReader client.Reader
	// GatewayName is the name of the gateway the model routes attach to.
	GatewayName string
	// ListenerName is the gateway listener the model routes attach to.
	ListenerName string
	// RouterServiceAccountName is the ServiceAccount the router pods run as.
	RouterServiceAccountName string
}

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	model, err := getObject(ctx, r.Client, req.Namespace, req.Name, &v1alpha1.Model{})
	if err != nil || model == nil {
		return ctrl.Result{}, err
	}

	// Capture status before artifact resolution, apply, or observe can change it.
	before := model.DeepCopy()
	artifact, artifactErr := r.resolveArtifact(ctx, model)
	identifiers := modelIdentifiers(model, artifact)

	// Apply only when the artifact is usable or we are tearing down.
	canApply := isArtifactUsable(artifact) || model.Spec.Replicas == 0
	var applyErr error
	if canApply {
		applyErr = r.apply(ctx, model, identifiers, artifact)
	}
	observeCondition, observeErr := r.observe(ctx, model, identifiers)

	reconcileErr := errors.Join(artifactErr, applyErr, observeErr)
	if reconcileErr != nil {
		reconcileErr = fmt.Errorf("reconcile model %s: %w", req.NamespacedName, reconcileErr)
		model.SetReady(metav1.ConditionFalse, v1alpha1.ModelReasonReconcileError, reconcileErr.Error())
	} else {
		if !canApply {
			// The artifact is unusable, so the desired state was not applied.
			// Keep observing serving health, but preserve the generation the
			// Ready condition last reflected instead of claiming the current one.
			observeCondition.ObservedGeneration = 0
			if previous := meta.FindStatusCondition(before.Status.Conditions, string(v1alpha1.ModelConditionReady)); previous != nil {
				observeCondition.ObservedGeneration = previous.ObservedGeneration
			}
		}
		meta.SetStatusCondition(&model.Status.Conditions, observeCondition)
	}

	reconcileErr = errors.Join(reconcileErr, r.patchModelStatus(ctx, before, model))

	if cond := meta.FindStatusCondition(model.Status.Conditions, string(v1alpha1.ModelConditionReady)); cond != nil {
		log.FromContext(ctx).V(levelDebug).Info("reconciled model", "namespace", model.Namespace, "name", model.Name, "ready", cond.Status, "reason", cond.Reason, "message", cond.Message)
	}
	return ctrl.Result{}, reconcileErr
}

// resolveArtifact reads the artifact and updates the Model's ArtifactReady condition.
func (r *ModelReconciler) resolveArtifact(ctx context.Context, model *v1alpha1.Model) (*v1alpha1.ModelArtifact, error) {
	// Read directly so a stale cache cannot hide a deletion.
	artifact, err := getObject(ctx, r.APIReader, model.Namespace, model.Spec.ArtifactRef.Name, &v1alpha1.ModelArtifact{})
	condition := artifactReadyCondition(model, artifact)
	if err != nil {
		condition = modelCondition(model, v1alpha1.ModelConditionArtifactReady, metav1.ConditionUnknown,
			v1alpha1.ModelArtifactReasonReconcileError, err.Error())
	}
	meta.SetStatusCondition(&model.Status.Conditions, condition)
	return artifact, err
}

// modelIdentifiers resolves serving names, retaining the last published names if the artifact is missing.
func modelIdentifiers(model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) []v1alpha1.Identifier {
	if artifact != nil {
		return v1alpha1.Identifiers(model.Spec.Aliases, artifact.BaseRepoID())
	}
	if model.Spec.Replicas > 0 {
		return model.Status.Identifiers
	}
	return nil
}

// isArtifactUsable reports whether the artifact can back new resources.
func isArtifactUsable(artifact *v1alpha1.ModelArtifact) bool {
	return artifact != nil &&
		artifact.GetDeletionTimestamp() == nil &&
		controllerutil.ContainsFinalizer(artifact, ModelArtifactFinalizer)
}

// apply creates, updates, or deletes all child resources for the model.
func (r *ModelReconciler) apply(ctx context.Context, model *v1alpha1.Model, identifiers []v1alpha1.Identifier, artifact *v1alpha1.ModelArtifact) error {
	engineDeployment, err := native.BuildEngineDeployment(model, artifact)
	if err != nil {
		return fmt.Errorf("build engine deployment: %w", err)
	}

	// Engine stack.
	objs := []client.Object{
		engineDeployment,
		native.BuildEngineService(model),
		native.BuildInferencePool(model),
		native.BuildAIBackend(model),
		native.BuildHTTPRoute(model, identifiers, r.GatewayName, r.ListenerName),
	}

	eppDeployment, err := native.BuildEPPDeployment(model, r.RouterServiceAccountName)
	if err != nil {
		return fmt.Errorf("build epp deployment: %w", err)
	}

	// EPP stack.
	objs = append(objs,
		native.BuildEPPConfigMap(model),
		eppDeployment,
		native.BuildEPPService(model),
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

// observe derives the model's ready condition.
func (r *ModelReconciler) observe(ctx context.Context, model *v1alpha1.Model, identifiers []v1alpha1.Identifier) (metav1.Condition, error) {
	model.Status.EngineType = model.DetectedEngineType()
	model.Status.EPPType = model.DetectedEPPType()

	condition, err := r.syncModelStatus(ctx, model, identifiers)
	if err != nil {
		return metav1.Condition{}, fmt.Errorf("sync model status: %w", err)
	}

	// Publish identifiers only once routing serves them.
	switch {
	case condition.Status == metav1.ConditionTrue:
		model.Status.Identifiers = identifiers
	case model.Spec.Replicas == 0:
		model.Status.Identifiers = nil
	}
	return condition, nil
}

// mapArtifactToModels enqueues every Model in the artifact's namespace that references it.
func (r *ModelReconciler) mapArtifactToModels(ctx context.Context, obj client.Object) []reconcile.Request {
	var models v1alpha1.ModelList
	if err := r.List(ctx, &models, client.InNamespace(obj.GetNamespace())); err != nil {
		log.FromContext(ctx).Error(err, "list models for artifact", "namespace", obj.GetNamespace(), "name", obj.GetName())
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
