// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"slices"
	"testing"
)

func TestModelNames(t *testing.T) {
	tests := []struct {
		name    string
		aliases []string
		repoID  string
		want    []string
	}{
		{
			name:   "no aliases uses the HF repository name verbatim",
			repoID: "arnir0/Tiny-LLM",
			want:   []string{"arnir0/Tiny-LLM"},
		},
		{
			name:    "empty alias list uses the HF repository name verbatim",
			aliases: []string{},
			repoID:  "arnir0/Tiny-LLM",
			want:    []string{"arnir0/Tiny-LLM"},
		},
		{
			name:    "aliases replace the HF repository name",
			aliases: []string{"tiny", "tiny-latest"},
			repoID:  "arnir0/Tiny-LLM",
			want:    []string{"tiny", "tiny-latest"},
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

			if got := model.ModelNames(); !slices.Equal(got, tt.want) {
				t.Errorf("ModelNames():\ngot:  %v\nwant: %v", got, tt.want)
			}
			if got := model.BaseModelName(); got != tt.repoID {
				t.Errorf("BaseModelName():\ngot:  %q\nwant: %q", got, tt.repoID)
			}
		})
	}
}
