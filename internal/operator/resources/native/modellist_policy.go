// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// ModelListPolicyName is the name of the AgentgatewayPolicy that provides the `/v1/models` endpoint and is kept in sync by the operator.
const ModelListPolicyName = "model-list"

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
func BuildModelListResponse(models []v1alpha1.Model) string {
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
		// unreachable due to modelListResponse containing only strings and slices.
		panic(err)
	}
	return string(body)
}
