// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

const (
	modelListContentTypeHeader = "Content-Type"
	modelListContentTypeValue  = "'application/json'"
	modelListDirectStatus      = int32(200)
)

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

	body, err := native.BuildModelListResponse(modelList.Items)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch the AgentgatewayPolicy that provides `/v1/models` via directResponse.
	policy := &agentgatewayv1alpha1.AgentgatewayPolicy{}
	if err := r.Get(ctx, types.NamespacedName{Name: native.ModelListPolicyName, Namespace: req.Namespace}, policy); err != nil {
		return ctrl.Result{}, err
	}

	// Skip the apply if the direct response already matches to avoid unnecessary updates.
	var dr *agentgatewayv1alpha1.DirectResponseOrConditional
	if policy.Spec.Traffic != nil {
		dr = policy.Spec.Traffic.DirectResponse
	}
	if dr != nil && dr.StatusCode != nil && *dr.StatusCode == modelListDirectStatus &&
		dr.Body != nil && *dr.Body == body {
		for _, h := range dr.Headers {
			if h.Name == modelListContentTypeHeader && h.Value == modelListContentTypeValue {
				return ctrl.Result{}, nil
			}
		}
	}

	// Build a minimal apply object so the operator only claims ownership of the fields it sets.
	patch := &agentgatewayv1alpha1.AgentgatewayPolicy{
		Name:      native.ModelListPolicyName,
		Namespace: req.Namespace,
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			Traffic: &agentgatewayv1alpha1.Traffic{
				DirectResponse: &agentgatewayv1alpha1.DirectResponseOrConditional{
					StatusCode: new(modelListDirectStatus),
					Body:       new(body),
					Headers: []agentgatewayv1alpha1.DirectResponseHeader{
						{
							Name:  modelListContentTypeHeader,
							Value: modelListContentTypeValue,
						},
					},
				},
			},
		},
	}
	ac, err := applyConfiguration(r.Scheme, patch)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Apply(ctx, ac,
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership, // required since helm initially owns the body field
	); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("synced model-list policy", "namespace", req.Namespace, "models", len(modelList.Items))
	return ctrl.Result{}, nil
}

func (r *ModelListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Use a Watch (not For) so the reconcile key is always the fixed model-list name,
		// not the individual Model name.
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					Name:      native.ModelListPolicyName,
					Namespace: obj.GetNamespace(),
				}}
			}),
			builder.WithPredicates(phaseChangedPredicate{})).
		Watches(&agentgatewayv1alpha1.AgentgatewayPolicy{}, &handler.EnqueueRequestForObject{},
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
