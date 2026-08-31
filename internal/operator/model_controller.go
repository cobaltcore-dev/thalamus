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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// ModelReconciler reconciles Model objects.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	model := &v1alpha1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error
	switch model.Spec.Backend {
	case v1alpha1.BackendTypeNative:
		err = r.reconcileNative(ctx, model)
	case v1alpha1.BackendTypeKServe:
		err = r.reconcileKServe(ctx, model)
	case v1alpha1.BackendTypeKAITO:
		err = r.reconcileKAITO(ctx, model)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown backend type %q", model.Spec.Backend)
	}
	if err != nil {
		return ctrl.Result{}, r.setFailedStatus(ctx, model, err)
	}
	return ctrl.Result{}, r.syncStatus(ctx, model)
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
func (r *ModelReconciler) reconcileNative(ctx context.Context, model *v1alpha1.Model) error {
	// Engine stack.
	objs := []client.Object{
		native.BuildEngineDeployment(model),
		native.BuildEngineService(model),
		native.BuildInferencePool(model),
		native.BuildHTTPRoute(model),
	}

	// EPP stack.
	objs = append(objs,
		native.BuildEPPServiceAccount(model),
		native.BuildEPPRole(model),
		native.BuildEPPRoleBinding(model),
		native.BuildEPPConfigMap(model),
		native.BuildEPPDeployment(model),
		native.BuildEPPService(model),
	)

	if model.Spec.Replicas == 0 {
		// Delete in reverse order so consumers are removed before their dependencies.
		for _, obj := range slices.Backward(objs) {
			if err := r.deleteOwned(ctx, model, obj); err != nil {
				return err
			}
		}
		return nil
	}

	for _, obj := range objs {
		if err := r.applyOwned(ctx, model, obj); err != nil {
			return err
		}
	}
	return nil
}

// deleteOwned removes desired if it exists and is controlled by owner.
func (r *ModelReconciler) deleteOwned(ctx context.Context, owner *v1alpha1.Model, desired client.Object) error {
	if err := r.Get(ctx, types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}, desired); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(desired, owner) {
		return fmt.Errorf("refusing to delete %T %s/%s: not owned by model %s", desired, desired.GetNamespace(), desired.GetName(), owner.Name)
	}
	return client.IgnoreNotFound(r.Delete(ctx, desired))
}

// applyConfiguration converts a typed object into an unstructured ApplyConfiguration for client.Apply,
// since no typed apply-config is generated for these CRDs.
func applyConfiguration(scheme *runtime.Scheme, obj client.Object) (runtime.ApplyConfiguration, error) {
	// SSA requires apiVersion/kind; set them from the scheme before converting.
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	obj.GetObjectKind().SetGroupVersionKind(gvks[0])

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetManagedFields(nil)
	return client.ApplyConfigurationFromUnstructured(u), nil
}

// applyOwned sets an owner reference on obj then applies it via Server-Side Apply.
func (r *ModelReconciler) applyOwned(ctx context.Context, model *v1alpha1.Model, desired client.Object) error {
	if err := controllerutil.SetControllerReference(model, desired, r.Scheme); err != nil {
		return err
	}
	ac, err := applyConfiguration(r.Scheme, desired)
	if err != nil {
		return err
	}
	return r.Apply(ctx, ac,
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership,
	)
}

// syncStatus updates common status fields and dispatches to the backend-specific status sync.
func (r *ModelReconciler) syncStatus(ctx context.Context, model *v1alpha1.Model) error {
	patch := client.MergeFrom(model.DeepCopy())

	model.Status.EngineType = model.DetectedEngineType()
	model.Status.EPPType = model.DetectedEPPType()

	var err error
	switch model.Spec.Backend {
	case v1alpha1.BackendTypeNative:
		err = r.syncNativeStatus(ctx, model)
	case v1alpha1.BackendTypeKServe:
		err = r.syncKServeStatus(ctx, model)
	case v1alpha1.BackendTypeKAITO:
		err = r.syncKAITOStatus(ctx, model)
	default:
		return fmt.Errorf("unknown backend type %q", model.Spec.Backend)
	}
	if err != nil {
		return err
	}

	return r.Status().Patch(ctx, model, patch)
}

// syncKServeStatus is a stub — not yet implemented.
func (r *ModelReconciler) syncKServeStatus(_ context.Context, _ *v1alpha1.Model) error {
	return nil
}

// syncKAITOStatus is a stub — not yet implemented.
func (r *ModelReconciler) syncKAITOStatus(_ context.Context, _ *v1alpha1.Model) error {
	return nil
}

// setFailed marks the model as Failed and returns the original error.
func (r *ModelReconciler) setFailedStatus(ctx context.Context, model *v1alpha1.Model, cause error) error {
	patch := client.MergeFrom(model.DeepCopy())
	model.Status.Phase = v1alpha1.ModelPhaseFailed
	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModelConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "ReconcileError",
		Message:            cause.Error(),
		ObservedGeneration: model.Generation,
	})
	if patchErr := r.Status().Patch(ctx, model, patch); patchErr != nil {
		log.FromContext(ctx).Error(patchErr, "failed to patch status after reconcile error")
	}
	return cause
}

func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Model{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&inferencev1.InferencePool{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Named("model").
		Complete(r)
}
