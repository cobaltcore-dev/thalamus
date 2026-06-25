// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
)

type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	model := &v1alpha1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling model",
		"name", model.Name,
		"namespace", model.Namespace,
	)

	engineType := parseEngineType(model.Spec.Serving.Engine.Image)
	eppType := parseEPPType(model.Spec.Serving.EPP)

	if model.Status.EngineType != engineType || model.Status.EPPType != eppType {
		model.Status.EngineType = engineType
		model.Status.EPPType = eppType
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Model{}).
		Named("model").
		Complete(r)
}
