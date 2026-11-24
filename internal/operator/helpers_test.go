package operator

import (
	"context"
	"testing"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	testNamespace       = "default"
	testGatewayName     = "test-gateway"
	testGatewayListener = "test-listener"
	testRouterSAName    = "thalamus-router"
)

type wantCondition struct {
	typ    string
	status bool
	reason v1alpha1.ModelReason
}

func (c wantCondition) assert(t *testing.T, model *v1alpha1.Model) {
	t.Helper()
	cond := meta.FindStatusCondition(model.Status.Conditions, c.typ)
	if cond == nil {
		t.Fatalf("%s condition not found on model", c.typ)
	}

	wantStatus := metav1.ConditionFalse
	if c.status {
		wantStatus = metav1.ConditionTrue
	}

	if cond.Status != wantStatus {
		t.Errorf("%s condition status:\ngot:  %q\nwant: %q", c.typ, cond.Status, wantStatus)
	}
	if cond.Reason != string(c.reason) {
		t.Errorf("%s condition reason:\ngot:  %q\nwant: %q", c.typ, cond.Reason, c.reason)
	}
}

// newTestReconciler returns a ModelReconciler backed by a fake client seeded
// with the given objects.
func newTestReconciler(t *testing.T, objs ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()
	s := testutil.NewScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.Model{}, &appsv1.Deployment{}, &inferencev1.InferencePool{}, &gatewayv1.HTTPRoute{}).
		Build()

	return &ModelReconciler{
		Client:                   c,
		Scheme:                   s,
		GatewayName:              testGatewayName,
		ListenerName:             testGatewayListener,
		RouterServiceAccountName: testRouterSAName,
	}, c
}

// reconcileModelOnce runs a single reconciliation for the named Model and fails
// the test if Reconcile returns an error, model is absent or Synced condition is not met.
func reconcileModelOnce(t *testing.T, r *ModelReconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		Name: name, Namespace: testNamespace,
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	model := new(v1alpha1.Model)
	testutil.MustGet(t, r.Client, name, testNamespace, model)
	wantCondition{v1alpha1.ModelConditionSynced, true, v1alpha1.ModelReasonSynced}.assert(t, model)

	return res
}
