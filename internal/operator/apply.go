// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// applyConfiguration converts a typed object into an unstructured ApplyConfiguration for client.Apply.
func applyConfiguration(scheme *runtime.Scheme, obj client.Object) (runtime.ApplyConfiguration, error) {
	// SSA requires apiVersion/kind; set them from the scheme before converting.
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	obj.GetObjectKind().SetGroupVersionKind(gvks[0])

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
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
	if err := controllerutil.SetControllerReference(owner, desired, scheme, opts...); err != nil {
		return err
	}
	ac, err := applyConfiguration(scheme, desired)
	if err != nil {
		return err
	}
	return c.Apply(ctx, ac,
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership,
	)
}
