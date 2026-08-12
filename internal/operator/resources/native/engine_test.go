// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestModelNames(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	if model.EngineName() != "tiny-llm-engine" {
		t.Errorf("EngineName:\ngot:  %q\nwant: tiny-llm-engine", model.EngineName())
	}
	if model.EPPName() != "tiny-llm-epp" {
		t.Errorf("EPPName:\ngot:  %q\nwant: tiny-llm-epp", model.EPPName())
	}
}

func TestBuildEngineDeployment(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	dep := BuildEngineDeployment(model)

	if dep.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", dep.Name, model.EngineName())
	}
	if dep.Namespace != "default" {
		t.Errorf("Namespace:\ngot:  %q\nwant: default", dep.Namespace)
	}

	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != model.Spec.Serving.Engine.Image {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, model.Spec.Serving.Engine.Image)
	}

	expectedCommand := []string{"vllm", "serve"}
	if !slices.Equal(c.Command, expectedCommand) {
		t.Errorf("Command:\ngot:  %v\nwant: %v", c.Command, expectedCommand)
	}
	expectedArgs := []string{"arnir0/Tiny-LLM", "--served-model-name=arnir0/Tiny-LLM", "--max-model-len=512"}
	if !slices.Equal(c.Args, expectedArgs) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, expectedArgs)
	}

	if c.Env[0].Name != "HF_TOKEN" {
		t.Errorf("first env:\ngot:  %q\nwant: HF_TOKEN", c.Env[0].Name)
	}
	if c.Env[0].ValueFrom.SecretKeyRef.Name != "hf-token" {
		t.Errorf("HF_TOKEN secret:\ngot:  %q\nwant: hf-token", c.Env[0].ValueFrom.SecretKeyRef.Name)
	}
	if c.Env[1].Name != "EXTRA" {
		t.Errorf("second env:\ngot:  %q\nwant: EXTRA", c.Env[1].Name)
	}
	if c.Resources.Requests == nil {
		t.Error("Resources.Requests is nil")
	}
	if dep.Spec.Template.Labels["thalamus.cloud/engine"] != model.EngineName() {
		t.Error("missing thalamus.cloud/engine label")
	}
	if c.StartupProbe == nil || c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("missing probes")
	}
	volNames := map[string]bool{}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		volNames[v.Name] = true
	}
	for _, want := range []string{"vllm-cache", "dshm"} {
		if !volNames[want] {
			t.Errorf("missing volume %q", want)
		}
	}
}

func TestBuildEngineDeployment_MultipleReplicas(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Replicas = 3
	dep := BuildEngineDeployment(model)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("Replicas:\ngot:  %v\nwant: 3", dep.Spec.Replicas)
	}
}

func TestBuildEngineDeployment_ZeroReplicas(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Replicas = 0
	dep := BuildEngineDeployment(model)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Errorf("Replicas:\ngot:  %v\nwant: 0", dep.Spec.Replicas)
	}
}

func TestBuildEngineDeployment_NodeSelector(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Scheduling = &v1alpha1.SchedulingSpec{
		NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
	}
	dep := BuildEngineDeployment(model)
	if dep.Spec.Template.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Error("NodeSelector not applied")
	}
}

func TestBuildEngineDeployment_NoScheduling(t *testing.T) {
	dep := BuildEngineDeployment(testutil.NewModel("tiny-llm", "default"))
	if dep.Spec.Template.Spec.NodeSelector != nil {
		t.Error("expected nil NodeSelector when scheduling not set")
	}
}

func TestBuildEngineService(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	svc := BuildEngineService(model)
	if svc.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", svc.Name, model.EngineName())
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != engineHTTPPort {
		t.Error("unexpected service ports")
	}
	if svc.Spec.Selector["thalamus.cloud/engine"] != model.EngineName() {
		t.Error("selector mismatch")
	}
}
