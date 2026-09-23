// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/event"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func namedGateway(name string) *gatewayv1.Gateway {
	gateway := &gatewayv1.Gateway{Spec: gatewayv1.GatewaySpec{GatewayClassName: "agentgateway"}}
	gateway.Name = name
	return gateway
}

func TestGatewayPredicate(t *testing.T) {
	pred := gatewayPredicate(testGatewayName)

	if !pred.Create(event.CreateEvent{Object: namedGateway(testGatewayName)}) {
		t.Error("create of the inference gateway should fire")
	}
	if pred.Create(event.CreateEvent{Object: namedGateway("other-gateway")}) {
		t.Error("create of another gateway should not fire")
	}
	if !pred.Delete(event.DeleteEvent{Object: namedGateway(testGatewayName)}) {
		t.Error("delete of the inference gateway should fire")
	}
	if pred.Generic(event.GenericEvent{Object: namedGateway("other-gateway")}) {
		t.Error("generic event of another gateway should not fire")
	}

	old := namedGateway(testGatewayName)
	cur := namedGateway(testGatewayName)
	if pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: cur}) {
		t.Error("status-only update should not fire")
	}
	cur.Generation = 2
	if !pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: cur}) {
		t.Error("spec update should fire")
	}
}
