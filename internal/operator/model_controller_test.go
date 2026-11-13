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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
const testRouterSAName = "thalamus-router"

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
	for _, obj := range objs {
		if artifact, ok := obj.(*v1alpha1.ModelArtifact); ok {
			controllerutil.AddFinalizer(artifact, ModelArtifactFinalizer)
		}
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
	return &ModelReconciler{Client: c, Scheme: s, APIReader: c, GatewayName: testGatewayName, ListenerName: testGatewayListener, RouterServiceAccountName: testRouterSAName}, c
}

func TestReconcile_NotFound(t *testing.T) {
	s := testutil.NewScheme(t)
	r := &ModelReconciler{Client: fake.NewClientBuilder().WithScheme(s).Build(), Scheme: s, GatewayName: testGatewayName, ListenerName: testGatewayListener, RouterServiceAccountName: testRouterSAName}
	reconcileModelOnce(t, r, "missing")
}

func TestReconcile_Native(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	r, c := newTestReconciler(t, model)

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
		{"EPP ConfigMap", &corev1.ConfigMap{}},
		{"EPP Service", &corev1.Service{}},
		{"EPP Deployment", &appsv1.Deployment{}},
	} {
		if err := c.Get(context.Background(), types.NamespacedName{Name: epp, Namespace: testNamespace}, check.obj); err != nil {
			t.Errorf("%s missing: %v", check.name, err)
		}
	}

	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, epp, testNamespace, dep)
	if got := dep.Spec.Template.Spec.ServiceAccountName; got != testRouterSAName {
		t.Errorf("EPP serviceAccountName:\ngot:  %q\nwant: %q", got, testRouterSAName)
	}

	// The identity and RBAC are shared and chart-owned, not Model children.
	testutil.MustNotGet(t, c, epp, testNamespace, &corev1.ServiceAccount{})
	testutil.MustNotGet(t, c, epp, testNamespace, &rbacv1.Role{})
	testutil.MustNotGet(t, c, epp, testNamespace, &rbacv1.RoleBinding{})
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
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &corev1.Service{})
}

func TestReconcile_ScaleToZero(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Spec.Replicas = 0
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	// All child resources should be deleted when scaled to zero.
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &inferencev1.InferencePool{})
	testutil.MustNotGet(t, c, "tiny-llm", testNamespace, &agentgatewayv1alpha1.AgentgatewayBackend{})
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &gatewayv1.HTTPRoute{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &corev1.ConfigMap{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, "tiny-llm-epp", testNamespace, &corev1.Service{})

	// Model CR should remain and be reported as not ready.
	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	cond := meta.FindStatusCondition(updated.Status.Conditions, string(v1alpha1.ModelConditionReady))
	if cond == nil {
		t.Fatal("Ready condition not found on model")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("Ready condition status:\ngot:  %q\nwant: %q", cond.Status, metav1.ConditionFalse)
	}
	if cond.Reason != string(v1alpha1.ModelReasonNoReplicasDesired) {
		t.Errorf("Ready condition reason:\ngot:  %q\nwant: %q", cond.Reason, v1alpha1.ModelReasonNoReplicasDesired)
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
	cond := meta.FindStatusCondition(updated.Status.Conditions, string(v1alpha1.ModelConditionReady))
	if cond == nil {
		t.Fatal("Ready condition not found on model")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("Ready condition status:\ngot:  %q\nwant: %q", cond.Status, metav1.ConditionFalse)
	}
	if cond.Reason != string(v1alpha1.ModelReasonNoReplicasDesired) {
		t.Errorf("Ready condition reason:\ngot:  %q\nwant: %q", cond.Reason, v1alpha1.ModelReasonNoReplicasDesired)
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

func TestReconcile_PublishesIdentifiersOnlyWhenReady(t *testing.T) {
	model := testutil.NewModel("tiny-llm", testNamespace)
	model.Status.Identifiers = nil
	r, c := newTestReconciler(t, model)

	reconcileModelOnce(t, r, "tiny-llm")

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if len(updated.Status.Identifiers) != 0 {
		t.Errorf("identifiers before Ready:\ngot:  %v\nwant: none", updated.Status.Identifiers)
	}

	markDeploymentReady(t, c, model.EngineName())
	markDeploymentReady(t, c, model.EPPName())
	markInferencePoolReady(t, c, model.EngineName())
	markHTTPRouteReady(t, c, model.EngineName())
	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	want := v1alpha1.Identifiers(model.Spec.Aliases, testutil.DefaultModelBaseRepoID)
	if !slices.Equal(updated.Status.Identifiers, want) {
		t.Errorf("identifiers after Ready:\ngot:  %v\nwant: %v", updated.Status.Identifiers, want)
	}

	updated.Spec.Replicas = 0
	if err := c.Update(context.Background(), updated); err != nil {
		t.Fatalf("scale to zero: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	if len(updated.Status.Identifiers) != 0 {
		t.Errorf("identifiers after scale to zero:\ngot:  %v\nwant: none", updated.Status.Identifiers)
	}
}

func conditionReason(model *v1alpha1.Model, conditionType v1alpha1.ModelConditionType) string {
	cond := meta.FindStatusCondition(model.Status.Conditions, string(conditionType))
	if cond == nil {
		return ""
	}
	return cond.Reason
}

func readyConditionReason(model *v1alpha1.Model) string {
	return conditionReason(model, v1alpha1.ModelConditionReady)
}

func conditionMessage(model *v1alpha1.Model, conditionType v1alpha1.ModelConditionType) string {
	cond := meta.FindStatusCondition(model.Status.Conditions, string(conditionType))
	if cond == nil {
		return ""
	}
	return cond.Message
}

func assertModelCondition[R ~string](t *testing.T, model *v1alpha1.Model, conditionType v1alpha1.ModelConditionType, status metav1.ConditionStatus, reason R) {
	t.Helper()
	cond := meta.FindStatusCondition(model.Status.Conditions, string(conditionType))
	if cond == nil {
		t.Fatalf("%s condition not found on model", conditionType)
	}
	if cond.Status != status || cond.Reason != string(reason) {
		t.Errorf("%s condition: status=%s reason=%s, want status=%s reason=%s", conditionType, cond.Status, cond.Reason, status, reason)
	}
	if cond.ObservedGeneration != model.Generation {
		t.Errorf("%s observedGeneration: %d, want %d", conditionType, cond.ObservedGeneration, model.Generation)
	}
}

func assertNativeChildrenDeleted(t *testing.T, c client.Client, model *v1alpha1.Model) {
	t.Helper()
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &inferencev1.InferencePool{})
	testutil.MustNotGet(t, c, model.Name, testNamespace, &agentgatewayv1alpha1.AgentgatewayBackend{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &corev1.ConfigMap{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &appsv1.Deployment{})
	testutil.MustNotGet(t, c, model.EPPName(), testNamespace, &corev1.Service{})
	testutil.MustNotGet(t, c, model.EngineName(), testNamespace, &gatewayv1.HTTPRoute{})
}

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

func removeArtifact(t *testing.T, c client.Client, name, namespace string) {
	t.Helper()
	artifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, name, namespace, artifact)
	artifact.Finalizers = nil
	if err := c.Update(context.Background(), artifact); err != nil {
		t.Fatalf("remove artifact finalizer: %v", err)
	}
	if err := c.Delete(context.Background(), artifact); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}
}

func TestReconcile_WaitsForArtifactProtection(t *testing.T) {
	s := testutil.NewScheme(t)
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(model, artifact).
		WithStatusSubresource(&v1alpha1.Model{}).
		Build()
	r := &ModelReconciler{Client: c, Scheme: s, APIReader: c, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelOnce(t, r, "tiny-llm")

	updatedArtifact := &v1alpha1.ModelArtifact{}
	testutil.MustGet(t, c, artifact.Name, testNamespace, updatedArtifact)
	if controllerutil.ContainsFinalizer(updatedArtifact, ModelArtifactFinalizer) {
		t.Error("model controller must not add the artifact finalizer")
	}
	testutil.MustNotGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonEngineNotReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotReady)

	controllerutil.AddFinalizer(updatedArtifact, ModelArtifactFinalizer)
	if err := c.Update(context.Background(), updatedArtifact); err != nil {
		t.Fatalf("protect artifact: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonEngineNotReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady)
}

func TestReconcile_MissingArtifact(t *testing.T) {
	s := testutil.NewScheme(t)
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(model).WithStatusSubresource(model).Build()
	r := &ModelReconciler{Client: c, Scheme: s, APIReader: c, GatewayName: testGatewayName, ListenerName: testGatewayListener}

	reconcileModelOnce(t, r, "tiny-llm")

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonEngineNotReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotFound)
	if msg := conditionMessage(updated, v1alpha1.ModelConditionArtifactReady); !strings.Contains(msg, "recreate the artifact") || !strings.Contains(msg, "delete the Model") {
		t.Errorf("ArtifactReady message should name the recovery options, got: %q", msg)
	}

	controllerutil.AddFinalizer(artifact, ModelArtifactFinalizer)
	if err := c.Create(context.Background(), artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady)
}

func TestReconcile_MissingArtifact_KeepsServing(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")
	markDeploymentReady(t, c, model.EngineName())
	markDeploymentReady(t, c, model.EPPName())
	markInferencePoolReady(t, c, model.EngineName())
	markHTTPRouteReady(t, c, model.EngineName())
	reconcileModelOnce(t, r, "tiny-llm")

	removeArtifact(t, c, artifact.Name, testNamespace)
	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionTrue, v1alpha1.ModelReasonReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotFound)
	if !slices.Equal(updated.Status.Identifiers, model.Status.Identifiers) {
		t.Errorf("identifiers after artifact removal: %v, want %v", updated.Status.Identifiers, model.Status.Identifiers)
	}

	testutil.UpdateStatus(t, c, model.EngineName(), testNamespace, &appsv1.Deployment{}, func(dep *appsv1.Deployment) {
		dep.Status.ReadyReplicas = 0
	})
	reconcileModelOnce(t, r, model.Name)
	testutil.MustGet(t, c, model.Name, testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonEngineNotReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotFound)
}

func TestReconcile_MissingArtifact_DoesNotReportUnappliedChanges(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")
	markDeploymentReady(t, c, model.EngineName())
	markDeploymentReady(t, c, model.EPPName())
	markInferencePoolReady(t, c, model.EngineName())
	markHTTPRouteReady(t, c, model.EngineName())
	reconcileModelOnce(t, r, "tiny-llm")

	applied := &v1alpha1.Model{}
	testutil.MustGet(t, c, model.Name, testNamespace, applied)
	assertModelCondition(t, applied, v1alpha1.ModelConditionReady, metav1.ConditionTrue, v1alpha1.ModelReasonReady)
	appliedGeneration := meta.FindStatusCondition(applied.Status.Conditions, string(v1alpha1.ModelConditionReady)).ObservedGeneration
	appliedIdentifiers := slices.Clone(applied.Status.Identifiers)

	removeArtifact(t, c, artifact.Name, testNamespace)

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, model.Name, testNamespace, updated)
	updated.Spec.Replicas = 3
	updated.Spec.Aliases = []string{"new-alias"}
	// The fake client does not advance generations like the API server.
	updated.Generation = appliedGeneration + 1
	if err := c.Update(context.Background(), updated); err != nil {
		t.Fatalf("update model: %v", err)
	}

	reconcileModelOnce(t, r, "tiny-llm")

	engine := &appsv1.Deployment{}
	testutil.MustGet(t, c, model.EngineName(), testNamespace, engine)
	if engine.Spec.Replicas == nil || *engine.Spec.Replicas != 1 {
		t.Errorf("engine replicas after unapplied scale-up: %v, want 1", engine.Spec.Replicas)
	}

	updated = &v1alpha1.Model{}
	testutil.MustGet(t, c, model.Name, testNamespace, updated)
	cond := meta.FindStatusCondition(updated.Status.Conditions, string(v1alpha1.ModelConditionReady))
	if cond == nil {
		t.Fatal("Ready condition not found")
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != string(v1alpha1.ModelReasonReady) {
		t.Errorf("Ready condition: status=%s reason=%s, want status=%s reason=%s",
			cond.Status, cond.Reason, metav1.ConditionTrue, v1alpha1.ModelReasonReady)
	}
	if cond.ObservedGeneration != appliedGeneration {
		t.Errorf("Ready observedGeneration: %d, want last applied generation %d", cond.ObservedGeneration, appliedGeneration)
	}
	if updated.Generation <= cond.ObservedGeneration {
		t.Errorf("test did not advance the generation: model=%d condition=%d", updated.Generation, cond.ObservedGeneration)
	}
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotFound)
	if !slices.Equal(updated.Status.Identifiers, appliedIdentifiers) {
		t.Errorf("identifiers after unapplied alias change: %v, want %v", updated.Status.Identifiers, appliedIdentifiers)
	}
}

func TestReconcile_MissingArtifact_ScaleToZero(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")
	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	removeArtifact(t, c, artifact.Name, testNamespace)

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
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonNoReplicasDesired) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonNoReplicasDesired)
	}
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonNotFound)
}

func TestReconcile_TerminatingArtifact(t *testing.T) {
	model, artifact := testutil.Fixture("tiny-llm", testNamespace).Build()
	r, c := newTestReconciler(t, model, artifact)

	reconcileModelOnce(t, r, "tiny-llm")
	markDeploymentReady(t, c, model.EngineName())
	markDeploymentReady(t, c, model.EPPName())
	markInferencePoolReady(t, c, model.EngineName())
	markHTTPRouteReady(t, c, model.EngineName())
	reconcileModelOnce(t, r, "tiny-llm")

	markArtifactTerminating(t, c, artifact.Name, testNamespace)
	reconcileModelOnce(t, r, "tiny-llm")

	testutil.MustGet(t, c, "tiny-llm-engine", testNamespace, &appsv1.Deployment{})
	testutil.MustGet(t, c, "tiny-llm-epp", testNamespace, &appsv1.Deployment{})

	updated := &v1alpha1.Model{}
	testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionTrue, v1alpha1.ModelReasonReady)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonTerminating)
	if msg := conditionMessage(updated, v1alpha1.ModelConditionArtifactReady); strings.Contains(msg, "update spec.artifactRef") {
		t.Errorf("ArtifactReady message must not suggest changing immutable artifactRef, got: %q", msg)
	}

	markHTTPRouteStatus(t, c, model.EngineName(),
		metav1.Condition{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionFalse, Reason: string(gatewayv1.RouteReasonNotAllowedByListeners)},
	)
	reconcileModelOnce(t, r, model.Name)
	testutil.MustGet(t, c, model.Name, testNamespace, updated)
	assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonHTTPRouteRejected)
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonTerminating)
}

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
	if reason := readyConditionReason(updated); reason != string(v1alpha1.ModelReasonNoReplicasDesired) {
		t.Errorf("Ready reason:\ngot:  %q\nwant: %q", reason, v1alpha1.ModelReasonNoReplicasDesired)
	}
	assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionFalse, v1alpha1.ModelArtifactReasonTerminating)
}

func TestReconcile_ConditionErrors(t *testing.T) {
	cases := []struct {
		name               string
		wantArtifactStatus metav1.ConditionStatus
		wantArtifactReason v1alpha1.ModelArtifactReason
	}{
		{"artifact lookup", metav1.ConditionUnknown, v1alpha1.ModelArtifactReasonReconcileError},
		{"apply", metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady},
		{"serving observation", metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := testutil.NewModel("tiny-llm", testNamespace)
			model.Generation = 3
			r, c := newTestReconciler(t, model)
			reconcileModelOnce(t, r, model.Name)
			markDeploymentReady(t, c, model.EngineName())
			markDeploymentReady(t, c, model.EPPName())
			markInferencePoolReady(t, c, model.EngineName())
			markHTTPRouteReady(t, c, model.EngineName())
			reconcileModelOnce(t, r, model.Name)

			injectedErr := errors.New("API request failed")
			failingClient := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
				Get: func(ctx context.Context, underlying client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*v1alpha1.ModelArtifact); ok && tc.name == "artifact lookup" {
						return injectedErr
					}
					if _, ok := obj.(*appsv1.Deployment); ok && tc.name == "serving observation" {
						return injectedErr
					}
					return underlying.Get(ctx, key, obj, opts...)
				},
				Apply: func(ctx context.Context, underlying client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
					if tc.name == "apply" {
						return injectedErr
					}
					return underlying.Apply(ctx, obj, opts...)
				},
			})
			r.Client = failingClient
			r.APIReader = failingClient
			if _, err := r.Reconcile(context.Background(), ctrl.Request{Name: model.Name, Namespace: testNamespace}); !errors.Is(err, injectedErr) {
				t.Fatalf("Reconcile error: %v, want %v", err, injectedErr)
			}
			updated := testutil.MustGet(t, c, model.Name, testNamespace, &v1alpha1.Model{})
			assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionFalse, v1alpha1.ModelReasonReconcileError)
			assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, tc.wantArtifactStatus, tc.wantArtifactReason)

			r.Client = c
			r.APIReader = c
			reconcileModelOnce(t, r, model.Name)
			testutil.MustGet(t, c, model.Name, testNamespace, updated)
			assertModelCondition(t, updated, v1alpha1.ModelConditionReady, metav1.ConditionTrue, v1alpha1.ModelReasonReady)
			assertModelCondition(t, updated, v1alpha1.ModelConditionArtifactReady, metav1.ConditionTrue, v1alpha1.ModelArtifactReasonReady)
		})
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
