// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// getGateway returns the gateway for nn, or nil when it does not exist or is being deleted.
func getGateway(ctx context.Context, c client.Client, nn types.NamespacedName) (*gatewayv1.Gateway, error) {
	gateway := &gatewayv1.Gateway{}
	err := c.Get(ctx, nn, gateway)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !gateway.DeletionTimestamp.IsZero() {
		return nil, nil
	}
	return gateway, nil
}

// gatewayPredicate fires only for events on the named gateway, ignoring status-only updates.
func gatewayPredicate(name string) predicate.Predicate {
	nameMatch := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == name
	})
	return predicate.And(nameMatch, predicate.GenerationChangedPredicate{})
}
