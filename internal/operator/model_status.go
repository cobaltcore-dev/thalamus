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

// modelStatus is the observed state of one resource in the serving stack.
type modelStatus struct {
	phase   v1alpha1.ModelPhase
	reason  v1alpha1.ModelReason
	message string
}

// setModelStatus updates the Model's Phase and Ready condition in-place.
// The caller is responsible for patching the status.
func setModelStatus(model *v1alpha1.Model, status modelStatus) {
	model.Status.Phase = status.phase
	conditionStatus := metav1.ConditionFalse
	if status.phase == v1alpha1.ModelPhaseReady {
		conditionStatus = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModelConditionReady,
		Status:             conditionStatus,
		Reason:             string(status.reason),
		Message:            status.message,
		ObservedGeneration: model.Generation,
	})
}

// nativeObservation holds the current state of the native serving stack.
type nativeObservation struct {
	engine *appsv1.Deployment
	router *appsv1.Deployment
	pool   *inferencev1.InferencePool
	route  *gatewayv1.HTTPRoute
}

func (o nativeObservation) empty() bool {
	return o.engine == nil && o.router == nil && o.pool == nil && o.route == nil
}

// syncNativeStatus derives and updates the ready state of a model from the
// observed engine, EPP, InferencePool, and HTTPRoute.
func (r *ModelReconciler) syncNativeStatus(ctx context.Context, model *v1alpha1.Model) error {
	obs, err := r.observeNative(ctx, model)
	if err != nil {
		return err
	}
	if model.Spec.Replicas == 0 && obs.empty() {
		setModelStatus(model, modelStatus{
			phase:   v1alpha1.ModelPhaseInactive,
			reason:  v1alpha1.ModelReasonNoReplicasDesired,
			message: "model has no desired replicas",
		})
		return nil
	}
	setModelStatus(model, nativeStatus(model, obs, r.GatewayName, r.ListenerName))
	return nil
}

func (r *ModelReconciler) observeNative(ctx context.Context, model *v1alpha1.Model) (nativeObservation, error) {
	var obs nativeObservation
	var err error

	if obs.engine, err = getObject(ctx, r.Client, model.Namespace, model.EngineName(), &appsv1.Deployment{}); err != nil {
		return obs, err
	}
	if obs.router, err = getObject(ctx, r.Client, model.Namespace, model.EPPName(), &appsv1.Deployment{}); err != nil {
		return obs, err
	}
	if obs.pool, err = getObject(ctx, r.Client, model.Namespace, model.EngineName(), &inferencev1.InferencePool{}); err != nil {
		return obs, err
	}
	if obs.route, err = getObject(ctx, r.Client, model.Namespace, model.EngineName(), &gatewayv1.HTTPRoute{}); err != nil {
		return obs, err
	}
	return obs, nil
}

func nativeStatus(model *v1alpha1.Model, obs nativeObservation, gatewayName, listenerName string) modelStatus {
	if status := deploymentStatus(obs.engine, model.EngineName(), v1alpha1.ModelReasonEngineNotReady, v1alpha1.ModelReasonEngineDeploymentFailed); status != nil {
		return *status
	}
	if obs.engine.Status.ReadyReplicas < model.Spec.Replicas {
		return modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonEngineNotReady, message: fmt.Sprintf("Deployment %s has insufficient ready replicas", model.EngineName())}
	}
	if status := deploymentStatus(obs.router, model.EPPName(), v1alpha1.ModelReasonEPPNotReady, v1alpha1.ModelReasonEPPDeploymentFailed); status != nil {
		return *status
	}
	if status := inferencePoolStatus(obs.pool, model.EngineName()); status != nil {
		return *status
	}
	if status := httpRouteStatus(model, obs.route, gatewayName, listenerName); status != nil {
		return *status
	}
	return modelStatus{phase: v1alpha1.ModelPhaseReady, reason: v1alpha1.ModelReasonReady, message: "Model is reachable"}
}

func deploymentStatus(dep *appsv1.Deployment, name string, notReadyReason, failedReason v1alpha1.ModelReason) *modelStatus {
	if dep == nil {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: notReadyReason, message: fmt.Sprintf("Deployment %s not found", name)}
	}
	if dep.Status.ObservedGeneration < dep.Generation {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: notReadyReason, message: fmt.Sprintf("Deployment %s status is stale", name)}
	}
	if deploymentReplicasReady(dep) {
		return nil
	}
	if failed, msg := deploymentFailed(dep); failed {
		return &modelStatus{phase: v1alpha1.ModelPhaseFailed, reason: failedReason, message: fmt.Sprintf("Deployment %s %s", name, msg)}
	}
	return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: notReadyReason, message: fmt.Sprintf("Deployment %s has insufficient ready replicas", name)}
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

func inferencePoolStatus(pool *inferencev1.InferencePool, name string) *modelStatus {
	if pool == nil {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonInferencePoolNotAccepted, message: fmt.Sprintf("InferencePool %s not found", name)}
	}
	if len(pool.Status.Parents) == 0 {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonInferencePoolNotAccepted, message: fmt.Sprintf("InferencePool %s has no parent status yet", name)}
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
		return &modelStatus{phase: v1alpha1.ModelPhaseFailed, reason: v1alpha1.ModelReasonInferencePoolRejected, message: failedMessage}
	}
	if allReady {
		return nil
	}
	return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonInferencePoolNotAccepted, message: fmt.Sprintf("InferencePool %s not accepted by gateway", name)}
}

func httpRouteStatus(model *v1alpha1.Model, route *gatewayv1.HTTPRoute, gatewayName, listenerName string) *modelStatus {
	name := model.EngineName()
	if route == nil {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonHTTPRouteNotAccepted, message: fmt.Sprintf("HTTPRoute %s not found", name)}
	}

	// The informer cache may still hold the route from before this apply.
	desired := native.BuildHTTPRoute(model, gatewayName, listenerName)
	if !httpRouteServesDesiredMatches(route, desired) {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonHTTPRouteNotAccepted, message: fmt.Sprintf("HTTPRoute %s does not serve the current model names", name)}
	}

	if len(route.Status.Parents) == 0 {
		return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonHTTPRouteNotAccepted, message: fmt.Sprintf("HTTPRoute %s has no parent status yet", name)}
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
		return &modelStatus{phase: v1alpha1.ModelPhaseFailed, reason: v1alpha1.ModelReasonHTTPRouteRejected, message: failedMessage}
	}
	if partiallyInvalid {
		return &modelStatus{phase: v1alpha1.ModelPhaseFailed, reason: v1alpha1.ModelReasonHTTPRoutePartiallyInvalid, message: fmt.Sprintf("HTTPRoute %s is partially invalid", name)}
	}
	if allReady {
		return nil
	}
	return &modelStatus{phase: v1alpha1.ModelPhaseCreating, reason: v1alpha1.ModelReasonHTTPRouteNotAccepted, message: fmt.Sprintf("HTTPRoute %s not accepted by gateway", name)}
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
