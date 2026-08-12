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
	resources := []struct {
		name          string
		syncCondition func(context.Context, *v1alpha1.Model) (bool, error)
	}{
		{"engine", r.syncEngineCondition},
		{"epp", r.syncEPPCondition},
		{"inference pool", r.syncInferencePoolCondition},
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

// syncInferencePoolCondition checks the InferencePool status and updates the model status.
func (r *ModelReconciler) syncInferencePoolCondition(ctx context.Context, model *v1alpha1.Model) (bool, error) {
	pool := &inferencev1.InferencePool{}
	if err := r.Get(ctx, types.NamespacedName{Name: model.EngineName(), Namespace: model.Namespace}, pool); err != nil {
		if client.IgnoreNotFound(err) == nil {
			setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s not found", model.EngineName()))
			return false, nil
		}
		return false, err
	}

	if len(pool.Status.Parents) == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s has no parent status yet", model.EngineName()))
		return false, nil
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
			nil,
		)
		if pphase != v1alpha1.ModelPhaseReady {
			allReady = false
		}
		if pphase == v1alpha1.ModelPhaseFailed {
			failedMessage = pmessage
		}
	}

	if failedMessage != "" {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, v1alpha1.ModelReasonInferencePoolRejected, failedMessage)
		return false, nil
	}
	if allReady {
		return true, nil
	}
	setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s not accepted by gateway", model.EngineName()))
	return false, nil
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
