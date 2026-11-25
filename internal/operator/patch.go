// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// patchStatusIfChanged patches obj's status subresource against before and reports whether a patch was applied.
func patchStatusIfChanged(ctx context.Context, c client.Client, before, obj client.Object) (bool, error) {
	if apiequality.Semantic.DeepEqual(before, obj) {
		return false, nil
	}
	if err := c.Status().Patch(ctx, obj, client.MergeFrom(before)); err != nil {
		return false, fmt.Errorf("patch %T %s/%s status: %w", obj, obj.GetNamespace(), obj.GetName(), err)
	}
	return true, nil
}

// patchFinalizersIfChanged patches obj's finalizers against before with an optimistic lock and reports whether a patch was applied.
func patchFinalizersIfChanged(ctx context.Context, c client.Client, before, obj client.Object) (bool, error) {
	if apiequality.Semantic.DeepEqual(before.GetFinalizers(), obj.GetFinalizers()) {
		return false, nil
	}
	if err := c.Patch(ctx, obj, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return false, fmt.Errorf("patch %T %s/%s finalizers: %w", obj, obj.GetNamespace(), obj.GetName(), err)
	}
	return true, nil
}
