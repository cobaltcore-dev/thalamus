// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"slices"
	"testing"
)

func TestIdentities(t *testing.T) {
	tests := []struct {
		name    string
		aliases []string
		repoID  string
		want    []Identity
	}{
		{
			name:   "no aliases uses the HF repository name verbatim",
			repoID: "arnir0/Tiny-LLM",
			want:   []Identity{{Name: "arnir0/Tiny-LLM", OwnedBy: "arnir0"}},
		},
		{
			name:    "empty alias list uses the HF repository name verbatim",
			aliases: []string{},
			repoID:  "arnir0/Tiny-LLM",
			want:    []Identity{{Name: "arnir0/Tiny-LLM", OwnedBy: "arnir0"}},
		},
		{
			name:    "aliases replace the HF repository name",
			aliases: []string{"tiny", "tiny-latest"},
			repoID:  "arnir0/Tiny-LLM",
			want: []Identity{
				{Name: "tiny", OwnedBy: "arnir0"},
				{Name: "tiny-latest", OwnedBy: "arnir0"},
			},
		},
		{
			name:   "repo ID without an organization owns itself",
			repoID: "somemodel",
			want:   []Identity{{Name: "somemodel", OwnedBy: "somemodel"}},
		},
		{
			name:   "empty repo ID without aliases yields no identities",
			repoID: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &Model{
				Spec: ModelSpec{
					Aliases: tt.aliases,
					Weights: WeightsSpec{
						Type: WeightsTypeHF,
						HF:   &HFWeightsSpec{RepoID: tt.repoID},
					},
				},
			}

			if got := Identities(tt.aliases, tt.repoID); !slices.Equal(got, tt.want) {
				t.Errorf("Identities():\ngot:  %v\nwant: %v", got, tt.want)
			}
			if got := model.BaseModelName(); got != tt.repoID {
				t.Errorf("BaseModelName():\ngot:  %q\nwant: %q", got, tt.repoID)
			}
		})
	}
}
