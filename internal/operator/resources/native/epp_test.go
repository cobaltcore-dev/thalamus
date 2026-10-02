// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const testEPPImage = "test/epp:latest"

func TestBuildEPPServiceAccount(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	sa := BuildEPPServiceAccount(model)
	if sa.Name != model.EPPName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", sa.Name, model.EPPName())
	}
}

func TestBuildEPPRole(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	role := BuildEPPRole(model)
	if role.Name != model.EPPName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", role.Name, model.EPPName())
	}
	if len(role.Rules) != 2 {
		t.Errorf("rules:\ngot:  %d\nwant: 2", len(role.Rules))
	}
}

func TestBuildEPPRoleBinding(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	rb := BuildEPPRoleBinding(model)
	if rb.RoleRef.Name != model.EPPName() {
		t.Errorf("RoleRef.Name:\ngot:  %q\nwant: %q", rb.RoleRef.Name, model.EPPName())
	}
	if rb.Subjects[0].Name != model.EPPName() {
		t.Errorf("Subject.Name:\ngot:  %q\nwant: %q", rb.Subjects[0].Name, model.EPPName())
	}
}

func TestBuildEPPConfigMap(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	cm := BuildEPPConfigMap(model)
	cfg, ok := cm.Data[eppConfigKey]
	if !ok {
		t.Fatalf("missing key %q in ConfigMap", eppConfigKey)
	}

	var parsed struct {
		RequestHandler struct {
			Parsers []struct {
				PluginRef string `json:"pluginRef"`
			} `json:"parsers"`
		} `json:"requestHandler"`
	}
	if err := yaml.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("unmarshal EPP config: %v", err)
	}

	// Order matters: first match wins, and a parser with no paths (passthrough)
	// must be last so it acts as the fallback for unclaimed paths.
	want := []string{"openai-parser", "anthropic-parser", "vllmhttp-parser", "passthrough-parser"}
	if len(parsed.RequestHandler.Parsers) != len(want) {
		t.Fatalf("requestHandler.parsers:\ngot:  %+v\nwant: %d entries", parsed.RequestHandler.Parsers, len(want))
	}
	for i, w := range want {
		if got := parsed.RequestHandler.Parsers[i].PluginRef; got != w {
			t.Errorf("requestHandler.parsers[%d]:\ngot:  %s\nwant: %s", i, got, w)
		}
	}
}

func TestBuildEPPDeployment(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	dep := BuildEPPDeployment(model)

	if dep.Name != model.EPPName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", dep.Name, model.EPPName())
	}
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != testEPPImage {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, testEPPImage)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != model.EPPName() {
		t.Errorf("ServiceAccountName:\ngot:  %q\nwant: %q", dep.Spec.Template.Spec.ServiceAccountName, model.EPPName())
	}
	if dep.Spec.Template.Labels["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("missing thalamus.cloud/epp label")
	}
	if dep.Spec.Selector.MatchLabels["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("selector mismatch")
	}
	if c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("missing probes")
	}
}

func TestBuildEPPDeploymentSecurity(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	dep := BuildEPPDeployment(model)

	podSC := dep.Spec.Template.Spec.SecurityContext
	if podSC == nil {
		t.Fatal("missing pod securityContext")
	}
	if podSC.RunAsNonRoot == nil || !*podSC.RunAsNonRoot {
		t.Error("pod runAsNonRoot must be true")
	}
	if podSC.RunAsUser == nil || *podSC.RunAsUser != 65532 {
		t.Errorf("pod runAsUser:\ngot:  %+v\nwant: 65532", podSC.RunAsUser)
	}
	if podSC.RunAsGroup == nil || *podSC.RunAsGroup != 65532 {
		t.Errorf("pod runAsGroup:\ngot:  %+v\nwant: 65532", podSC.RunAsGroup)
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
	if sc.RunAsUser == nil || *sc.RunAsUser != 65532 {
		t.Errorf("container runAsUser:\ngot:  %+v\nwant: 65532", sc.RunAsUser)
	}
	if sc.RunAsGroup == nil || *sc.RunAsGroup != 65532 {
		t.Errorf("container runAsGroup:\ngot:  %+v\nwant: 65532", sc.RunAsGroup)
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

func TestBuildEPPService(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.EPP = &v1alpha1.EPPSpec{Image: testEPPImage}
	svc := BuildEPPService(model)
	if svc.Name != model.EPPName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", svc.Name, model.EPPName())
	}
	ports := map[int32]bool{}
	for _, p := range svc.Spec.Ports {
		ports[p.Port] = true
	}
	for _, want := range []int32{eppGRPCExtProcPort, eppMetricsPort} {
		if !ports[want] {
			t.Errorf("missing port %d", want)
		}
	}
	if svc.Spec.Selector["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("selector mismatch")
	}
	if svc.Labels["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("missing thalamus.cloud/epp metadata label (required for ServiceMonitor selector)")
	}
}
