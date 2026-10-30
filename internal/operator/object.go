// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getObject fetches obj by name/namespace and returns its zero value when absent.
func getObject[T client.Object](ctx context.Context, c client.Client, namespace, name string, obj T) (T, error) {
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		var zero T
		if client.IgnoreNotFound(err) == nil {
			return zero, nil
		}
		return zero, fmt.Errorf("get %T %s/%s: %w", obj, namespace, name, err)
	}
	return obj, nil
}
