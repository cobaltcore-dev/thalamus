// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

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
		return ctrl.Result{}, fmt.Errorf("list models in namespace %s: %w", req.Namespace, err)
	}
	var artifactList v1alpha1.ModelArtifactList
	if err := r.List(ctx, &artifactList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list model artifacts in namespace %s: %w", req.Namespace, err)
	}

	body, err := native.BuildModelListResponse(modelList.Items, artifactList.Items)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("build model-list response: %w", err)
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

func (r *ModelListReconciler) mapToGateway(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{
		Name:      r.GatewayName,
		Namespace: obj.GetNamespace(),
	}}
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
		// Models and artifacts are not owned by the gateway, so map their events onto it.
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(r.mapToGateway)).
		Watches(&v1alpha1.ModelArtifact{}, handler.EnqueueRequestsFromMapFunc(r.mapToGateway)).
		Named("model-list").
		Complete(r)
}
