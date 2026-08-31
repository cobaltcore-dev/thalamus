// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// syncNativeStatus derives and updates the ready state of a model by checking:
// engine Deployment, EPP Deployment, ext-proc policy attachment, and
// HTTPRoute acceptance.
func (r *ModelReconciler) syncNativeStatus(ctx context.Context, model *v1alpha1.Model) error {
	if model.Spec.Replicas == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseInactive, v1alpha1.ModelReasonNoReplicasDesired, "model has no desired replicas")
		return nil
	}

	resources := []struct {
		name          string
		syncCondition func(context.Context, *v1alpha1.Model) (bool, error)
	}{
		{"engine", r.syncEngineCondition},
		{"epp", r.syncEPPCondition},
		{"epp ext-proc policy", r.syncEPPExtProcPolicyCondition},
		{"http route", r.syncHTTPRouteCondition},
	}

	for _, resource := range resources {
		ready, err := resource.syncCondition(ctx, model)
		if err != nil {
			return fmt.Errorf("sync %s condition failed: %w", resource.name, err)
		}
		if !ready {
			return nil
		}
	}

	setModelStatus(model, v1alpha1.ModelPhaseReady, v1alpha1.ModelReasonReady, "Model is reachable")
	return nil
}

// setModelStatus updates the Model's Phase and Ready condition in-place.
// The caller is responsible for patching the status.
func setModelStatus(model *v1alpha1.Model, phase v1alpha1.ModelPhase, reason v1alpha1.ModelReason, message string) {
	model.Status.Phase = phase
	status := metav1.ConditionFalse
	if phase == v1alpha1.ModelPhaseReady {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModelConditionReady,
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: model.Generation,
	})
}

// syncEngineCondition checks the engine Deployment and updates the model status.
func (r *ModelReconciler) syncEngineCondition(ctx context.Context, model *v1alpha1.Model) (bool, error) {
	return r.syncDeploymentCondition(ctx, model, model.EngineName(), v1alpha1.ModelReasonEngineNotReady, v1alpha1.ModelReasonEngineDeploymentFailed)
}

// syncEPPCondition checks the EPP Deployment and updates the model status.
func (r *ModelReconciler) syncEPPCondition(ctx context.Context, model *v1alpha1.Model) (bool, error) {
	return r.syncDeploymentCondition(ctx, model, model.EPPName(), v1alpha1.ModelReasonEPPNotReady, v1alpha1.ModelReasonEPPDeploymentFailed)
}

// syncEPPExtProcPolicyCondition checks the model's ext-proc AgentgatewayPolicy
// and updates the model status. The policy is ready once agentgateway has
// accepted it (Accepted=True) and attached it to the target HTTPRoute
// (Attached=True).
func (r *ModelReconciler) syncEPPExtProcPolicyCondition(ctx context.Context, model *v1alpha1.Model) (bool, error) {
	policyName := model.ExtProcPolicyName()
	policy := &agentgatewayv1alpha1.AgentgatewayPolicy{}
	if err := r.Get(ctx, types.NamespacedName{Name: policyName, Namespace: model.Namespace}, policy); err != nil {
		if client.IgnoreNotFound(err) == nil {
			setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonEPPExtProcNotAccepted, fmt.Sprintf("ext-proc policy %s not found", policyName))
			return false, nil
		}
		return false, err
	}

	if len(policy.Status.Ancestors) == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonEPPExtProcNotAccepted, fmt.Sprintf("ext-proc policy %s has no status yet", policyName))
		return false, nil
	}

	allReady := true
	var failedMessage string
	for _, ancestor := range policy.Status.Ancestors {
		// Accepted is the primary validity signal: the agentgateway controller
		// sets Attached=False/Pending both for rejected policies and for
		// policies whose target is not resolvable yet, so the Accepted
		// condition decides rejection.
		acceptedCond := meta.FindStatusCondition(ancestor.Conditions, agentgatewayv1alpha1.PolicyConditionAccepted)
		attachedCond := meta.FindStatusCondition(ancestor.Conditions, agentgatewayv1alpha1.PolicyConditionAttached)
		if acceptedCond != nil && acceptedCond.Status == metav1.ConditionFalse {
			failedMessage = fmt.Sprintf("%s: %s", acceptedCond.Reason, acceptedCond.Message)
			continue
		}
		if acceptedCond != nil && acceptedCond.Status == metav1.ConditionTrue &&
			attachedCond != nil && attachedCond.Status == metav1.ConditionTrue {
			continue
		}
		allReady = false
	}

	if failedMessage != "" {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonEPPExtProcRejected, failedMessage)
		return false, nil
	}
	if allReady {
		return true, nil
	}
	setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonEPPExtProcNotAccepted, fmt.Sprintf("ext-proc policy %s not accepted by gateway", policyName))
	return false, nil
}

// syncDeploymentCondition checks a child Deployment and updates the model status.
func (r *ModelReconciler) syncDeploymentCondition(ctx context.Context, model *v1alpha1.Model, name string, notReadyReason, failedReason v1alpha1.ModelReason) (bool, error) {
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: model.Namespace}, dep); err != nil {
		if client.IgnoreNotFound(err) == nil {
			setModelStatus(model, v1alpha1.ModelPhaseCreating, notReadyReason, fmt.Sprintf("Deployment %s not found", name))
			return false, nil
		}
		return false, err
	}

	if dep.Status.ObservedGeneration < dep.Generation {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, notReadyReason, fmt.Sprintf("Deployment %s status is stale", name))
		return false, nil
	}

	if failed, msg := deploymentFailed(dep); failed {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, failedReason, fmt.Sprintf("Deployment %s %s", name, msg))
		return false, nil
	}

	if deploymentReplicasReady(dep) {
		return true, nil
	}

	setModelStatus(model, v1alpha1.ModelPhaseCreating, notReadyReason, fmt.Sprintf("Deployment %s has insufficient ready replicas", name))
	return false, nil
}

// deploymentReplicasReady reports whether the Deployment's ready replicas
// have reached the desired count. A count of zero is not considered ready.
func deploymentReplicasReady(dep *appsv1.Deployment) bool {
	var desired int32 = 1
	if dep.Spec.Replicas != nil && *dep.Spec.Replicas > desired {
		desired = *dep.Spec.Replicas
	}
	return dep.Status.ReadyReplicas >= desired
}

// syncHTTPRouteCondition checks the HTTPRoute status and updates the model status.
func (r *ModelReconciler) syncHTTPRouteCondition(ctx context.Context, model *v1alpha1.Model) (bool, error) {
	route := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, types.NamespacedName{Name: model.EngineName(), Namespace: model.Namespace}, route); err != nil {
		if client.IgnoreNotFound(err) == nil {
			setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s not found", model.EngineName()))
			return false, nil
		}
		return false, err
	}

	if len(route.Status.Parents) == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s has no parent status yet", model.EngineName()))
		return false, nil
	}

	allReady := true
	var failedMessage string
	partiallyInvalid := false
	for _, parent := range route.Status.Parents {
		pphase, pmessage := checkParentConditions(
			parent.Conditions,
			string(gatewayv1.RouteConditionAccepted),
			string(gatewayv1.RouteConditionResolvedRefs),
			func(condReason string) bool {
				return condReason == string(gatewayv1.RouteReasonPending)
			},
			func(condReason string) bool {
				return condReason == string(gatewayv1.RouteReasonBackendNotFound)
			},
		)
		if pphase != v1alpha1.ModelPhaseReady {
			allReady = false
		}
		if pphase == v1alpha1.ModelPhaseFailed {
			failedMessage = pmessage
		}
		if cond := meta.FindStatusCondition(parent.Conditions, string(gatewayv1.RouteConditionPartiallyInvalid)); cond != nil && cond.Status == metav1.ConditionTrue {
			partiallyInvalid = true
		}
	}

	if failedMessage != "" {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonHTTPRouteRejected, failedMessage)
		return false, nil
	}
	if partiallyInvalid {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonHTTPRoutePartiallyInvalid, fmt.Sprintf("HTTPRoute %s is partially invalid", model.EngineName()))
		return false, nil
	}
	if allReady {
		return true, nil
	}
	setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s not accepted by gateway", model.EngineName()))
	return false, nil
}

// checkParentConditions evaluates the Accepted and ResolvedRefs conditions for a
// single parent. It returns ModelPhaseReady only when the Accepted condition is
// present and True and the ResolvedRefs condition is either absent or True.
// ResolvedRefs=False is treated as ModelPhaseFailed unless the reason is known
// to be transient. Accepted=False is treated as ModelPhaseCreating unless the
// reason is known to be transient; otherwise it is ModelPhaseFailed.
func checkParentConditions(
	conditions []metav1.Condition,
	acceptedType, resolvedType string,
	isTransientAcceptedReason func(string) bool,
	isTransientResolvedReason func(string) bool,
) (phase v1alpha1.ModelPhase, message string) {

	acceptedCond := meta.FindStatusCondition(conditions, acceptedType)
	resolvedCond := meta.FindStatusCondition(conditions, resolvedType)

	if (acceptedCond != nil && acceptedCond.Status == metav1.ConditionTrue) &&
		(resolvedCond == nil || resolvedCond.Status == metav1.ConditionTrue) {
		return v1alpha1.ModelPhaseReady, ""
	}

	if resolvedCond != nil && resolvedCond.Status == metav1.ConditionFalse {
		msg := fmt.Sprintf("%s: %s", resolvedCond.Reason, resolvedCond.Message)
		if isTransientResolvedReason != nil && isTransientResolvedReason(resolvedCond.Reason) {
			return v1alpha1.ModelPhaseCreating, msg
		}
		return v1alpha1.ModelPhaseFailed, msg
	}

	if acceptedCond != nil && acceptedCond.Status == metav1.ConditionFalse {
		msg := fmt.Sprintf("%s: %s", acceptedCond.Reason, acceptedCond.Message)
		if isTransientAcceptedReason(acceptedCond.Reason) {
			return v1alpha1.ModelPhaseCreating, msg
		}
		return v1alpha1.ModelPhaseFailed, msg
	}

	return v1alpha1.ModelPhaseCreating, "parent status is pending"
}

// deploymentFailed reports whether a Deployment is in a terminal failure state
// and returns a human-readable message.
func deploymentFailed(dep *appsv1.Deployment) (failed bool, message string) {
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Status == corev1.ConditionTrue {
			return true, "replica failure: " + c.Message
		}
		if c.Type == appsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
			return true, "progress deadline exceeded: " + c.Message
		}
	}
	return false, ""
}
