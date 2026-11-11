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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

// patchModelStatus persists status changes against the snapshot taken before reconciliation.
func (r *ModelReconciler) patchModelStatus(ctx context.Context, before, model *v1alpha1.Model) error {
	if apiequality.Semantic.DeepEqual(before.Status, model.Status) {
		return nil
	}
	if cond := meta.FindStatusCondition(model.Status.Conditions, v1alpha1.ModelConditionReady); cond != nil {
		log.FromContext(ctx).V(levelInfo).Info("model status changed", "namespace", model.Namespace, "name", model.Name, "ready", cond.Status, "reason", cond.Reason, "message", cond.Message)
	}
	if err := r.Status().Patch(ctx, model, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch model status %s/%s: %w", model.Namespace, model.Name, err)
	}
	return nil
}

// modelCondition builds a condition for the model's current generation.
func modelCondition(model *v1alpha1.Model, conditionType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: model.Generation,
	}
}

// readyCondition builds the Ready condition.
func readyCondition(model *v1alpha1.Model, status metav1.ConditionStatus, reason v1alpha1.ModelReason, message string) metav1.Condition {
	return modelCondition(model, v1alpha1.ModelConditionReady, status, string(reason), message)
}

// artifactReadyCondition reports artifact lifecycle readiness, independently of serving health.
func artifactReadyCondition(model *v1alpha1.Model, artifact *v1alpha1.ModelArtifact) metav1.Condition {
	status := metav1.ConditionFalse
	var reason v1alpha1.ModelArtifactReason
	var message string
	switch {
	case artifact == nil:
		reason = v1alpha1.ModelArtifactReasonNotFound
		message = fmt.Sprintf("model artifact %s not found; recreate the artifact or delete the Model to resolve this", model.Spec.ArtifactRef.Name)
	case artifact.GetDeletionTimestamp() != nil:
		reason = v1alpha1.ModelArtifactReasonTerminating
		message = fmt.Sprintf("model artifact %s is being deleted; delete the Model to let the deletion complete", model.Spec.ArtifactRef.Name)
	case !isArtifactUsable(artifact):
		reason = v1alpha1.ModelArtifactReasonNotReady
		message = fmt.Sprintf("model artifact %s is not protected by the artifact controller yet", model.Spec.ArtifactRef.Name)
	default:
		status = metav1.ConditionTrue
		reason = v1alpha1.ModelArtifactReasonReady
		message = fmt.Sprintf("model artifact %s is available and protected", model.Spec.ArtifactRef.Name)
	}
	return modelCondition(model, v1alpha1.ModelConditionArtifactReady, status, string(reason), message)
}

// notReadyCondition builds a Ready=False condition.
func notReadyCondition(model *v1alpha1.Model, reason v1alpha1.ModelReason, message string) *metav1.Condition {
	return new(readyCondition(model, metav1.ConditionFalse, reason, message))
}

// syncModelStatus derives the model's ready condition.
func (r *ModelReconciler) syncModelStatus(ctx context.Context, model *v1alpha1.Model, identifiers []v1alpha1.Identifier) (metav1.Condition, error) {
	if model.Spec.Replicas == 0 {
		return readyCondition(model, metav1.ConditionFalse, v1alpha1.ModelReasonNoReplicasDesired, "model has no desired replicas"), nil
	}

	resources := []struct {
		name          string
		syncCondition func(context.Context, *v1alpha1.Model) (*metav1.Condition, error)
	}{
		{"engine", r.syncEngineCondition},
		{"epp", r.syncEPPCondition},
		{"inference pool", r.syncInferencePoolCondition},
		{"http route", func(ctx context.Context, m *v1alpha1.Model) (*metav1.Condition, error) {
			return r.syncHTTPRouteCondition(ctx, m, identifiers)
		}},
	}

	for _, resource := range resources {
		condition, err := resource.syncCondition(ctx, model)
		if err != nil {
			return metav1.Condition{}, fmt.Errorf("sync %s condition failed: %w", resource.name, err)
		}
		if condition != nil {
			return *condition, nil
		}
	}

	return readyCondition(model, metav1.ConditionTrue, v1alpha1.ModelReasonReady, "Model is reachable"), nil
}

// syncEngineCondition checks the engine Deployment.
func (r *ModelReconciler) syncEngineCondition(ctx context.Context, model *v1alpha1.Model) (*metav1.Condition, error) {
	return r.syncDeploymentCondition(ctx, model, model.EngineName(), v1alpha1.ModelReasonEngineNotReady, v1alpha1.ModelReasonEngineDeploymentFailed)
}

// syncEPPCondition checks the EPP Deployment.
func (r *ModelReconciler) syncEPPCondition(ctx context.Context, model *v1alpha1.Model) (*metav1.Condition, error) {
	return r.syncDeploymentCondition(ctx, model, model.EPPName(), v1alpha1.ModelReasonEPPNotReady, v1alpha1.ModelReasonEPPDeploymentFailed)
}

// syncDeploymentCondition checks a child Deployment.
func (r *ModelReconciler) syncDeploymentCondition(ctx context.Context, model *v1alpha1.Model, name string, notReadyReason, failedReason v1alpha1.ModelReason) (*metav1.Condition, error) {
	dep, err := getObject(ctx, r.Client, model.Namespace, name, &appsv1.Deployment{})
	switch {
	case err != nil:
		return nil, err
	case dep == nil:
		return notReadyCondition(model, notReadyReason, fmt.Sprintf("Deployment %s not found", name)), nil
	}

	if dep.Status.ObservedGeneration < dep.Generation {
		return notReadyCondition(model, notReadyReason, fmt.Sprintf("Deployment %s status is stale", name)), nil
	}

	if deploymentReplicasReady(dep) {
		return nil, nil
	}

	if failed, msg := deploymentFailed(dep); failed {
		return notReadyCondition(model, failedReason, fmt.Sprintf("Deployment %s %s", name, msg)), nil
	}

	return notReadyCondition(model, notReadyReason, fmt.Sprintf("Deployment %s has insufficient ready replicas", name)), nil
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

// syncInferencePoolCondition checks the InferencePool.
func (r *ModelReconciler) syncInferencePoolCondition(ctx context.Context, model *v1alpha1.Model) (*metav1.Condition, error) {
	pool, err := getObject(ctx, r.Client, model.Namespace, model.EngineName(), &inferencev1.InferencePool{})
	switch {
	case err != nil:
		return nil, err
	case pool == nil:
		return notReadyCondition(model, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s not found", model.EngineName())), nil
	case len(pool.Status.Parents) == 0:
		return notReadyCondition(model, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s has no parent status yet", model.EngineName())), nil
	}

	allReady := true
	var failedMessage string
	for _, parent := range pool.Status.Parents {
		ready, failed, message := checkParentConditions(parent.Conditions, pool.Generation, inferencePoolParentPolicy)
		if !ready {
			allReady = false
		}
		if failed {
			failedMessage = message
		}
	}

	if failedMessage != "" {
		return notReadyCondition(model, v1alpha1.ModelReasonInferencePoolRejected, failedMessage), nil
	}
	if allReady {
		return nil, nil
	}
	return notReadyCondition(model, v1alpha1.ModelReasonInferencePoolNotAccepted, fmt.Sprintf("InferencePool %s not accepted by gateway", model.EngineName())), nil
}

// syncHTTPRouteCondition checks the HTTPRoute.
func (r *ModelReconciler) syncHTTPRouteCondition(ctx context.Context, model *v1alpha1.Model, identifiers []v1alpha1.Identifier) (*metav1.Condition, error) {
	route, err := getObject(ctx, r.Client, model.Namespace, model.EngineName(), &gatewayv1.HTTPRoute{})
	switch {
	case err != nil:
		return nil, err
	case route == nil:
		return notReadyCondition(model, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s not found", model.EngineName())), nil
	}

	// The informer cache may still hold the route from before this apply.
	desired := native.BuildHTTPRoute(model, identifiers, r.GatewayName, r.ListenerName)
	if !httpRouteServesDesiredMatches(route, desired) {
		return notReadyCondition(model, v1alpha1.ModelReasonHTTPRouteNotAccepted,
			fmt.Sprintf("HTTPRoute %s does not serve the current model names", model.EngineName())), nil
	}

	if len(route.Status.Parents) == 0 {
		return notReadyCondition(model, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s has no parent status yet", model.EngineName())), nil
	}

	allReady := true
	var failedMessage string
	partiallyInvalid := false
	for _, parent := range route.Status.Parents {
		ready, failed, message := checkParentConditions(parent.Conditions, route.Generation, httpRouteParentPolicy)
		if !ready {
			allReady = false
		}
		if failed {
			failedMessage = message
		}
		partiallyInvalidCond := meta.FindStatusCondition(parent.Conditions, string(gatewayv1.RouteConditionPartiallyInvalid))
		if partiallyInvalidCond != nil && partiallyInvalidCond.Status == metav1.ConditionTrue &&
			partiallyInvalidCond.ObservedGeneration >= route.Generation {
			partiallyInvalid = true
		}
	}

	if failedMessage != "" {
		return notReadyCondition(model, v1alpha1.ModelReasonHTTPRouteRejected, failedMessage), nil
	}
	if partiallyInvalid {
		return notReadyCondition(model, v1alpha1.ModelReasonHTTPRoutePartiallyInvalid, fmt.Sprintf("HTTPRoute %s is partially invalid", model.EngineName())), nil
	}
	if allReady {
		return nil, nil
	}
	return notReadyCondition(model, v1alpha1.ModelReasonHTTPRouteNotAccepted, fmt.Sprintf("HTTPRoute %s not accepted by gateway", model.EngineName())), nil
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

// checkParentConditions reports whether one parent is ready or has failed
// terminally. Stale conditions are pending. It is ready only when the accepted
// condition is True and the resolved condition is absent or True; a False
// condition is a terminal failure unless its reason is transient.
func checkParentConditions(
	conditions []metav1.Condition,
	generation int64,
	policy parentStatusPolicy,
) (ready, failed bool, message string) {

	accepted := meta.FindStatusCondition(conditions, policy.acceptedType)
	resolved := meta.FindStatusCondition(conditions, policy.resolvedType)

	condFalse := func(cond *metav1.Condition) {
		message = fmt.Sprintf("%s: %s", cond.Reason, cond.Message)
		_, isTransient := policy.transientReasons[cond.Reason]
		failed = !isTransient
	}

	switch {
	// Stale conditions may still report the previous spec as accepted.
	case accepted != nil && accepted.ObservedGeneration < generation:
		message = accepted.Type + " condition is stale"
	case resolved != nil && resolved.ObservedGeneration < generation:
		message = resolved.Type + " condition is stale"
	case accepted != nil && accepted.Status == metav1.ConditionTrue &&
		(resolved == nil || resolved.Status == metav1.ConditionTrue):
		ready = true
	case resolved != nil && resolved.Status == metav1.ConditionFalse:
		condFalse(resolved)
	case accepted != nil && accepted.Status == metav1.ConditionFalse:
		condFalse(accepted)
	default:
		message = "parent status is pending"
	}

	return
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
