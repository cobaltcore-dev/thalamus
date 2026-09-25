// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"encoding/json"
	"sort"
	"strings"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

const (
	ModelListPolicyName = "model-list"
	ModelListRouteName  = "model-list"
	modelListPath       = "/v1/models"
	modelListStatus     = int32(200)

	modelListContentTypeName  = "Content-Type"
	modelListContentTypeValue = "'application/json'"
)

type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type modelListResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

// BuildModelListResponse returns the directResponse JSON body listing only Ready models,
// sorted by model name for a stable output.
func BuildModelListResponse(models []v1alpha1.Model) (string, error) {
	entries := make([]modelEntry, 0, len(models))
	for _, m := range models {
		if m.Status.Phase != v1alpha1.ModelPhaseReady {
			continue
		}
		repoID := ""
		if m.Spec.Weights.HF != nil {
			repoID = m.Spec.Weights.HF.RepoID
		}
		ownedBy := repoID
		if before, _, found := strings.Cut(repoID, "/"); found {
			ownedBy = before
		}
		entries = append(entries, modelEntry{
			ID:      repoID,
			Object:  "model",
			OwnedBy: ownedBy,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	body, err := json.Marshal(modelListResponse{Object: "list", Data: entries})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// BuildModelListRoute returns the HTTPRoute exposing /v1/models on the gateway's API listener.
func BuildModelListRoute(gateway *gatewayv1.Gateway) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		Name:      ModelListRouteName,
		Namespace: gateway.Namespace,
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:        gatewayv1.ObjectName(gateway.Name),
						Namespace:   new(gatewayv1.Namespace(gateway.Namespace)),
						SectionName: new(gatewayv1.SectionName(defaultGatewaySectionName)),
					},
				},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  new(gatewayv1.PathMatchExact),
								Value: new(modelListPath),
							},
						},
					},
				},
			},
		},
	}
}

// BuildModelListPolicy returns the AgentgatewayPolicy serving the /v1/models response body.
func BuildModelListPolicy(namespace, body string) *agentgatewayv1alpha1.AgentgatewayPolicy {
	return &agentgatewayv1alpha1.AgentgatewayPolicy{
		Name:      ModelListPolicyName,
		Namespace: namespace,
		Spec: agentgatewayv1alpha1.AgentgatewayPolicySpec{
			TargetRefs: []agentgatewayv1alpha1.LocalPolicyTargetReferenceWithSectionName{
				{
					Group: gatewayv1.GroupName,
					Kind:  "HTTPRoute",
					Name:  ModelListRouteName,
				},
			},
			Traffic: &agentgatewayv1alpha1.Traffic{
				DirectResponse: &agentgatewayv1alpha1.DirectResponseOrConditional{
					StatusCode: new(modelListStatus),
					Body:       new(body),
					Headers: []agentgatewayv1alpha1.DirectResponseHeader{
						{
							Name:  modelListContentTypeName,
							Value: modelListContentTypeValue,
						},
					},
				},
			},
		},
	}
}
