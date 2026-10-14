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
	if c.Image != model.Spec.Serving.Engine.Containers[0].Image {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, model.Spec.Serving.Engine.Containers[0].Image)
	}

	expectedCommand := []string{"vllm", "serve"}
	if !slices.Equal(c.Command, expectedCommand) {
		t.Errorf("Command:\ngot:  %v\nwant: %v", c.Command, expectedCommand)
	}
	expectedArgs := []string{"arnir0/Tiny-LLM", "--max-model-len=512", "--served-model-name=arnir0/Tiny-LLM"}
	if !slices.Equal(c.Args, expectedArgs) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, expectedArgs)
	}

	if c.Env[0].Name != "EXTRA" {
		t.Errorf("env[0]:\ngot:  %q\nwant: EXTRA", c.Env[0].Name)
	}
	if c.Env[1].Name != "HF_TOKEN" {
		t.Errorf("env[1]:\ngot:  %q\nwant: HF_TOKEN", c.Env[1].Name)
	}
	if c.Env[1].ValueFrom.SecretKeyRef.Name != "hf-token" {
		t.Errorf("HF_TOKEN secret:\ngot:  %q\nwant: hf-token", c.Env[1].ValueFrom.SecretKeyRef.Name)
	}
	wantCacheEnv := map[string]string{
		"HOME":           "/cache",
		"HF_HOME":        "/cache/huggingface",
		"XDG_CACHE_HOME": "/cache",
	}
	for i, w := range []string{"HOME", "HF_HOME", "XDG_CACHE_HOME"} {
		if c.Env[i+2].Name != w || c.Env[i+2].Value != wantCacheEnv[w] {
			t.Errorf("env[%d]:\ngot:  %+v\nwant: %s=%s", i+2, c.Env[i+2], w, wantCacheEnv[w])
		}
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
	mountPaths := map[string]string{}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		volNames[v.Name] = true
	}
	for _, m := range c.VolumeMounts {
		mountPaths[m.Name] = m.MountPath
	}
	for _, want := range []string{"cache", "dshm", "tmp"} {
		if !volNames[want] {
			t.Errorf("missing volume %q", want)
		}
	}
	wantMounts := map[string]string{
		"cache": "/cache",
		"dshm":  "/dev/shm",
		"tmp":   "/tmp",
	}
	for name, path := range wantMounts {
		if mountPaths[name] != path {
			t.Errorf("mount %q:\ngot:  %q\nwant: %q", name, mountPaths[name], path)
		}
	}
}

func TestBuildEngineDeployment_Defaults(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine = corev1.PodSpec{}
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Name != "engine" {
		t.Errorf("Name:\ngot:  %q\nwant: engine", c.Name)
	}
	if c.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("ImagePullPolicy:\ngot:  %q\nwant: IfNotPresent", c.ImagePullPolicy)
	}
}

func TestBuildEngineDeployment_UserEnvWins(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers[0].Env = append(model.Spec.Serving.Engine.Containers[0].Env,
		corev1.EnvVar{Name: "HOME", Value: "/root"},
		corev1.EnvVar{Name: "XDG_CACHE_HOME", Value: "/root/.cache"},
	)
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	got := map[string]string{}
	for _, e := range c.Env {
		got[e.Name] = e.Value
	}
	if got["HOME"] != "/root" {
		t.Errorf("HOME:\ngot:  %q\nwant: /root", got["HOME"])
	}
	if got["XDG_CACHE_HOME"] != "/root/.cache" {
		t.Errorf("XDG_CACHE_HOME:\ngot:  %q\nwant: /root/.cache", got["XDG_CACHE_HOME"])
	}
}

func TestBuildEngineDeployment_NonVLLM(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers[0].Image = "test/other-engine:latest"
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	if !slices.Equal(c.Args, []string{"--max-model-len=512"}) {
		t.Errorf("Args:\ngot:  %v\nwant: only the user args, no vllm platform args", c.Args)
	}
	if c.Command != nil {
		t.Errorf("Command:\ngot:  %v\nwant: nil (engine is not vllm)", c.Command)
	}
	if c.StartupProbe != nil || c.LivenessProbe != nil || c.ReadinessProbe != nil {
		t.Error("probes must not be injected for non-vllm engines")
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

func TestBuildEngineDeployment_ServedModelNameOverride(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Containers[0].Args = append(model.Spec.Serving.Engine.Containers[0].Args, "--served-model-name=custom")
	dep := BuildEngineDeployment(model)
	c := dep.Spec.Template.Spec.Containers[0]

	count := 0
	for _, a := range c.Args {
		if a == "--served-model-name=custom" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("--served-model-name occurrences:\ngot:  %d\nwant: 1 (args=%v)", count, c.Args)
	}
	for _, a := range c.Args {
		if a == "--served-model-name=arnir0/Tiny-LLM" {
			t.Errorf("platform --served-model-name must not be added when the user sets one (args=%v)", c.Args)
		}
	}
}

func TestBuildEngineDeploymentSecurity(t *testing.T) {
	dep := BuildEngineDeployment(testutil.NewModel("tiny-llm", "default"))

	podSC := dep.Spec.Template.Spec.SecurityContext
	if podSC == nil {
		t.Fatal("missing pod securityContext")
	}
	if podSC.RunAsNonRoot == nil || !*podSC.RunAsNonRoot {
		t.Error("pod runAsNonRoot must be true")
	}
	if podSC.RunAsUser == nil || *podSC.RunAsUser != wantEngineUID {
		t.Errorf("pod runAsUser:\ngot:  %+v\nwant: %d", podSC.RunAsUser, wantEngineUID)
	}
	if podSC.RunAsGroup == nil || *podSC.RunAsGroup != wantEngineUID {
		t.Errorf("pod runAsGroup:\ngot:  %+v\nwant: %d", podSC.RunAsGroup, wantEngineUID)
	}
	if podSC.FSGroup == nil || *podSC.FSGroup != wantEngineUID {
		t.Errorf("pod fsGroup:\ngot:  %+v\nwant: %d", podSC.FSGroup, wantEngineUID)
	}
	if podSC.SeccompProfile == nil || podSC.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("pod seccompProfile.type:\ngot:  %+v\nwant: RuntimeDefault", podSC.SeccompProfile)
	}

	c := dep.Spec.Template.Spec.Containers[0]
	sc := c.SecurityContext
	if sc == nil {
		t.Fatal("missing container securityContext")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Error("container allowPrivilegeEscalation must be false")
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Error("container readOnlyRootFilesystem must be true")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("container runAsNonRoot must be true")
	}
	if sc.RunAsUser == nil || *sc.RunAsUser != wantEngineUID {
		t.Errorf("container runAsUser:\ngot:  %+v\nwant: %d", sc.RunAsUser, wantEngineUID)
	}
	if sc.RunAsGroup == nil || *sc.RunAsGroup != wantEngineUID {
		t.Errorf("container runAsGroup:\ngot:  %+v\nwant: %d", sc.RunAsGroup, wantEngineUID)
	}
	if sc.Privileged != nil && *sc.Privileged {
		t.Error("container privileged must be false")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("container capabilities.drop:\ngot:  %+v\nwant: [ALL]", sc.Capabilities)
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("container seccompProfile.type:\ngot:  %+v\nwant: RuntimeDefault", sc.SeccompProfile)
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

func TestBuildEngineDeployment_CacheDefaultsToEmptyDir(t *testing.T) {
	dep := BuildEngineDeployment(testutil.NewModel("tiny-llm", "default"))
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "cache" {
			if v.EmptyDir == nil {
				t.Error("expected emptyDir when cache not set")
			}
			return
		}
	}
	t.Error("cache volume not found")
}

func TestBuildEngineDeployment_CachePVC(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Engine.Volumes = []corev1.Volume{
		{
			Name:                  "cache",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-model-cache"},
		},
	}
	dep := BuildEngineDeployment(model)
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "cache" {
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "my-model-cache" {
				t.Errorf("unexpected cache volume source: %+v", v.VolumeSource)
			}
			return
		}
	}
	t.Error("cache volume not found")
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
