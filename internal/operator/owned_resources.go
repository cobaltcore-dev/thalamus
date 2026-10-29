// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// getControlledOrConflict loads obj from the API server. It returns nil when no
// such object exists, and an error when it exists but is not controlled by owner.
func getControlledOrConflict(ctx context.Context, c client.Client, owner, obj client.Object) (client.Object, error) {
	existing := obj.DeepCopyObject().(client.Object)
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), existing); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get existing %T %s/%s: %w", obj, obj.GetNamespace(), obj.GetName(), err)
	}
	if !metav1.IsControlledBy(existing, owner) {
		return nil, fmt.Errorf("not controlled by %s/%s", owner.GetNamespace(), owner.GetName())
	}
	return existing, nil
}

// deleteOwned deletes desired if it exists and is controlled by owner.
func deleteOwned(ctx context.Context, c client.Client, owner, desired client.Object) error {
	existing, err := getControlledOrConflict(ctx, c, owner, desired)
	if err != nil {
		return fmt.Errorf("refusing to delete %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}
	if existing == nil {
		return nil
	}
	if err := client.IgnoreNotFound(c.Delete(ctx, existing)); err != nil {
		return fmt.Errorf("delete %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}
	return nil
}

func applyWithOwner(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner, desired client.Object, opts ...controllerutil.OwnerReferenceOption) error {
	desired = desired.DeepCopyObject().(client.Object)
	if err := controllerutil.SetControllerReference(owner, desired, scheme, opts...); err != nil {
		return fmt.Errorf("set controller reference on %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
	}

	// Never let an SSA ForceOwnership apply take over a resource the owner does
	// not control.
	if _, err := getControlledOrConflict(ctx, c, owner, desired); err != nil {
		return fmt.Errorf("refusing to apply %T %s/%s: %w", desired, desired.GetNamespace(), desired.GetName(), err)
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
