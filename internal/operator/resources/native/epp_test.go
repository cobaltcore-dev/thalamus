// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"testing"

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
	if len(role.Rules) != 3 {
		t.Errorf("rules:\ngot:  %d\nwant: 3", len(role.Rules))
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
	if _, ok := cm.Data[eppConfigKey]; !ok {
		t.Errorf("missing key %q in ConfigMap", eppConfigKey)
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
}
