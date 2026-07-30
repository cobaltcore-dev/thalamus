// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

var agentgatewayPolicyGVK = schema.GroupVersionKind{
	Group:   "agentgateway.dev",
	Version: "v1alpha1",
	Kind:    "AgentgatewayPolicy",
}

// ModelListReconciler keeps the model-list AgentgatewayPolicy in sync
// with the set of Ready Model CRs in each namespace.
type ModelListReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ModelListReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// List all Models in the namespace.
	var modelList v1alpha1.ModelList
	if err := r.List(ctx, &modelList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	body := native.BuildModelListResponse(modelList.Items)

	// Fetch the AgentgatewayPolicy, that provides `/v1/models` via directResponse.
	policy := &unstructured.Unstructured{}
	policy.SetGroupVersionKind(agentgatewayPolicyGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: native.ModelListPolicyName, Namespace: req.Namespace}, policy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if the body already matches to avoid unnecessary updates.
	current, _, err := unstructured.NestedString(policy.Object, "spec", "traffic", "directResponse", "body")
	if err != nil {
		return ctrl.Result{}, err
	}
	if current == body {
		return ctrl.Result{}, nil
	}

	if err := unstructured.SetNestedField(policy.Object, body, "spec", "traffic", "directResponse", "body"); err != nil {
		return ctrl.Result{}, err
	}
	// Server-Side Apply requires managedFields to be nil on the object passed to Apply.
	policy.SetManagedFields(nil)
	if err := r.Apply(ctx,
		client.ApplyConfigurationFromUnstructured(policy),
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership, // required since the policy is initially created using helm
	); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("synced model-list policy", "namespace", req.Namespace, "models", len(modelList.Items))
	return ctrl.Result{}, nil
}

func (r *ModelListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	policyType := &unstructured.Unstructured{}
	policyType.SetGroupVersionKind(agentgatewayPolicyGVK)

	return ctrl.NewControllerManagedBy(mgr).
		// Use a Watch (not For) so the reconcile key is always the fixed model-list name,
		// not the individual Model name.
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      native.ModelListPolicyName,
						Namespace: obj.GetNamespace(),
					},
				}}
			}),
			builder.WithPredicates(phaseChangedPredicate{})).
		Watches(policyType, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				}}
			}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("model-list").
		Complete(r)
}

// phaseChangedPredicate fires only when a Model is created, deleted, or its phase changes to/from Ready.
type phaseChangedPredicate struct{ predicate.Funcs }

func (phaseChangedPredicate) Update(e event.UpdateEvent) bool {
	oldModel, ok1 := e.ObjectOld.(*v1alpha1.Model)
	newModel, ok2 := e.ObjectNew.(*v1alpha1.Model)
	if !ok1 || !ok2 {
		return true
	}
	oldPhase := oldModel.Status.Phase
	newPhase := newModel.Status.Phase
	return oldPhase != newPhase && (oldPhase == v1alpha1.ModelPhaseReady || newPhase == v1alpha1.ModelPhaseReady)
}
