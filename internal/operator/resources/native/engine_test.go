// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const wantEngineUID = 65534

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
	if c.Name != "engine" {
		t.Errorf("container name:\ngot:  %q\nwant: engine (user name kept)", c.Name)
	}
	if c.Image != model.Spec.Serving.Engine.Containers[0].Image {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, model.Spec.Serving.Engine.Containers[0].Image)
	}
	wantArgs := []string{"arnir0/Tiny-LLM", "--max-model-len=512", "--served-model-name=arnir0/Tiny-LLM"}
	if !slices.Equal(c.Args, wantArgs) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, wantArgs)
	}

	wantEnv := map[string]string{
		"EXTRA":          "val",
		"HOME":           "/cache",
		"HF_HOME":        "/cache/huggingface",
		"XDG_CACHE_HOME": "/cache",
	}
	gotEnv := map[string]string{}
	for _, e := range c.Env {
		gotEnv[e.Name] = e.Value
		if e.Name == "HF_TOKEN" {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil || e.ValueFrom.SecretKeyRef.Name != "hf-token" {
				t.Errorf("HF_TOKEN:\ngot:  %+v\nwant: secretKeyRef hf-token", e)
			}
		}
	}
	for name, want := range wantEnv {
		if gotEnv[name] != want {
			t.Errorf("env %q:\ngot:  %q\nwant: %q", name, gotEnv[name], want)
		}
	}
	if c.Resources.Requests == nil {
		t.Error("Resources.Requests is nil")
	}
	if dep.Spec.Template.Labels["thalamus.cloud/engine"] != model.EngineName() {
		t.Error("missing thalamus.cloud/engine label")
	}
	if c.StartupProbe == nil || c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("missing default probes")
	}
	sc := c.SecurityContext
	if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != wantEngineUID {
		t.Errorf("container securityContext:\ngot:  %+v\nwant: runAsUser %d", sc, wantEngineUID)
	}

	vols := map[string]corev1.Volume{}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		vols[v.Name] = v
	}
	for _, name := range []string{"cache", "dshm", "tmp"} {
		if _, ok := vols[name]; !ok {
			t.Errorf("missing volume %q", name)
		}
	}
	if vols["dshm"].EmptyDir == nil || vols["dshm"].EmptyDir.Medium != corev1.StorageMediumMemory {
		t.Errorf("dshm volume:\ngot:  %+v\nwant: emptyDir medium Memory", vols["dshm"])
	}
	mounts := map[string]string{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m.MountPath
	}
	for name, path := range map[string]string{"cache": "/cache", "dshm": "/dev/shm", "tmp": "/tmp"} {
		if mounts[name] != path {
			t.Errorf("mount %q:\ngot:  %q\nwant: %q", name, mounts[name], path)
		}
	}
}

func TestBuildEngineDeployment_UserOverrides(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	userUID := int64(1234)
	model.Spec.Serving.Engine.Containers[0].SecurityContext = &corev1.SecurityContext{RunAsUser: &userUID}
	model.Spec.Serving.Engine.Volumes = []corev1.Volume{
		{
			Name:                  "cache",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-model-cache"},
		},
	}
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	if c.SecurityContext == nil || c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser != userUID {
		t.Errorf("user securityContext must win:\ngot:  %+v", c.SecurityContext)
	}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "cache" {
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "my-model-cache" {
				t.Errorf("user cache volume must win:\ngot:  %+v", v.VolumeSource)
			}
			return
		}
	}
	t.Error("cache volume not found")
}

func TestBuildEngineDeployment_UserEnvWins(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers[0].Env = append(model.Spec.Serving.Engine.Containers[0].Env,
		corev1.EnvVar{Name: "HOME", Value: "/root"},
	)
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	count := map[string]int{}
	got := map[string]string{}
	for _, e := range c.Env {
		count[e.Name]++
		got[e.Name] = e.Value
	}
	for name, n := range count {
		if n != 1 {
			t.Errorf("env %q appears %d times: %+v", name, n, c.Env)
		}
	}
	if got["HOME"] != "/root" {
		t.Errorf("HOME:\ngot:  %q\nwant: /root", got["HOME"])
	}
}

func TestBuildEngineDeployment_NonVLLM(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers[0].Image = "test/other-engine:latest"
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	if !slices.Equal(c.Args, []string{"--max-model-len=512"}) {
		t.Errorf("Args:\ngot:  %v\nwant: only the user args", c.Args)
	}
	if c.StartupProbe != nil || c.LivenessProbe != nil || c.ReadinessProbe != nil {
		t.Error("vllm probes must not be applied to non-vllm engines")
	}
	hasToken := false
	for _, e := range c.Env {
		if e.Name == "HF_TOKEN" {
			hasToken = true
		}
	}
	if !hasToken {
		t.Error("HF_TOKEN must still be injected for non-vllm engines")
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
	model.Spec.Serving.Engine.NodeSelector = map[string]string{"kubernetes.io/arch": "amd64"}
	dep := BuildEngineDeployment(model)
	if dep.Spec.Template.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Error("NodeSelector not applied")
	}
}

func TestBuildEngineDeployment_NoNodeSelector(t *testing.T) {
	dep := BuildEngineDeployment(testutil.NewModel("tiny-llm", "default"))
	if dep.Spec.Template.Spec.NodeSelector != nil {
		t.Error("expected nil NodeSelector when not set")
	}
}

func TestBuildEngineDeployment_PublicModel(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Weights.HF.TokenSecret = nil
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	for _, e := range c.Env {
		if e.Name == "HF_TOKEN" {
			t.Error("HF_TOKEN must not be injected when TokenSecret is nil")
		}
	}
}

func TestBuildEngineDeployment_SidecarContainer(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers = append(model.Spec.Serving.Engine.Containers,
		corev1.Container{Name: "metrics-agent", Image: "test/agent:latest"})
	dep := BuildEngineDeployment(model)

	if len(dep.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("containers:\ngot:  %d\nwant: 2", len(dep.Spec.Template.Spec.Containers))
	}
	if dep.Spec.Template.Spec.Containers[1].Name != "metrics-agent" {
		t.Errorf("sidecar container:\ngot:  %+v", dep.Spec.Template.Spec.Containers[1])
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
