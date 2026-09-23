// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// namedPredicate fires only for events on the named object, ignoring status-only updates.
func namedPredicate(name string) predicate.Predicate {
	nameMatch := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == name
	})
	return predicate.And(nameMatch, predicate.GenerationChangedPredicate{})
}

// ownedByPredicate fires only for events on objects controlled by the named owner.
func ownedByPredicate(kind, name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		ref := metav1.GetControllerOf(obj)
		return ref != nil && ref.Kind == kind && ref.Name == name
	})
}
