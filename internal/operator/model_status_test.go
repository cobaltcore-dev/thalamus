// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestReconcile_ModelStatus(t *testing.T) {
	cases := []struct {
		name       string
		prepare    func(*testing.T, client.Client, *v1alpha1.Model)
		wantPhase  v1alpha1.ModelPhase
		wantReason v1alpha1.ModelReason
	}{
		{
			name: "ready when all ready",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteReady(t, c, m.EngineName())
			},
			wantPhase:  v1alpha1.ModelPhaseReady,
			wantReason: v1alpha1.ModelReasonReady,
		},
		{
			name: "creating when engine deployment not ready",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteReady(t, c, m.EngineName())
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonEngineNotReady,
		},
		{
			name: "creating when epp deployment not ready",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markHTTPRouteReady(t, c, m.EngineName())
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonEPPNotReady,
		},
		{
			name: "creating when http route not ready",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonHTTPRouteNotAccepted,
		},
		{
			name: "creating when engine deployment observed generation is stale",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				dep := &appsv1.Deployment{}
				testutil.MustGet(t, c, m.EngineName(), testNamespace, dep)
				dep.Status.ObservedGeneration = dep.Generation - 1
				if err := c.Status().Update(context.Background(), dep); err != nil {
					t.Fatalf("update deployment status: %v", err)
				}
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteReady(t, c, m.EngineName())
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonEngineNotReady,
		},
		{
			name: "creating when http route accepted is pending",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteStatus(t, c, m.EngineName(), metav1.Condition{
					Type:    string(gatewayv1.RouteConditionAccepted),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.RouteReasonPending),
					Message: "pending",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonHTTPRouteNotAccepted,
		},
		{
			name: "failed when engine deployment replica failure",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentFailed(t, c, m.EngineName(), appsv1.DeploymentCondition{
					Type:    appsv1.DeploymentReplicaFailure,
					Status:  corev1.ConditionTrue,
					Reason:  "FailedCreate",
					Message: "replica creation failed",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonEngineDeploymentFailed,
		},
		{
			name: "failed when engine deployment progress deadline exceeded",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentFailed(t, c, m.EngineName(), appsv1.DeploymentCondition{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionFalse,
					Reason:  "ProgressDeadlineExceeded",
					Message: "progress deadline exceeded",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonEngineDeploymentFailed,
		},
		{
			name: "failed when epp deployment replica failure",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentFailed(t, c, m.EPPName(), appsv1.DeploymentCondition{
					Type:    appsv1.DeploymentReplicaFailure,
					Status:  corev1.ConditionTrue,
					Reason:  "FailedCreate",
					Message: "replica creation failed",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonEPPDeploymentFailed,
		},
		{
			name: "failed when epp deployment progress deadline exceeded",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentFailed(t, c, m.EPPName(), appsv1.DeploymentCondition{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionFalse,
					Reason:  "ProgressDeadlineExceeded",
					Message: "progress deadline exceeded",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonEPPDeploymentFailed,
		},
		{
			name: "failed when http route rejected",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteStatus(t, c, m.EngineName(), metav1.Condition{
					Type:    string(gatewayv1.RouteConditionAccepted),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.RouteReasonNotAllowedByListeners),
					Message: "not allowed",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonHTTPRouteRejected,
		},
		{
			name: "failed when http route has incompatible filters",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteStatus(t, c, m.EngineName(), metav1.Condition{
					Type:    string(gatewayv1.RouteConditionAccepted),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.RouteReasonIncompatibleFilters),
					Message: "incompatible filters",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonHTTPRouteRejected,
		},
		{
			name: "failed when http route partially invalid",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteStatus(t, c, m.EngineName(),
					metav1.Condition{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonAccepted)},
					metav1.Condition{Type: string(gatewayv1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonResolvedRefs)},
					metav1.Condition{Type: string(gatewayv1.RouteConditionPartiallyInvalid), Status: metav1.ConditionTrue, Reason: "PartiallyInvalid", Message: "partial error"},
				)
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonHTTPRoutePartiallyInvalid,
		},
		{
			name: "creating when http route backend not found",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteStatus(t, c, m.EngineName(), metav1.Condition{
					Type:    string(gatewayv1.RouteConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.RouteReasonBackendNotFound),
					Message: "backend not found",
				})
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonHTTPRouteNotAccepted,
		},
		{
			name: "failed when http route has one ready and one failed parent",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteParents(t, c, m.EngineName(),
					gatewayv1.RouteParentStatus{
						ParentRef:      gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")},
						ControllerName: "test-controller",
						Conditions: []metav1.Condition{
							{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonAccepted)},
							{Type: string(gatewayv1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonResolvedRefs)},
						},
					},
					gatewayv1.RouteParentStatus{
						ParentRef:      gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw2")},
						ControllerName: "test-controller",
						Conditions: []metav1.Condition{
							{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionFalse, Reason: string(gatewayv1.RouteReasonNotAllowedByListeners), Message: "not allowed"},
						},
					},
				)
			},
			wantPhase:  v1alpha1.ModelPhaseFailed,
			wantReason: v1alpha1.ModelReasonHTTPRouteRejected,
		},
		{
			name: "creating when http route has one ready and one pending parent",
			prepare: func(t *testing.T, c client.Client, m *v1alpha1.Model) {
				markDeploymentReady(t, c, m.EngineName())
				markDeploymentReady(t, c, m.EPPName())
				markHTTPRouteParents(t, c, m.EngineName(),
					gatewayv1.RouteParentStatus{
						ParentRef:      gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")},
						ControllerName: "test-controller",
						Conditions: []metav1.Condition{
							{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonAccepted)},
							{Type: string(gatewayv1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonResolvedRefs)},
						},
					},
					gatewayv1.RouteParentStatus{
						ParentRef:      gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw2")},
						ControllerName: "test-controller",
						Conditions: []metav1.Condition{
							{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionFalse, Reason: string(gatewayv1.RouteReasonPending), Message: "pending"},
						},
					},
				)
			},
			wantPhase:  v1alpha1.ModelPhaseCreating,
			wantReason: v1alpha1.ModelReasonHTTPRouteNotAccepted,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := testutil.NewModel("tiny-llm", testNamespace)
			r, c := newTestReconciler(t, model)

			reconcileModelOnce(t, r, "tiny-llm")
			tc.prepare(t, c, model)
			reconcileModelOnce(t, r, "tiny-llm")

			updated := &v1alpha1.Model{}
			testutil.MustGet(t, c, "tiny-llm", testNamespace, updated)

			if updated.Status.Phase != tc.wantPhase {
				t.Errorf("Phase:\ngot:  %q\nwant: %q", updated.Status.Phase, tc.wantPhase)
			}
			wantStatus := metav1.ConditionFalse
			if tc.wantPhase == v1alpha1.ModelPhaseReady {
				wantStatus = metav1.ConditionTrue
			}
			cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ModelConditionReady)
			if cond == nil {
				t.Fatalf("Ready condition not found on model")
			}
			if cond.Status != wantStatus {
				t.Errorf("Ready condition status:\ngot:  %q\nwant: %q", cond.Status, wantStatus)
			}
			if cond.Reason != string(tc.wantReason) {
				t.Errorf("Ready condition reason:\ngot:  %q\nwant: %q", cond.Reason, tc.wantReason)
			}
		})
	}
}

func markDeploymentReady(t *testing.T, c client.Client, name string) {
	t.Helper()
	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, name, testNamespace, dep)
	dep.Status.ReadyReplicas = 1
	dep.Status.ObservedGeneration = dep.Generation
	if err := c.Status().Update(context.Background(), dep); err != nil {
		t.Fatalf("update deployment status: %v", err)
	}
}

func markDeploymentFailed(t *testing.T, c client.Client, name string, cond appsv1.DeploymentCondition) {
	t.Helper()
	dep := &appsv1.Deployment{}
	testutil.MustGet(t, c, name, testNamespace, dep)
	dep.Status.Conditions = append(dep.Status.Conditions, cond)
	dep.Status.ObservedGeneration = dep.Generation
	if err := c.Status().Update(context.Background(), dep); err != nil {
		t.Fatalf("update deployment status: %v", err)
	}
}

func markHTTPRouteStatus(t *testing.T, c client.Client, name string, conds ...metav1.Condition) {
	t.Helper()
	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, name, testNamespace, route)
	route.Status.Parents = []gatewayv1.RouteParentStatus{{
		ParentRef:      gatewayv1.ParentReference{Name: gatewayv1.ObjectName("gw")},
		ControllerName: "test-controller",
		Conditions:     conds,
	}}
	if err := c.Status().Update(context.Background(), route); err != nil {
		t.Fatalf("update http route status: %v", err)
	}
}

func markHTTPRouteReady(t *testing.T, c client.Client, name string) {
	t.Helper()
	markHTTPRouteStatus(t, c, name,
		metav1.Condition{Type: string(gatewayv1.RouteConditionAccepted), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonAccepted)},
		metav1.Condition{Type: string(gatewayv1.RouteConditionResolvedRefs), Status: metav1.ConditionTrue, Reason: string(gatewayv1.RouteReasonResolvedRefs)},
	)
}

func markHTTPRouteParents(t *testing.T, c client.Client, name string, parents ...gatewayv1.RouteParentStatus) {
	t.Helper()
	route := &gatewayv1.HTTPRoute{}
	testutil.MustGet(t, c, name, testNamespace, route)
	route.Status.Parents = parents
	if err := c.Status().Update(context.Background(), route); err != nil {
		t.Fatalf("update http route status: %v", err)
	}
}

func TestDeploymentReplicasReady(t *testing.T) {
	cases := []struct {
		name     string
		replicas *int32
		ready    int32
		want     bool
	}{
		{"ready with default one replica", nil, 1, true},
		{"ready when all three replicas ready", new(int32(3)), 3, true},
		{"not ready when only one of three ready", new(int32(3)), 1, false},
		{"not ready when zero ready", new(int32(1)), 0, false},
		{"not ready when zero replicas requested", new(int32(0)), 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dep := &appsv1.Deployment{
				Spec:   appsv1.DeploymentSpec{Replicas: tc.replicas},
				Status: appsv1.DeploymentStatus{ReadyReplicas: tc.ready},
			}
			if got := deploymentReplicasReady(dep); got != tc.want {
				t.Errorf("deploymentReplicasReady() = %v, want %v", got, tc.want)
			}
		})
	}
}
