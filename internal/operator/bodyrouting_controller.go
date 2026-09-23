// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// BodyRoutingReconciler keeps the body-based routing policy in sync, which per-model routes depend on.
type BodyRoutingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// GatewayName is the name of the gateway the policy attaches to.
	GatewayName string
}

func (r *BodyRoutingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gateway, err := getGateway(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if gateway == nil {
		return ctrl.Result{}, nil
	}

	if err := applyOwned(ctx, r.Client, r.Scheme, gateway, native.BuildBodyBasedRoutingPolicy(gateway)); err != nil {
		return ctrl.Result{}, err
	}

	log.FromContext(ctx).Info("synced body-based routing policy", "namespace", req.Namespace)
	return ctrl.Result{}, nil
}

func (r *BodyRoutingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}, builder.WithPredicates(namedPredicate(r.GatewayName))).
		Owns(&agentgatewayv1alpha1.AgentgatewayPolicy{}, builder.WithPredicates(namedPredicate(native.BodyBasedRoutingPolicyName))).
		Named("body-routing").
		Complete(r)
}
