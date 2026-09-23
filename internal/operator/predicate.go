// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
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
