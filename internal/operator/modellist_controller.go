// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/resources/native"
)

var agentgatewayPolicyGVK = schema.GroupVersionKind{
	Group:   "agentgateway.dev",
	Version: "v1alpha1",
	Kind:    "AgentgatewayPolicy",
}

const (
	modelListContentTypeHeader = "Content-Type"
	modelListContentTypeValue  = "application/json"
	modelListDirectStatus      = 200
)

// ModelListReconciler keeps the model-list AgentgatewayPolicy in sync
// with the set of Ready Model CRs in each namespace.
type ModelListReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ModelListReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// List all Models in the namespace.
	var modelList v1alpha1.ModelList
	if err := r.List(ctx, &modelList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	body, err := native.BuildModelListResponse(modelList.Items)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch the AgentgatewayPolicy, that provides `/v1/models` via directResponse.
	policy := &unstructured.Unstructured{}
	policy.SetGroupVersionKind(agentgatewayPolicyGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: native.ModelListPolicyName, Namespace: req.Namespace}, policy); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the direct response already matches to avoid unnecessary updates.
	currentBody, _, err := unstructured.NestedString(policy.Object, "spec", "traffic", "directResponse", "body")
	if err != nil {
		return ctrl.Result{}, err
	}
	currentStatus, _, err := unstructured.NestedInt64(policy.Object, "spec", "traffic", "directResponse", "status")
	if err != nil {
		return ctrl.Result{}, err
	}
	currentContentType, found, err := directResponseHeaderValue(policy.Object, modelListContentTypeHeader)
	if err != nil {
		return ctrl.Result{}, err
	}
	if currentBody == body && currentStatus == modelListDirectStatus && found && currentContentType == modelListContentTypeValue {
		return ctrl.Result{}, nil
	}

	// Build a minimal apply object so the operator only claims ownership of the fields it sets.
	patch := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "agentgateway.dev/v1alpha1",
		"kind":       "AgentgatewayPolicy",
		"metadata": map[string]any{
			"name":      native.ModelListPolicyName,
			"namespace": req.Namespace,
		},
		"spec": map[string]any{
			"traffic": map[string]any{
				"directResponse": map[string]any{
					"status": modelListDirectStatus,
					"body":   body,
					"headers": []map[string]any{
						{"name": modelListContentTypeHeader, "value": modelListContentTypeValue},
					},
				},
			},
		},
	}}
	if err := r.Apply(ctx,
		client.ApplyConfigurationFromUnstructured(patch),
		client.FieldOwner("thalamus-operator"),
		client.ForceOwnership, // required since helm initially owns the body field
	); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("synced model-list policy", "namespace", req.Namespace, "models", len(modelList.Items))
	return ctrl.Result{}, nil
}

func (r *ModelListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	policyType := &unstructured.Unstructured{}
	policyType.SetGroupVersionKind(agentgatewayPolicyGVK)

	return ctrl.NewControllerManagedBy(mgr).
		// Use a Watch (not For) so the reconcile key is always the fixed model-list name,
		// not the individual Model name.
		Watches(&v1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      native.ModelListPolicyName,
						Namespace: obj.GetNamespace(),
					},
				}}
			}),
			builder.WithPredicates(phaseChangedPredicate{})).
		Watches(policyType, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				}}
			}),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("model-list").
		Complete(r)
}

// directResponseHeaderValue returns the value of the named header in the policy's
// spec.traffic.directResponse.headers list, if present.
func directResponseHeaderValue(obj map[string]any, name string) (string, bool, error) {
	headers, found, err := unstructured.NestedSlice(obj, "spec", "traffic", "directResponse", "headers")
	if err != nil || !found {
		return "", false, err
	}
	for _, h := range headers {
		header, ok := h.(map[string]any)
		if !ok {
			continue
		}
		headerName, ok := header["name"].(string)
		if !ok {
			continue
		}
		if headerName == name {
			value, ok := header["value"].(string)
			return value, ok, nil
		}
	}
	return "", false, nil
}

// phaseChangedPredicate fires only when a Model is created, deleted, or its phase changes to/from Ready.
type phaseChangedPredicate struct{ predicate.Funcs }

func (phaseChangedPredicate) Update(e event.UpdateEvent) bool {
	oldModel, ok1 := e.ObjectOld.(*v1alpha1.Model)
	newModel, ok2 := e.ObjectNew.(*v1alpha1.Model)
	if !ok1 || !ok2 {
		return true
	}
	oldPhase := oldModel.Status.Phase
	newPhase := newModel.Status.Phase
	return oldPhase != newPhase && (oldPhase == v1alpha1.ModelPhaseReady || newPhase == v1alpha1.ModelPhaseReady)
}
