// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestOwnedByPredicate(t *testing.T) {
	withOwner := &corev1.ConfigMap{
		Name:      "child",
		Namespace: testNamespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "Gateway",
			Name:       testGatewayName,
			Controller: func() *bool { b := true; return &b }(),
		}}}
	foreignOwned := withOwner.DeepCopy()
	foreignOwned.OwnerReferences[0].Name = "other-gateway"
	wrongKind := withOwner.DeepCopy()
	wrongKind.OwnerReferences[0].Kind = "Model"
	unowned := &corev1.ConfigMap{Name: "child", Namespace: testNamespace}

	p := ownedByPredicate("Gateway", testGatewayName)
	for _, tc := range []struct {
		name string
		obj  *corev1.ConfigMap
		want bool
	}{
		{"controlled by configured gateway", withOwner, true},
		{"controlled by other gateway", foreignOwned, false},
		{"controlled by other kind", wrongKind, false},
		{"no owner", unowned, false},
	} {
		if got := p.Create(event.CreateEvent{Object: tc.obj}); got != tc.want {
			t.Errorf("%s: Create:\ngot:  %v\nwant: %v", tc.name, got, tc.want)
		}
		if got := p.Update(event.UpdateEvent{ObjectNew: tc.obj}); got != tc.want {
			t.Errorf("%s: Update:\ngot:  %v\nwant: %v", tc.name, got, tc.want)
		}
		if got := p.Delete(event.DeleteEvent{Object: tc.obj}); got != tc.want {
			t.Errorf("%s: Delete:\ngot:  %v\nwant: %v", tc.name, got, tc.want)
		}
		if got := p.Generic(event.GenericEvent{Object: tc.obj}); got != tc.want {
			t.Errorf("%s: Generic:\ngot:  %v\nwant: %v", tc.name, got, tc.want)
		}
	}
}
