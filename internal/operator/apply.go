// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// applyConfiguration converts a typed object into an unstructured ApplyConfiguration for client.Apply.
func applyConfiguration(scheme *runtime.Scheme, obj client.Object) (runtime.ApplyConfiguration, error) {
	obj = obj.DeepCopyObject().(client.Object)
	// SSA requires apiVersion/kind; set them from the scheme before converting.
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return nil, fmt.Errorf("resolve kind for %T: %w", obj, err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvks[0])

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("convert %T to unstructured: %w", obj, err)
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetManagedFields(nil)
	return client.ApplyConfigurationFromUnstructured(u), nil
}

// applyOwned applies desired via SSA with a blocking owner reference.
func applyOwned(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner, desired client.Object) error {
	return applyWithOwner(ctx, c, scheme, owner, desired)
}

// applyOwnedNonBlocking applies desired via SSA with a non-blocking owner reference.
func applyOwnedNonBlocking(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner, desired client.Object) error {
	return applyWithOwner(ctx, c, scheme, owner, desired, controllerutil.WithBlockOwnerDeletion(false))
}

func applyWithOwner(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner, desired client.Object, opts ...controllerutil.OwnerReferenceOption) error {
	desired = desired.DeepCopyObject().(client.Object)
	if err := controllerutil.SetControllerReference(owner, desired, scheme, opts...); err != nil {
		return fmt.Errorf("set controller reference on %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}
	ac, err := applyConfiguration(scheme, desired)
	if err != nil {
		return err
	}
	if err := c.Apply(ctx, ac,
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}
	return nil
}
