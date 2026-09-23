// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestBuildModelListRoute(t *testing.T) {
	gateway := &gatewayv1.Gateway{Name: testGatewayName, Namespace: "thalamus"}

	route := BuildModelListRoute(gateway)

	if route.Name != ModelListPolicyName {
		t.Errorf("route.Name:\ngot:  %q\nwant: %q", route.Name, ModelListPolicyName)
	}
	if route.Namespace != "thalamus" {
		t.Errorf("route.Namespace:\ngot:  %q\nwant: %q", route.Namespace, "thalamus")
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("len(route.Spec.ParentRefs): %d, want 1", len(route.Spec.ParentRefs))
	}
	parentRef := route.Spec.ParentRefs[0]
	if parentRef.Name != testGatewayName {
		t.Errorf("parentRef.Name:\ngot:  %q\nwant: %q", parentRef.Name, testGatewayName)
	}
	if parentRef.SectionName == nil || *parentRef.SectionName != defaultGatewaySectionName {
		t.Errorf("parentRef.SectionName:\ngot:  %v\nwant: %q", parentRef.SectionName, defaultGatewaySectionName)
	}
	if len(route.Spec.Rules) != 1 || len(route.Spec.Rules[0].Matches) != 1 {
		t.Fatal("expected exactly one route rule with one match")
	}
	path := route.Spec.Rules[0].Matches[0].Path
	if path == nil || path.Type == nil || *path.Type != gatewayv1.PathMatchExact ||
		path.Value == nil || *path.Value != "/v1/models" {
		t.Errorf("path match:\ngot:  %v\nwant: exact /v1/models", path)
	}
}

func TestBuildModelListPolicy(t *testing.T) {
	body := `{"object":"list","data":[]}`

	policy := BuildModelListPolicy("thalamus", body)

	if policy.Name != ModelListPolicyName {
		t.Errorf("policy.Name:\ngot:  %q\nwant: %q", policy.Name, ModelListPolicyName)
	}
	if policy.Namespace != "thalamus" {
		t.Errorf("policy.Namespace:\ngot:  %q\nwant: %q", policy.Namespace, "thalamus")
	}
	if len(policy.Spec.TargetRefs) != 1 {
		t.Fatalf("len(policy.Spec.TargetRefs): %d, want 1", len(policy.Spec.TargetRefs))
	}
	ref := policy.Spec.TargetRefs[0]
	if string(ref.Group) != gatewayv1.GroupName || ref.Kind != "HTTPRoute" || ref.Name != ModelListPolicyName {
		t.Errorf("targetRef:\ngot:  %s/%s %s\nwant: %s/HTTPRoute %s",
			ref.Group, ref.Kind, ref.Name, gatewayv1.GroupName, ModelListPolicyName)
	}
	dr := policy.Spec.Traffic.DirectResponse
	if dr == nil || dr.StatusCode == nil || *dr.StatusCode != modelListStatus || dr.Body == nil || *dr.Body != body {
		t.Errorf("directResponse:\ngot:  %v\nwant: status %d and body %s", dr, modelListStatus, body)
	}
	if len(dr.Headers) != 1 || dr.Headers[0].Name != modelListContentTypeName || dr.Headers[0].Value != modelListContentTypeValue {
		t.Errorf("directResponse headers:\ngot:  %v\nwant: [%s: %s]", dr.Headers, modelListContentTypeName, modelListContentTypeValue)
	}
}

func TestBuildModelListResponse_Empty(t *testing.T) {
	want := `{"object":"list","data":[]}`

	body, err := BuildModelListResponse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if body != want {
		t.Errorf("model list response (nil):\ngot:  %s\nwant: %s", body, want)
	}

	body, err = BuildModelListResponse([]v1alpha1.Model{})
	if err != nil {
		t.Fatal(err)
	}
	if body != want {
		t.Errorf("model list response (empty slice):\ngot:  %s\nwant: %s", body, want)
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

	body, err := BuildModelListResponse([]v1alpha1.Model{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
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

	body, err := BuildModelListResponse([]v1alpha1.Model{z, a, m})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"object":"list","data":[{"id":"org/aaa","object":"model","owned_by":"org"},{"id":"org/mmm","object":"model","owned_by":"org"},{"id":"org/zzz","object":"model","owned_by":"org"}]}`
	if body != want {
		t.Errorf("model list response:\ngot:  %s\nwant: %s", body, want)
	}
}

func TestBuildModelListResponse_NoSlashInRepoID(t *testing.T) {
	model := *testutil.NewModel("tiny-llm", "default")
	model.Spec.Weights.HF.RepoID = "somemodel"
	model.Status.Phase = v1alpha1.ModelPhaseReady

	body, err := BuildModelListResponse([]v1alpha1.Model{model})
	if err != nil {
		t.Fatal(err)
	}
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
	first, err := BuildModelListResponse(models)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildModelListResponse(models)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("model list response is not stable:\ngot:  %s\nthen: %s", first, second)
	}
}
