// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"slices"
	"testing"
)

func TestIdentifiers(t *testing.T) {
	tests := []struct {
		name    string
		aliases []string
		base    string
		want    []Identifier
	}{
		{
			name: "no aliases uses the HF repository name verbatim",
			base: "arnir0/Tiny-LLM",
			want: []Identifier{{Name: "arnir0/Tiny-LLM", OwnedBy: "arnir0"}},
		},
		{
			name:    "empty alias list uses the HF repository name verbatim",
			aliases: []string{},
			base:    "arnir0/Tiny-LLM",
			want:    []Identifier{{Name: "arnir0/Tiny-LLM", OwnedBy: "arnir0"}},
		},
		{
			name:    "aliases replace the HF repository name",
			aliases: []string{"tiny", "tiny-latest"},
			base:    "arnir0/Tiny-LLM",
			want: []Identifier{
				{Name: "tiny", OwnedBy: "arnir0"},
				{Name: "tiny-latest", OwnedBy: "arnir0"},
			},
		},
		{
			name: "repo ID without an organization owns itself",
			base: "somemodel",
			want: []Identifier{{Name: "somemodel", OwnedBy: "somemodel"}},
		},
		{
			name: "empty repo ID without aliases yields no identifiers",
			base: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Identifiers(tt.aliases, tt.base); !slices.Equal(got, tt.want) {
				t.Errorf("Identifiers():\ngot:  %v\nwant: %v", got, tt.want)
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
