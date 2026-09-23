// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// ModelListReconciler keeps the /v1/models route and policy in sync, listing the namespace's Ready Models.
type ModelListReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// GatewayName is the name of the gateway the route and policy attach to.
	GatewayName string
	// ListenerName is the gateway listener the route attaches to.
	ListenerName string
}

func (r *ModelListReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gateway, err := getGateway(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if gateway == nil {
		return ctrl.Result{}, nil
	}

	var modelList v1alpha1.ModelList
	if err := r.List(ctx, &modelList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	body, err := native.BuildModelListResponse(modelList.Items)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := applyOwnedNonBlocking(ctx, r.Client, r.Scheme, gateway, native.BuildModelListRoute(gateway, r.ListenerName)); err != nil {
		return ctrl.Result{}, err
	}
	if err := applyOwnedNonBlocking(ctx, r.Client, r.Scheme, gateway, native.BuildModelListPolicy(req.Namespace, body)); err != nil {
		return ctrl.Result{}, err
	}

	log.FromContext(ctx).Info("synced model-list route and policy", "namespace", req.Namespace, "models", len(modelList.Items))
	return ctrl.Result{}, nil
}

func (r *ModelListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}, builder.WithPredicates(predicate.And(
			namePredicate(r.GatewayName),
			predicate.GenerationChangedPredicate{},
		))).
		Owns(&gatewayv1.HTTPRoute{}, builder.WithPredicates(predicate.And(
			namePredicate(native.ModelListRouteName),
			ownedByPredicate("Gateway", r.GatewayName),
		))).
		Owns(&agentgatewayv1alpha1.AgentgatewayPolicy{}, builder.WithPredicates(predicate.And(
			namePredicate(native.ModelListPolicyName),
			ownedByPredicate("Gateway", r.GatewayName),
		))).
		// Models are not owned by the gateway, so map their events onto it.
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					Name:      r.GatewayName,
					Namespace: obj.GetNamespace(),
				}}
			})).
		Named("model-list").
		Complete(r)
}
