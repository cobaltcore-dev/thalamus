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
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

// syncNativeStatus derives and updates the ready state of a model by checking:
// engine Deployment, EPP Deployment, InferencePool acceptance,
// and HTTPRoute acceptance.
func (r *ModelReconciler) syncNativeStatus(ctx context.Context, model *v1alpha1.Model) error {
	// Engine Deployment
	phase, reason, message, err := r.deploymentReady(ctx, model, model.EngineName(), "engine", v1alpha1.ModelReasonEngineNotReady, v1alpha1.ModelReasonEngineDeploymentFailed)
	if err != nil {
		return err
	}
	if phase != v1alpha1.ModelPhaseReady {
		setModelStatus(model, phase, reason, message)
		return nil
	}

	// EPP Deployment
	phase, reason, message, err = r.deploymentReady(ctx, model, model.EPPName(), "epp", v1alpha1.ModelReasonEPPNotReady, v1alpha1.ModelReasonEPPDeploymentFailed)
	if err != nil {
		return err
	}
	if phase != v1alpha1.ModelPhaseReady {
		setModelStatus(model, phase, reason, message)
		return nil
	}

	// InferencePool acceptance
	phase, reason, message, err = r.inferencePoolReady(ctx, model)
	if err != nil {
		return err
	}
	if phase != v1alpha1.ModelPhaseReady {
		setModelStatus(model, phase, reason, message)
		return nil
	}

	// HTTPRoute acceptance
	phase, reason, message, err = r.httpRouteReady(ctx, model)
	if err != nil {
		return err
	}
	if phase != v1alpha1.ModelPhaseReady {
		setModelStatus(model, phase, reason, message)
		return nil
	}

	setModelStatus(model, v1alpha1.ModelPhaseReady, v1alpha1.ModelReasonReady, "model is reachable")
	return nil
}

// setModelStatus updates the Model's Phase and Ready condition in-place.
// The caller is responsible for patching the status.
func setModelStatus(model *v1alpha1.Model, phase v1alpha1.ModelPhase, reason, message string) {
	model.Status.Phase = phase
	status := metav1.ConditionFalse
	if phase == v1alpha1.ModelPhaseReady {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModelConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: model.Generation,
	})
}

// deploymentReady reports the model phase implied by a child Deployment.
func (r *ModelReconciler) deploymentReady(ctx context.Context, model *v1alpha1.Model, name, resource, notReadyReason, failedReason string) (phase v1alpha1.ModelPhase, reason, message string, err error) {
	dep := &appsv1.Deployment{}
	if err = r.Get(ctx, types.NamespacedName{Name: name, Namespace: model.Namespace}, dep); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return v1alpha1.ModelPhaseCreating, notReadyReason, resource + " deployment not found", nil
		}
		return
	}

	if failed, msg := deploymentFailed(dep); failed {
		return v1alpha1.ModelPhaseFailed, failedReason, msg, nil
	}

	if deploymentReplicasReady(dep) {
		return v1alpha1.ModelPhaseReady, "", "", nil
	}
	return v1alpha1.ModelPhaseCreating, notReadyReason, resource + " deployment has insufficient ready replicas", nil
}

// deploymentReplicasReady reports whether the Deployment's observed ready
// replicas have reached the desired count for the current generation.
func deploymentReplicasReady(dep *appsv1.Deployment) bool {
	if dep.Status.ObservedGeneration < dep.Generation {
		return false
	}
	var desired int32 = 1
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	return dep.Status.ReadyReplicas >= desired
}

// inferencePoolReady reports the model phase implied by the InferencePool status.
func (r *ModelReconciler) inferencePoolReady(ctx context.Context, model *v1alpha1.Model) (phase v1alpha1.ModelPhase, reason, message string, err error) {
	pool := &inferencev1.InferencePool{}
	if err = r.Get(ctx, types.NamespacedName{Name: model.EngineName(), Namespace: model.Namespace}, pool); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, "inference pool not found", nil
		}
		return
	}

	if len(pool.Status.Parents) == 0 {
		return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, "inference pool has no parent status yet", nil
	}

	allReady := true
	var failedMessage string
	for _, parent := range pool.Status.Parents {
		pphase, pmessage := checkParentConditions(
			parent.Conditions,
			string(inferencev1.InferencePoolConditionAccepted),
			string(inferencev1.InferencePoolConditionResolvedRefs),
			func(condReason string) bool {
				switch condReason {
				case string(inferencev1.InferencePoolReasonNotRequested),
					string(inferencev1.InferencePoolReasonHTTPRouteNotAccepted):
					return true
				}
				return false
			},
		)
		if pphase != v1alpha1.ModelPhaseReady {
			allReady = false
		}
		if pphase == v1alpha1.ModelPhaseFailed {
			failedMessage = pmessage
		}
	}

	if failedMessage != "" {
		return v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonInferencePoolRejected, failedMessage, nil
	}
	if allReady {
		return v1alpha1.ModelPhaseReady, "", "", nil
	}
	return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, "inference pool not accepted by gateway", nil
}

// httpRouteReady reports the model phase implied by the HTTPRoute status.
func (r *ModelReconciler) httpRouteReady(ctx context.Context, model *v1alpha1.Model) (phase v1alpha1.ModelPhase, reason, message string, err error) {
	route := &gatewayv1.HTTPRoute{}
	if err = r.Get(ctx, types.NamespacedName{Name: model.EngineName(), Namespace: model.Namespace}, route); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, "http route not found", nil
		}
		return
	}

	if len(route.Status.Parents) == 0 {
		return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, "http route has no parent status yet", nil
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
		return v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonHTTPRouteRejected, failedMessage, nil
	}
	if partiallyInvalid {
		return v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonHTTPRoutePartiallyInvalid, "http route is partially invalid", nil
	}
	if allReady {
		return v1alpha1.ModelPhaseReady, "", "", nil
	}
	return v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, "http route not accepted by gateway", nil
}

// checkParentConditions evaluates the Accepted and ResolvedRefs conditions for a
// single parent. It returns ModelPhaseReady only when the Accepted condition is
// present and True and the ResolvedRefs condition is either absent or True.
func checkParentConditions(
	conditions []metav1.Condition,
	acceptedType, resolvedType string,
	isTransientAcceptedReason func(string) bool,
) (phase v1alpha1.ModelPhase, message string) {

	acceptedCond := meta.FindStatusCondition(conditions, acceptedType)
	resolvedCond := meta.FindStatusCondition(conditions, resolvedType)

	if (acceptedCond != nil && acceptedCond.Status == metav1.ConditionTrue) &&
		(resolvedCond == nil || resolvedCond.Status == metav1.ConditionTrue) {
		return v1alpha1.ModelPhaseReady, ""
	}

	if resolvedCond != nil && resolvedCond.Status == metav1.ConditionFalse {
		return v1alpha1.ModelPhaseFailed, fmt.Sprintf("%s: %s", resolvedCond.Reason, resolvedCond.Message)
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
			return true, "deployment replica failure: " + c.Message
		}
		if c.Type == appsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
			return true, "deployment progress deadline exceeded: " + c.Message
		}
	}
	return false, ""
}
