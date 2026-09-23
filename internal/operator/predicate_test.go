// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func namedObject(name string, generation int64) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		Name: name, Generation: generation,
	}
}

func TestNamedPredicate(t *testing.T) {
	pred := namedPredicate("wanted")

	if !pred.Create(event.CreateEvent{Object: namedObject("wanted", 1)}) {
		t.Error("create of a matching name should fire")
	}
	if pred.Create(event.CreateEvent{Object: namedObject("other", 1)}) {
		t.Error("create of another name should not fire")
	}
	if !pred.Delete(event.DeleteEvent{Object: namedObject("wanted", 1)}) {
		t.Error("delete of a matching name should fire")
	}
	if pred.Generic(event.GenericEvent{Object: namedObject("other", 1)}) {
		t.Error("generic event of another name should not fire")
	}
	if pred.Update(event.UpdateEvent{ObjectOld: namedObject("wanted", 1), ObjectNew: namedObject("wanted", 1)}) {
		t.Error("status-only update should not fire")
	}
	if !pred.Update(event.UpdateEvent{ObjectOld: namedObject("wanted", 1), ObjectNew: namedObject("wanted", 2)}) {
		t.Error("spec update should fire")
	}
}
