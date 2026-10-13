// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// getGateway returns the gateway for nn, or nil when it does not exist or is being deleted.
func getGateway(ctx context.Context, c client.Client, nn types.NamespacedName) (*gatewayv1.Gateway, error) {
	gateway := &gatewayv1.Gateway{}
	err := c.Get(ctx, nn, gateway)
	if apierrors.IsNotFound(err) {
		log.FromContext(ctx).V(levelInfo).Info("gateway not found, skipping", "namespace", nn.Namespace, "name", nn.Name)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway %s/%s: %w", nn.Namespace, nn.Name, err)
	}
	if !gateway.DeletionTimestamp.IsZero() {
		log.FromContext(ctx).V(levelInfo).Info("gateway is being deleted, skipping", "namespace", nn.Namespace, "name", nn.Name)
		return nil, nil
	}
	return gateway, nil
}
