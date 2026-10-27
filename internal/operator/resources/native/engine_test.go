// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/helpers"
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
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}

	if dep.Name != model.EngineName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", dep.Name, model.EngineName())
	}
	if dep.Namespace != "default" {
		t.Errorf("Namespace:\ngot:  %q\nwant: default", dep.Namespace)
	}

	c := dep.Spec.Template.Spec.Containers[0]
	if c.Name != "engine" {
		t.Errorf("container name:\ngot:  %q\nwant: engine (main container)", c.Name)
	}
	if c.Image != model.Spec.Serving.Engine.Containers[0].Image {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, model.Spec.Serving.Engine.Containers[0].Image)
	}
	wantArgs := []string{"arnir0/Tiny-LLM", "--max-model-len=512"}
	if !slices.Equal(c.Args, wantArgs) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, wantArgs)
	}

	wantEnv := map[string]string{
		"EXTRA":                           "val",
		"HOME":                            "/cache",
		"HF_HOME":                         "/cache/huggingface",
		"XDG_CACHE_HOME":                  "/cache",
		"VLLM_SKIP_MODEL_NAME_VALIDATION": "1",
	}
	gotEnv := map[string]string{}
	for _, e := range c.Env {
		gotEnv[e.Name] = e.Value
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
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	userUID := int64(1234)
	model.Spec.Serving.Engine.Containers[0].SecurityContext = &corev1.SecurityContext{RunAsUser: &userUID}
	model.Spec.Serving.Engine.Volumes = []corev1.Volume{
		{
			Name:                  "cache",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "my-model-cache"},
		},
	}
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
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
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Serving.Engine.Containers[0].Env = append(model.Spec.Serving.Engine.Containers[0].Env,
		corev1.EnvVar{Name: "HOME", Value: "/root"},
	)
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
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
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Serving.Engine.Containers[0].Image = "test/other-engine:latest"
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	c := dep.Spec.Template.Spec.Containers[0]

	if !slices.Equal(c.Args, []string{"--max-model-len=512"}) {
		t.Errorf("Args:\ngot:  %v\nwant: only the user args", c.Args)
	}
	if c.StartupProbe != nil || c.LivenessProbe != nil || c.ReadinessProbe != nil {
		t.Error("vllm probes must not be applied to non-vllm engines")
	}
}

func TestBuildEngineDeployment_MultipleReplicas(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Replicas = 3
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("Replicas:\ngot:  %v\nwant: 3", dep.Spec.Replicas)
	}
}

func TestBuildEngineDeployment_ZeroReplicas(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Replicas = 0
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Errorf("Replicas:\ngot:  %v\nwant: 0", dep.Spec.Replicas)
	}
}

func TestBuildEngineDeployment_NodeSelector(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Serving.Engine.NodeSelector = map[string]string{"kubernetes.io/arch": "amd64"}
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	if dep.Spec.Template.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Error("NodeSelector not applied")
	}
}

func TestBuildEngineDeployment_NoNodeSelector(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	if dep.Spec.Template.Spec.NodeSelector != nil {
		t.Error("expected nil NodeSelector when not set")
	}
}

func TestBuildEngineDeployment_Draft(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Draft("hf://arnir0/Tiny-Draft").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	c := dep.Spec.Template.Spec.Containers[0]

	want := []string{
		"arnir0/Tiny-LLM",
		"--max-model-len=512",
		"--spec-model=arnir0/Tiny-Draft",
	}
	if !slices.Equal(c.Args, want) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, want)
	}
}

func TestBuildEngineDeployment_Security(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}

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

func TestBuildEngineDeployment_Aliases(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Aliases = []string{"tiny", "tiny-llm"}
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
	c := dep.Spec.Template.Spec.Containers[0]

	want := []string{"arnir0/Tiny-LLM", "--max-model-len=512"}
	if !slices.Equal(c.Args, want) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, want)
	}
	for _, arg := range c.Args {
		if strings.HasPrefix(arg, "--served-model-name") {
			t.Error("engine args must not contain --served-model-name")
		}
	}
}

func TestBuildEngineDeployment_SidecarContainer(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Serving.Engine.Containers = append(model.Spec.Serving.Engine.Containers,
		corev1.Container{Name: "metrics-agent", Image: "test/agent:latest"})
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}

	if len(dep.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("containers:\ngot:  %d\nwant: 2", len(dep.Spec.Template.Spec.Containers))
	}
	if dep.Spec.Template.Spec.Containers[1].Name != "metrics-agent" {
		t.Errorf("sidecar container:\ngot:  %+v", dep.Spec.Template.Spec.Containers[1])
	}
}

func TestBuildEngineDeployment_SidecarBeforeMain(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	model.Spec.Serving.Engine.Containers = append([]corev1.Container{
		{Name: "metrics-agent", Image: "test/agent:latest"},
	}, model.Spec.Serving.Engine.Containers...)
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}

	c := helpers.GetContainerByName(&dep.Spec.Template.Spec, "engine")
	if c == nil {
		t.Fatal("main container engine not found")
	}
	if c.StartupProbe == nil || c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("missing default probes on main container")
	}
	if helpers.GetContainerByName(&dep.Spec.Template.Spec, "metrics-agent") == nil {
		t.Error("sidecar container not found")
	}
}

func TestBuildEngineDeployment_CacheDefaultsToEmptyDir(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", "default").Build()
	dep, err := BuildEngineDeployment(model, artifact)
	if err != nil {
		t.Fatalf("BuildEngineDeployment: %v", err)
	}
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
