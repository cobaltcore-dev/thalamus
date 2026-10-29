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
		base    string
		want    []string
	}{
		{
			name: "no aliases uses the HF repository name verbatim",
			base: "hf://arnir0/Tiny-LLM",
			want: []string{"arnir0/Tiny-LLM"},
		},
		{
			name:    "empty alias list uses the HF repository name verbatim",
			aliases: []string{},
			base:    "hf://arnir0/Tiny-LLM",
			want:    []string{"arnir0/Tiny-LLM"},
		},
		{
			name:    "aliases replace the HF repository name",
			aliases: []string{"tiny", "tiny-latest"},
			base:    "hf://arnir0/Tiny-LLM",
			want:    []string{"tiny", "tiny-latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &Model{
				Spec: ModelSpec{Aliases: tt.aliases},
			}
			artifact := ModelArtifact{
				Spec: ModelArtifactSpec{Base: tt.base},
			}

			if got := model.ModelNames(artifact); !slices.Equal(got, tt.want) {
				t.Errorf("ModelNames():\ngot:  %v\nwant: %v", got, tt.want)
			}
		})
	}
}

func TestModelArtifactRepoIDs(t *testing.T) {
	artifact := &ModelArtifact{
		Spec: ModelArtifactSpec{
			Base:  "hf://arnir0/Tiny-LLM",
			Draft: new("hf://arnir0/Tiny-Draft"),
		},
	}

	if got := artifact.BaseRepoID(); got != "arnir0/Tiny-LLM" {
		t.Errorf("BaseRepoID():\ngot:  %q\nwant: %q", got, "arnir0/Tiny-LLM")
	}
	if got := artifact.DraftRepoID(); got != "arnir0/Tiny-Draft" {
		t.Errorf("DraftRepoID():\ngot:  %q\nwant: %q", got, "arnir0/Tiny-Draft")
	}

	artifact.Spec.Draft = nil
	if got := artifact.DraftRepoID(); got != "" {
		t.Errorf("DraftRepoID() without draft:\ngot:  %q\nwant: %q", got, "")
	}
}
