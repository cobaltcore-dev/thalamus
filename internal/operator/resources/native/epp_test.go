// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const (
	wantEPPUID       = 65532
	testRouterSAName = "thalamus-router"
)

func TestBuildEPPConfigMap(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
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

func wantEPPContractArgs(model *v1alpha1.Model) []string {
	return []string{
		"--pool-name", model.EngineName(),
		"--pool-namespace", model.Namespace,
		"--pool-group", inferencev1.GroupName,
		"--zap-encoder", "json",
		"--config-file", "/config/" + eppConfigKey,
		"--metrics-endpoint-auth=false",
	}
}

func TestBuildEPPDeployment(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	dep, err := BuildEPPDeployment(model, testRouterSAName)
	if err != nil {
		t.Fatalf("BuildEPPDeployment: %v", err)
	}

	if dep.Name != model.EPPName() {
		t.Errorf("Name:\ngot:  %q\nwant: %q", dep.Name, model.EPPName())
	}
	podSpec := dep.Spec.Template.Spec
	c := podSpec.Containers[0]
	if c.Name != "router" {
		t.Errorf("container name:\ngot:  %q\nwant: router (main container)", c.Name)
	}
	if c.Image != model.Spec.Serving.Router.Containers[0].Image {
		t.Errorf("Image:\ngot:  %q\nwant: %q", c.Image, model.Spec.Serving.Router.Containers[0].Image)
	}
	if !slices.Equal(c.Args, wantEPPContractArgs(model)) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, wantEPPContractArgs(model))
	}
	if podSpec.ServiceAccountName != testRouterSAName {
		t.Errorf("ServiceAccountName:\ngot:  %q\nwant: %q", podSpec.ServiceAccountName, testRouterSAName)
	}
	if podSpec.TerminationGracePeriodSeconds == nil || *podSpec.TerminationGracePeriodSeconds != 130 {
		t.Errorf("TerminationGracePeriodSeconds:\ngot:  %v\nwant: 130", podSpec.TerminationGracePeriodSeconds)
	}
	if dep.Spec.Template.Labels["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("missing thalamus.cloud/epp label")
	}
	if dep.Spec.Selector.MatchLabels["thalamus.cloud/epp"] != model.EPPName() {
		t.Error("selector mismatch")
	}
	if c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("missing default probes")
	}
	sc := c.SecurityContext
	if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != wantEPPUID {
		t.Errorf("container securityContext:\ngot:  %+v\nwant: runAsUser %d", sc, wantEPPUID)
	}
}

func TestBuildEPPDeployment_ServiceAccountIsNotUserOverridable(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Router.ServiceAccountName = "attacker"
	model.Spec.Serving.Router.DeprecatedServiceAccount = "attacker"
	dep, err := BuildEPPDeployment(model, testRouterSAName)
	if err != nil {
		t.Fatalf("BuildEPPDeployment: %v", err)
	}
	if got := dep.Spec.Template.Spec.ServiceAccountName; got != testRouterSAName {
		t.Errorf("ServiceAccountName:\ngot:  %q\nwant: %q", got, testRouterSAName)
	}
	if got := dep.Spec.Template.Spec.DeprecatedServiceAccount; got != "" {
		t.Errorf("DeprecatedServiceAccount:\ngot:  %q\nwant: empty", got)
	}
}

func TestBuildEPPDeployment_AutomountIsNotUserControllable(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Router.AutomountServiceAccountToken = new(false)
	dep, err := BuildEPPDeployment(model, testRouterSAName)
	if err != nil {
		t.Fatalf("BuildEPPDeployment: %v", err)
	}
	if got := dep.Spec.Template.Spec.AutomountServiceAccountToken; got == nil || !*got {
		t.Error("AutomountServiceAccountToken must be true")
	}
}

func TestBuildEPPDeployment_UserArgsBeforeContract(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Router.Containers[0].Args = []string{"--log-level=debug"}
	dep, err := BuildEPPDeployment(model, testRouterSAName)
	if err != nil {
		t.Fatalf("BuildEPPDeployment: %v", err)
	}
	c := dep.Spec.Template.Spec.Containers[0]

	wantArgs := slices.Concat([]string{"--log-level=debug"}, wantEPPContractArgs(model))
	if !slices.Equal(c.Args, wantArgs) {
		t.Errorf("Args:\ngot:  %v\nwant: %v", c.Args, wantArgs)
	}
}

func TestBuildEPPDeployment_UserEnvWins(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
	model.Spec.Serving.Router.Containers[0].Env = []corev1.EnvVar{{Name: "NAMESPACE", Value: "static"}}
	dep, err := BuildEPPDeployment(model, testRouterSAName)
	if err != nil {
		t.Fatalf("BuildEPPDeployment: %v", err)
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
	if got["NAMESPACE"] != "static" {
		t.Errorf("NAMESPACE:\ngot:  %q\nwant: static (user value wins)", got["NAMESPACE"])
	}
}

func TestBuildEPPService(t *testing.T) {
	model := testutil.NewModel("tiny-llm", "default")
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
