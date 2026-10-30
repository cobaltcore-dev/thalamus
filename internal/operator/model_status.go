// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// syncNativeStatus derives and updates the ready state of a model by checking:
// engine Deployment, EPP Deployment, InferencePool acceptance,
// and HTTPRoute acceptance.
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
	dep, err := getObject(ctx, r.Client, model.Namespace, name, &appsv1.Deployment{})
	if err != nil {
		return false, err
	}
	if dep == nil {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, notReadyReason, fmt.Sprintf("Deployment %s not found", name))
		return false, nil
	}

	if dep.Status.ObservedGeneration < dep.Generation {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, notReadyReason, fmt.Sprintf("Deployment %s status is stale", name))
		return false, nil
	}

	if deploymentReplicasReady(dep) {
		return true, nil
	}

	if failed, msg := deploymentFailed(dep); failed {
		setModelStatus(model, v1alpha1.ModelPhaseFailed, failedReason, fmt.Sprintf("Deployment %s %s", name, msg))
		return false, nil
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
	pool, err := getObject(ctx, r.Client, model.Namespace, model.EngineName(), &inferencev1.InferencePool{})
	if err != nil {
		return false, err
	}
	if pool == nil {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s not found", model.EngineName()))
		return false, nil
	}

	if len(pool.Status.Parents) == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s has no parent status yet", model.EngineName()))
		return false, nil
	}

	allReady := true
	var failedMessage string
	for _, parent := range pool.Status.Parents {
		pphase, pmessage := checkParentConditions(parent.Conditions, pool.Generation, inferencePoolParentPolicy)
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
	route, err := getObject(ctx, r.Client, model.Namespace, model.EngineName(), &gatewayv1.HTTPRoute{})
	if err != nil {
		return false, err
	}
	if route == nil {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s not found", model.EngineName()))
		return false, nil
	}

	// The informer cache may still hold the route from before this apply.
	desired := native.BuildHTTPRoute(model, r.GatewayName, r.ListenerName)
	if !httpRouteServesDesiredMatches(route, desired) {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted,
			fmt.Sprintf("HTTPRoute %s does not serve the current model names", model.EngineName()))
		return false, nil
	}

	if len(route.Status.Parents) == 0 {
		setModelStatus(model, v1alpha1.ModelPhaseCreating, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s has no parent status yet", model.EngineName()))
		return false, nil
	}

	allReady := true
	var failedMessage string
	partiallyInvalid := false
	for _, parent := range route.Status.Parents {
		pphase, pmessage := checkParentConditions(parent.Conditions, route.Generation, httpRouteParentPolicy)
		if pphase != v1alpha1.ModelPhaseReady {
			allReady = false
		}
		if pphase == v1alpha1.ModelPhaseFailed {
			failedMessage = pmessage
		}
		partiallyInvalidCond := meta.FindStatusCondition(parent.Conditions, string(gatewayv1.RouteConditionPartiallyInvalid))
		if partiallyInvalidCond != nil && partiallyInvalidCond.Status == metav1.ConditionTrue &&
			partiallyInvalidCond.ObservedGeneration >= route.Generation {
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

// httpRouteServesDesiredMatches reports whether route serves the desired
// matches. Other fields are skipped because the API server defaults them.
func httpRouteServesDesiredMatches(route, desired *gatewayv1.HTTPRoute) bool {
	if len(route.Spec.Rules) != len(desired.Spec.Rules) {
		return false
	}
	for i := range desired.Spec.Rules {
		if !apiequality.Semantic.DeepEqual(route.Spec.Rules[i].Matches, desired.Spec.Rules[i].Matches) {
			return false
		}
	}
	return true
}

// parentStatusPolicy describes how a parent's conditions are interpreted.
type parentStatusPolicy struct {
	acceptedType string
	resolvedType string
	// transientReasons are reasons that mean "not ready yet" rather than
	// "failed". Reason values are specific to their condition type.
	transientReasons map[string]struct{}
}

var (
	inferencePoolParentPolicy = parentStatusPolicy{
		acceptedType: string(inferencev1.InferencePoolConditionAccepted),
		resolvedType: string(inferencev1.InferencePoolConditionResolvedRefs),
		transientReasons: map[string]struct{}{
			string(inferencev1.InferencePoolReasonNotRequested):         {},
			string(inferencev1.InferencePoolReasonHTTPRouteNotAccepted): {},
		},
	}
	httpRouteParentPolicy = parentStatusPolicy{
		acceptedType: string(gatewayv1.RouteConditionAccepted),
		resolvedType: string(gatewayv1.RouteConditionResolvedRefs),
		transientReasons: map[string]struct{}{
			string(gatewayv1.RouteReasonPending):         {},
			string(gatewayv1.RouteReasonBackendNotFound): {},
		},
	}
)

// checkParentConditions returns the model phase for one parent. Stale conditions
// are pending. It is Ready only when the accepted condition is True and the
// resolved condition is absent or True; a False condition is Failed unless its
// reason is transient.
func checkParentConditions(
	conditions []metav1.Condition,
	generation int64,
	policy parentStatusPolicy,
) (phase v1alpha1.ModelPhase, message string) {

	acceptedCond := meta.FindStatusCondition(conditions, policy.acceptedType)
	resolvedCond := meta.FindStatusCondition(conditions, policy.resolvedType)

	// Stale conditions may still report the previous spec as accepted.
	for _, cond := range []*metav1.Condition{acceptedCond, resolvedCond} {
		if cond != nil && cond.ObservedGeneration < generation {
			return v1alpha1.ModelPhaseCreating, cond.Type + " condition is stale"
		}
	}

	if acceptedCond != nil && acceptedCond.Status == metav1.ConditionTrue &&
		(resolvedCond == nil || resolvedCond.Status == metav1.ConditionTrue) {
		return v1alpha1.ModelPhaseReady, ""
	}

	for _, cond := range []*metav1.Condition{resolvedCond, acceptedCond} {
		if cond != nil && cond.Status == metav1.ConditionFalse {
			msg := fmt.Sprintf("%s: %s", cond.Reason, cond.Message)
			if _, transient := policy.transientReasons[cond.Reason]; transient {
				return v1alpha1.ModelPhaseCreating, msg
			}
			return v1alpha1.ModelPhaseFailed, msg
		}
	}

	return v1alpha1.ModelPhaseCreating, "parent status is pending"
}

// deploymentFailed reports whether a Deployment is in a terminal failure state
// and returns a human-readable message.
func deploymentFailed(dep *appsv1.Deployment) (failed bool, message string) {
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return false, ""
		}
	}

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
