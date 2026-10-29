// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

const testNamespace = "default"
const testGatewayName = "test-gateway"
const testGatewayListener = "test-listener"

// reconcileModelOnce runs a single reconciliation for the named Model and fails
// the test if Reconcile returns an error.
func reconcileModelOnce(t *testing.T, r *ModelReconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: name, Namespace: testNamespace,
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	return res
}

// withDefaultArtifacts seeds a default ModelArtifact for every Model missing one.
func withDefaultArtifacts(objs []client.Object) []client.Object {
	existing := make(map[types.NamespacedName]bool)
	for _, obj := range objs {
		if _, ok := obj.(*v1alpha1.ModelArtifact); ok {
			existing[types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}] = true
		}
	}
	for _, obj := range objs {
		model, ok := obj.(*v1alpha1.Model)
		if !ok {
			continue
		}
		key := types.NamespacedName{Name: model.Spec.ArtifactRef.Name, Namespace: model.Namespace}
		if existing[key] {
			continue
		}
		existing[key] = true
		objs = append(objs, testutil.NewModelArtifact(key.Name, key.Namespace))
	}
	return objs
}

// newTestReconciler returns a ModelReconciler backed by a fake client seeded
// with the given objects.
func newTestReconciler(t *testing.T, objs ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(withDefaultArtifacts(objs)...).
		WithStatusSubresource(&v1alpha1.Model{}, &appsv1.Deployment{}, &inferencev1.InferencePool{}, &gatewayv1.HTTPRoute{}).
		Build()
	return &ModelReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}, c
}

// readyConditionReason returns the Reason of the model's Ready condition.
func readyConditionReason(model *v1alpha1.Model) string {
	cond := meta.FindStatusCondition(model.Status.Conditions, v1alpha1.ModelConditionReady)
	if cond == nil {
		return ""
	}
	return cond.Reason
}

// readyConditionMessage returns the Message of the model's Ready condition.
func readyConditionMessage(model *v1alpha1.Model) string {
	cond := meta.FindStatusCondition(model.Status.Conditions, v1alpha1.ModelConditionReady)
	if cond == nil {
		return ""
	}
	return cond.Message
}

// assertNativeChildrenDeleted asserts that all native child resources are gone.
func assertNativeChildrenDeleted(t *testing.T, c client.Client, model *v1alpha1.Model) {
	t.Helper()
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &inferencev1.InferencePool{})
	testutil.MustNotGet(t, c, model.Name, testNamespace, &agentgatewayv1alpha1.AgentgatewayBackend{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &corev1.ServiceAccount{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &rbacv1.Role{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &rbacv1.RoleBinding{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &corev1.ConfigMap{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &gatewayv1.HTTPRoute{})
}

// markArtifactTerminating deletes the artifact while its finalizer keeps it readable.
func markArtifactTerminating(t *testing.T, c client.Client, name, namespace string) {
	t.Helper()
	artifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, name, namespace, artifact)
	artifact.Finalizers = []string{ModelArtifactFinalizer}
	if err := c.Update(context.Background(), artifact); err != nil {
		t.Fatalf("add artifact finalizer: %v", err)
	}
	if err := c.Delete(context.Background(), artifact); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}
	testutil.MustGet(t, c, name, namespace, artifact)
	if artifact.GetDeletionTimestamp() == nil {
		t.Fatal("artifact should be terminating with a deletion timestamp")
	}
}

func TestReconcile_ProtectsArtifactBeforeDeploy(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")

	updated := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updated)
	if !controllerutil.ContainsFinalizer(updated, ModelArtifactFinalizer) {
		t.Error("artifact must be protected before the model is deployed")
	}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
}

func TestReconcile_NotFound(t *testing.T) {
	s := testutil.NewScheme(t)
	r := &ModelReconciler{Client: fake.NewClientBuilder().WithScheme(s).Build(), Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}
	reconcileModelOnce(t, r, "missing")
}

func TestReconcile_Native(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &corev1.Service{})
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	backend := &agentgatewayv1alpha1.AgentgatewayBackend{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, backend)
	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, route)
	if got := testutil.HTTPRouteHeaderValues(*route, native.GatewayBaseModelHeaderName); !slices.Equal(got, []string{"arnir0/Tiny-LLM"}) {
		t.Errorf("served names:\ngot:  %q\nwant: %q", got, []string{"arnir0/Tiny-LLM"})
	}

	if len(dep.OwnerReferences) == 0 || dep.OwnerReferences[0].Name != "tiny-llm" {
		t.Error("Deployment missing owner reference to Model")
	}
	if ref := dep.OwnerReferences[0]; ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
		t.Error("Deployment owner reference to Model must block owner deletion")
	}
	if len(backend.OwnerReferences) == 0 || backend.OwnerReferences[0].Name != "tiny-llm" {
		t.Error("AgentgatewayBackend missing owner reference to Model")
	}
}

func TestReconcile_NativeWithDraft(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	artifact.Spec.Draft = new("hf://arnir0/Tiny-Draft")
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)

	wantArgs := []string{
		"arnir0/Tiny-LLM",
		"--max-model-len=512",
		"--spec-model=arnir0/Tiny-Draft",
	}
	if got := dep.Spec.Template.Spec.Containers[0].Args; !slices.Equal(got, wantArgs) {
		t.Errorf("engine args:\ngot:  %v\nwant: %v", got, wantArgs)
	}
}

func TestReconcile_NativeWithAliases(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Aliases = []string{"tiny"}
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, route)
	if got := testutil.HTTPRouteHeaderValues(*route, native.GatewayBaseModelHeaderName); !slices.Equal(got, []string{"tiny"}) {
		t.Errorf("served names:\ngot:  %q\nwant: %q", got, []string{"tiny"})
	}

	wantArgs := []string{
		"arnir0/Tiny-LLM",
		"--max-model-len=512",
	}
	if got := dep.Spec.Template.Spec.Containers[0].Args; !slices.Equal(got, wantArgs) {
		t.Errorf("engine args:\ngot:  %v\nwant: %v", got, wantArgs)
	}

	matches := route.Spec.Rules[0].Matches
	if len(matches) == 0 {
		t.Fatal("route has no matches")
	}
	for i, match := range matches {
		if len(match.Headers) != 1 || match.Headers[0].Value != "tiny" {
			t.Errorf("route match %d headers:\ngot:  %+v\nwant: X-Gateway-Base-Model-Name=tiny", i, match.Headers)
		}
	}
}

func TestReconcile_NativeWithMultipleAliases(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Aliases = []string{"tiny", "tiny-latest"}
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, route)

	matches := route.Spec.Rules[0].Matches
	if len(matches) == 0 {
		t.Fatal("route has no matches")
	}
	seenNames := make(map[string]bool)
	for _, match := range matches {
		if len(match.Headers) != 1 {
			t.Fatalf("unexpected match headers: %+v", match.Headers)
		}
		if !slices.Contains(model.Spec.Aliases, match.Headers[0].Value) {
			t.Errorf("unexpected served name: %q", match.Headers[0].Value)
		}
		seenNames[match.Headers[0].Value] = true
	}
	for _, alias := range model.Spec.Aliases {
		if !seenNames[alias] {
			t.Errorf("missing matches for alias %q", alias)
		}
	}
}

func TestReconcile_NativeUpdatesRouteAliases(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Aliases = []string{"tiny", "old"}
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, route)
	if got := testutil.HTTPRouteHeaderValues(*route, native.GatewayBaseModelHeaderName); !slices.Equal(got, []string{"tiny", "old"}) {
		t.Fatalf("served names before update:\ngot:  %q\nwant: %q", got, []string{"tiny", "old"})
	}

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	updated.Spec.Aliases = []string{"tiny", "new"}
	if err := c.Update(context.Background(), updated); err != nil {
		t.Fatalf("update model aliases: %v", err)
	}

	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, route)
	if got := testutil.HTTPRouteHeaderValues(*route, native.GatewayBaseModelHeaderName); !slices.Equal(got, []string{"tiny", "new"}) {
		t.Errorf("served names after update:\ngot:  %q\nwant: %q", got, []string{"tiny", "new"})
	}
}

func TestReconcile_MissingArtifact(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("tiny-llm", testNamespace)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	// A missing artifact fails the model, no error backoff.
	reconcileModelOnce(t, r, "tiny-llm")

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseFailed {
		t.Errorf("Phase:\ngot:  %q\nwant: Failed", updated.Status.Phase)
	}
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonArtifactNotFound) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonArtifactNotFound)
	}
	if msg := readyConditionMessage(updated); !strings.Contains(msg, "recreate the artifact") || !strings.Contains(msg, "delete the Model") {
		t.Errorf("Ready message should name the recovery options, got: %q", msg)
	}
	// Re-creating the artifact clears the failure (watch-triggered in the cluster).
	artifact := testutil.NewModelArtifact(model.Spec.ArtifactRef.Name, testNamespace)
	if err := c.Create(context.Background(), artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase == v1alpha1.ModelPhaseFailed {
		t.Errorf("Phase:\ngot:  %q\nwant: not Failed after the artifact was created", updated.Status.Phase)
	}
}

// TestReconcile_MissingArtifact_ScaleToZero checks cleanup after the artifact is deleted.
func TestReconcile_MissingArtifact_ScaleToZero(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	// Deploy at one replica.
	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	// Force-remove protection, then delete the artifact and scale to zero.
	updatedArtifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updatedArtifact)
	updatedArtifact.Finalizers = nil
	if err := c.Update(context.Background(), updatedArtifact); err != nil {
		t.Fatalf("remove artifact finalizer: %v", err)
	}
	if err := c.Delete(context.Background(), updatedArtifact); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}
	updatedModel := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updatedModel)
	updatedModel.Spec.Replicas = 0
	if err := c.Update(context.Background(), updatedModel); err != nil {
		t.Fatalf("update model replicas: %v", err)
	}

	reconcileModelOnce(t, r, "tiny-llm")

	assertNativeChildrenDeleted(t, c, model)

	// The model reports Failed, not Inactive: its artifact reference is dangling.
	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseFailed {
		t.Errorf("Phase:\ngot:  %q\nwant: Failed", updated.Status.Phase)
	}
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonArtifactNotFound) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonArtifactNotFound)
	}
}

// TestReconcile_TerminatingArtifact checks failure without disturbing running children.
func TestReconcile_TerminatingArtifact(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})

	markArtifactTerminating(t, c, artifact.Name, testNamespace)

	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseFailed {
		t.Errorf("Phase:\ngot:  %q\nwant: Failed", updated.Status.Phase)
	}
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonArtifactTerminating) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonArtifactTerminating)
	}
	if msg := readyConditionMessage(updated); !strings.Contains(msg, "delete the Model") || !strings.Contains(msg, "spec.artifactRef") {
		t.Errorf("Ready message should name the recovery options, got: %q", msg)
	}
}

// TestReconcile_TerminatingArtifact_ScaleToZero checks cleanup during termination.
func TestReconcile_TerminatingArtifact_ScaleToZero(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")

	markArtifactTerminating(t, c, artifact.Name, testNamespace)

	updatedModel := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updatedModel)
	updatedModel.Spec.Replicas = 0
	if err := c.Update(context.Background(), updatedModel); err != nil {
		t.Fatalf("update model replicas: %v", err)
	}

	reconcileModelOnce(t, r, "tiny-llm")

	assertNativeChildrenDeleted(t, c, model)

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseFailed {
		t.Errorf("Phase:\ngot:  %q\nwant: Failed", updated.Status.Phase)
	}
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonArtifactTerminating) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonArtifactTerminating)
	}
}

func TestMapArtifactToModels(t *testing.T) {
	s := testutil.NewScheme(t)
	model := testutil.NewModel("one", testNamespace)
	other := testutil.NewModel("two", testNamespace)
	other.Spec.ArtifactRef.Name = "other-artifact"
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model, other).Build()
	r := &ModelReconciler{Client: c, Scheme: s}

	requests := r.mapArtifactToModels(context.Background(), testutil.NewModelArtifact("one-artifact", testNamespace))
	if len(requests) != 1 {
		t.Fatalf("len(requests): %d, want 1", len(requests))
	}
	if requests[0].Name != "one" || requests[0].Namespace != testNamespace {
		t.Errorf("request:\ngot:  %s/%s\nwant: %s/%s", requests[0].Namespace, requests[0].Name, testNamespace, "one")
	}
}

func TestReconcile_NativeWithEPP(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Serving.Router = corev1.PodSpec{
		Containers: []corev1.Container{{Name: "router", Image: "test/epp:latest"}},
	}
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	epp := model.EPPName()
	for _, check := range []struct {
		name string
		obj  client.Object
	}{
		{"EPP ServiceAccount", &corev1.ServiceAccount{}},
		{"EPP ConfigMap", &corev1.ConfigMap{}},
		{"EPP Service", &corev1.Service{}},
		{"EPP Role", &rbacv1.Role{}},
		{"EPP RoleBinding", &rbacv1.RoleBinding{}},
		{"EPP Deployment", &appsv1.Deployment{}},
	} {
		if err := c.Get(context.Background(), types.NamespacedName{Name: epp, Namespace: testNamespace}, check.obj); err != nil {
			t.Errorf("%s missing: %v", check.name, err)
		}
	}
}

func TestReconcile_NativeMultipleReplicas(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 3
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, dep)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("engine replicas:\ngot:  %v\nwant: 3", dep.Spec.Replicas)
	}

	// Singleton resources should still exist exactly once.
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustGet(t, c, model.EngineName(), testNamespace, &gatewayv1.HTTPRoute{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &corev1.Service{})
}

func TestReconcile_ScaleToZero(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 0
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	// All child resources should be deleted when scaled to zero.
	assertNativeChildrenDeleted(t, c, model)

	// Model CR should remain and be reported as Inactive.
	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseInactive {
		t.Errorf("Phase:\ngot:  %q\nwant: Inactive", updated.Status.Phase)
	}
}

func TestReconcile_ScaleDownFromOne(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	r, c := newTestReconciler(t, model)

	// Start with replicas=1 and resources created.
	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	// Scale to zero and reconcile again.
	updatedModel := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updatedModel)
	updatedModel.Spec.Replicas = 0
	if err := c.Update(context.Background(), updatedModel); err != nil {
		t.Fatalf("update model replicas: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")

	// Child resources should be gone.
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if updated.Status.Phase != v1alpha1.ModelPhaseInactive {
		t.Errorf("Phase:\ngot:  %q\nwant: Inactive", updated.Status.Phase)
	}
}

func TestReconcile_ScaleToZero_KeepsForeignOwnedObject(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 0

	// A pre-existing engine deployment that is owned by something else.
	controller := true
	foreign := &appsv1.Deployment{
		Name:      "tiny-llm-engine",
		Namespace: testNamespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "foreign-owner",
			UID:        "foreign-uid",
			Controller: &controller,
		}},
	}

	r, c := newTestReconciler(t, model, foreign)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: "tiny-llm", Namespace: testNamespace,
	})
	if err == nil {
		t.Fatal("expected error for foreign-owned colliding resource, got nil")
	}

	// The foreign-owned deployment must not be deleted.
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
}

// failingStatusClient wraps a client so status patches always fail.
type failingStatusClient struct{ client.Client }

func (c failingStatusClient) Status() client.SubResourceWriter {
	return failingStatusWriter{c.Client.Status()}
}

type failingStatusWriter struct{ client.SubResourceWriter }

func (failingStatusWriter) Patch(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
	return errors.New("status patch failed")
}

func TestSetFailedStatus_ReturnsPatchError(t *testing.T) {
	s := testutil.NewScheme(t)
	model, _ := testutil.Fixture("tiny-llm", testNamespace).Build()
	c := failingStatusClient{fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()}
	r := &ModelReconciler{Client: c, Scheme: s}

	err := r.setFailedStatus(context.Background(), model, v1alpha1.ModelReasonArtifactNotFound, "missing")
	if err == nil {
		t.Fatal("expected status patch error")
	}
	if !strings.Contains(err.Error(), "status patch failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestReconcile_MissingArtifact_StatusPatchError ensures patch failures are
// returned so the controller retries instead of dropping the Failed status.
func TestReconcile_MissingArtifact_StatusPatchError(t *testing.T) {
	s := testutil.NewScheme(t)
	model, _ := testutil.Fixture("tiny-llm", testNamespace).Build()
	c := failingStatusClient{fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()}
	r := &ModelReconciler{Client: c, Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{Name: "tiny-llm", Namespace: testNamespace}); err == nil {
		t.Fatal("expected the status patch error to be returned")
	}
}
