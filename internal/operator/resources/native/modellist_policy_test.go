// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildModelListResponse_Empty(t *testing.T) {
	body := BuildModelListResponse(nil)
	want := `{"object":"list","data":[]}`
	if body != want {
		t.Errorf("model list response:\ngot:  %s\nwant: %s", body, want)
	}
}

func TestBuildModelListResponse_OnlyReady(t *testing.T) {
	a := *testutil.NewModel("tiny-llm", "default")
	a.Name = "a"
	a.Spec.Weights.HF.RepoID = "org/model-a"
	a.Status.Phase = v1alpha1.ModelPhaseReady

	b := *testutil.NewModel("tiny-llm", "default")
	b.Name = "b"
	b.Spec.Weights.HF.RepoID = "org/model-b"
	b.Status.Phase = v1alpha1.ModelPhaseCreating

	c := *testutil.NewModel("tiny-llm", "default")
	c.Name = "c"
	c.Spec.Weights.HF.RepoID = "org/model-c"
	c.Status.Phase = v1alpha1.ModelPhaseFailed

	body := BuildModelListResponse([]v1alpha1.Model{a, b, c})
	want := `{"object":"list","data":[{"id":"org/model-a","object":"model","owned_by":"org"}]}`
	if body != want {
		t.Errorf("model list response:\ngot:  %s\nwant: %s", body, want)
	}
}

func TestBuildModelListResponse_MultipleReady_Sorted(t *testing.T) {
	z := *testutil.NewModel("tiny-llm", "default")
	z.Name = "z"
	z.Spec.Weights.HF.RepoID = "org/zzz"
	z.Status.Phase = v1alpha1.ModelPhaseReady

	a := *testutil.NewModel("tiny-llm", "default")
	a.Name = "a"
	a.Spec.Weights.HF.RepoID = "org/aaa"
	a.Status.Phase = v1alpha1.ModelPhaseReady

	m := *testutil.NewModel("tiny-llm", "default")
	m.Name = "m"
	m.Spec.Weights.HF.RepoID = "org/mmm"
	m.Status.Phase = v1alpha1.ModelPhaseReady

	body := BuildModelListResponse([]v1alpha1.Model{z, a, m})
	want := `{"object":"list","data":[{"id":"org/aaa","object":"model","owned_by":"org"},{"id":"org/mmm","object":"model","owned_by":"org"},{"id":"org/zzz","object":"model","owned_by":"org"}]}`
	if body != want {
		t.Errorf("model list response:\ngot:  %s\nwant: %s", body, want)
	}
}

func TestBuildModelListResponse_NoSlashInRepoID(t *testing.T) {
	model := *testutil.NewModel("tiny-llm", "default")
	model.Spec.Weights.HF.RepoID = "somemodel"
	model.Status.Phase = v1alpha1.ModelPhaseReady

	body := BuildModelListResponse([]v1alpha1.Model{model})
	want := `{"object":"list","data":[{"id":"somemodel","object":"model","owned_by":"somemodel"}]}`
	if body != want {
		t.Errorf("model list response:\ngot:  %s\nwant: %s", body, want)
	}
}

func TestBuildModelListResponse_Stable(t *testing.T) {
	z := *testutil.NewModel("tiny-llm", "default")
	z.Name = "z"
	z.Spec.Weights.HF.RepoID = "org/zzz"
	z.Status.Phase = v1alpha1.ModelPhaseReady

	a := *testutil.NewModel("tiny-llm", "default")
	a.Name = "a"
	a.Spec.Weights.HF.RepoID = "org/aaa"
	a.Status.Phase = v1alpha1.ModelPhaseReady

	models := []v1alpha1.Model{z, a}
	first := BuildModelListResponse(models)
	second := BuildModelListResponse(models)
	if first != second {
		t.Errorf("model list response is not stable:\ngot:  %s\nthen: %s", first, second)
	}
}
