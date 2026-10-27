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
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// GatewayName is the name of the gateway the model routes attach to.
	GatewayName string
	// ListenerName is the gateway listener the model routes attach to.
	ListenerName string
}

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	model := &v1alpha1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, fmt.Errorf("get model %s: %w", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	// Missing or terminating artifact fails the Model, even at zero replicas.
	artifact := &v1alpha1.ModelArtifact{}
	artifactKey := types.NamespacedName{Name: model.Spec.ArtifactRef.Name, Namespace: model.Namespace}
	var failureReason v1alpha1.ModelReason
	var failureMessage string
	switch err := r.Get(ctx, artifactKey, artifact); {
	case apierrors.IsNotFound(err):
		failureReason = v1alpha1.ModelReasonArtifactNotFound
		failureMessage = fmt.Sprintf("model artifact %s not found; recreate the artifact or delete the Model to resolve this", artifactKey)
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("get model artifact %s: %w", artifactKey, err)
	case artifact.GetDeletionTimestamp() != nil:
		failureReason = v1alpha1.ModelReasonArtifactTerminating
		failureMessage = fmt.Sprintf("model artifact %s is being deleted; delete the Model or update spec.artifactRef to let the deletion complete", artifactKey)
	}

	if failureReason != "" {
		// Teardown only needs names and owner refs, so an empty artifact is safe.
		if model.Spec.Replicas == 0 {
			if err := r.reconcileNative(ctx, model, &v1alpha1.ModelArtifact{}); err != nil {
				return ctrl.Result{}, fmt.Errorf("reconcile model %s: %w", req.NamespacedName, err)
			}
		}
		if err := r.setFailedStatus(ctx, model, failureReason, failureMessage); err != nil {
			return ctrl.Result{}, err
		}
		log.FromContext(ctx).V(levelInfo).Info("model artifact unavailable", "namespace", model.Namespace, "name", model.Name, "reason", failureReason)
		return ctrl.Result{}, nil
	}

	// Protect the artifact before deploying anything that depends on it.
	if err := ensureArtifactFinalizer(ctx, r.Client, artifact); err != nil {
		return ctrl.Result{}, fmt.Errorf("protect model artifact %s: %w", artifactKey, err)
	}

	var err error
	switch model.Spec.Backend {
	case v1alpha1.BackendTypeNative:
		err = r.reconcileNative(ctx, model, artifact)
	case v1alpha1.BackendTypeKServe:
		err = r.reconcileKServe(ctx, model)
	case v1alpha1.BackendTypeKAITO:
		err = r.reconcileKAITO(ctx, model)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown backend type %q", model.Spec.Backend)
	}
	if err != nil {
		statusErr := r.setFailedStatus(ctx, model, v1alpha1.ModelReasonReconcileError, err.Error())
		return ctrl.Result{}, errors.Join(fmt.Errorf("reconcile model %s: %w", req.NamespacedName, err), statusErr)
	}
	if err := r.syncStatus(ctx, model, artifact); err != nil {
		return ctrl.Result{}, err
	}
	log.FromContext(ctx).V(levelDebug).Info("reconciled model", "namespace", model.Namespace, "name", model.Name, "backend", model.Spec.Backend, "phase", model.Status.Phase)
	return ctrl.Result{}, nil
}

// reconcileKServe is a stub — not yet implemented.
func (r *ModelReconciler) reconcileKServe(_ context.Context, _ *v1alpha1.Model) error {
	return errors.New("kserve backend is not yet implemented")
}

// reconcileKAITO is a stub — not yet implemented.
func (r *ModelReconciler) reconcileKAITO(_ context.Context, _ *v1alpha1.Model) error {
	return errors.New("kaito backend is not yet implemented")
}

// reconcileNative creates, updates, or deletes all child resources for the native backend.
func (r *ModelReconciler) reconcileNative(ctx context.Context, model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) error {
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
		native.BuildHTTPRoute(model, artifact, r.GatewayName, r.ListenerName),
	}

	eppDeployment, err := native.BuildEPPDeployment(model)
	if err != nil {
		return fmt.Errorf("build epp deployment: %w", err)
	}

	// EPP stack.
	objs = append(objs,
		native.BuildEPPServiceAccount(model),
		native.BuildEPPRole(model),
		native.BuildEPPRoleBinding(model),
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
	if err := r.Get(ctx, types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}, desired); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			return fmt.Errorf("get %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
		}
		return nil
	}
	if !metav1.IsControlledBy(desired, owner) {
		return fmt.Errorf("refusing to delete %T %s/%s: not owned by model %s", desired, desired.GetNamespace(), desired.GetName(), owner.Name)
	}
	if err := client.IgnoreNotFound(r.Delete(ctx, desired)); err != nil {
		return fmt.Errorf("delete %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}
	return nil
}

// syncStatus updates common status fields and dispatches to the backend-specific status sync.
func (r *ModelReconciler) syncStatus(ctx context.Context, model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) error {
	patch := client.MergeFrom(model.DeepCopy())
	before := model.Status.DeepCopy()

	model.Status.EngineType = model.DetectedEngineType()
	model.Status.EPPType = model.DetectedEPPType()

	var err error
	switch model.Spec.Backend {
	case v1alpha1.BackendTypeNative:
		err = r.syncNativeStatus(ctx, model, artifact)
	case v1alpha1.BackendTypeKServe:
		err = r.syncKServeStatus(ctx, model)
	case v1alpha1.BackendTypeKAITO:
		err = r.syncKAITOStatus(ctx, model)
	default:
		return fmt.Errorf("unknown backend type %q", model.Spec.Backend)
	}
	if err != nil {
		return fmt.Errorf("sync %s status: %w", model.Spec.Backend, err)
	}

	if apiequality.Semantic.DeepEqual(before, &model.Status) {
		return nil
	}
	if cond := meta.FindStatusCondition(model.Status.Conditions, v1alpha1.ModelConditionReady); cond != nil {
		log.FromContext(ctx).V(levelInfo).Info("model status changed", "namespace", model.Namespace, "name", model.Name, "phase", model.Status.Phase, "reason", cond.Reason, "message", cond.Message)
	}
	if err := r.Status().Patch(ctx, model, patch); err != nil {
		return fmt.Errorf("patch model status %s/%s: %w", model.Namespace, model.Name, err)
	}
	return nil
}

// syncKServeStatus is a stub — not yet implemented.
func (r *ModelReconciler) syncKServeStatus(_ context.Context, _ *v1alpha1.Model) error {
	return nil
}

// syncKAITOStatus is a stub — not yet implemented.
func (r *ModelReconciler) syncKAITOStatus(_ context.Context, _ *v1alpha1.Model) error {
	return nil
}

// setFailedStatus marks the model Failed and returns any status-patch error.
func (r *ModelReconciler) setFailedStatus(ctx context.Context, model *v1alpha1.Model, reason v1alpha1.ModelReason, message string) error {
	patch := client.MergeFrom(model.DeepCopy())
	model.Status.Phase = v1alpha1.ModelPhaseFailed
	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModelConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: model.Generation,
	})
	if err := r.Status().Patch(ctx, model, patch); err != nil {
		return fmt.Errorf("patch failed status for model %s/%s: %w", model.Namespace, model.Name, err)
	}
	return nil
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
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&inferencev1.InferencePool{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Owns(&agentgatewayv1alpha1.AgentgatewayBackend{}).
		Named("model").
		Complete(r)
}
