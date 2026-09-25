// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func namePredicate(name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == name
	})
}

// ownedByPredicate fires only for objects controlled by the named owner.
//
//nolint:unparam // kind is a parameter for future non-Gateway owners.
func ownedByPredicate(kind, name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		ref := metav1.GetControllerOf(obj)
		return ref != nil && ref.Kind == kind && ref.Name == name
	})
}
