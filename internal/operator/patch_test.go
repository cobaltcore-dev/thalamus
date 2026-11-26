// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

// newPatchTestClient returns a fake client with the test pod present.
func newPatchTestClient(t *testing.T) client.Client {
	t.Helper()
	pod := &corev1.Pod{Name: "test-pod", Namespace: testNamespace}
	return fake.NewClientBuilder().WithScheme(testutil.NewScheme(t)).
		WithObjects(pod).WithStatusSubresource(&corev1.Pod{}).Build()
}

// fetchPod returns the persisted test pod.
func fetchPod(t *testing.T, c client.Client) *corev1.Pod {
	t.Helper()
	pod := &corev1.Pod{}
	testutil.MustGet(t, c, "test-pod", testNamespace, pod)
	return pod
}

func TestPatchStatusIfChanged(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*corev1.Pod)
		wantChange bool
	}{
		{
			name: "patches changed status",
			mutate: func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodRunning
			},
			wantChange: true,
		},
		{
			name:   "skips unchanged status",
			mutate: func(*corev1.Pod) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newPatchTestClient(t)
			before := fetchPod(t, c)
			pod := before.DeepCopy()
			tt.mutate(pod)

			changed, err := patchStatusIfChanged(context.Background(), c, before, pod)
			if err != nil {
				t.Fatalf("error:\ngot:  %v", err)
			}
			if changed != tt.wantChange {
				t.Errorf("changed:\ngot:  %t\nwant: %t", changed, tt.wantChange)
			}
			if tt.wantChange {
				updated := fetchPod(t, c)
				if updated.Status.Phase != corev1.PodRunning {
					t.Errorf("patched status:\ngot:  %+v", updated.Status)
				}
			}
		})
	}
}

func TestPatchStatusIfChanged_ObjectDeleted(t *testing.T) {
	c := newPatchTestClient(t)
	before := fetchPod(t, c)
	if err := c.Delete(context.Background(), before.DeepCopy()); err != nil {
		t.Fatalf("delete pod: %v", err)
	}
	pod := before.DeepCopy()
	pod.Status.Phase = corev1.PodRunning

	if changed, err := patchStatusIfChanged(context.Background(), c, before, pod); err == nil || changed {
		t.Fatalf("error:\ngot:  %v (changed %t)\nwant: not-found error", err, changed)
	}
}

func TestPatchFinalizersIfChanged(t *testing.T) {
	const testFinalizer = "test.example.com/protect"

	tests := []struct {
		name       string
		mutate     func(*corev1.Pod)
		wantChange bool
	}{
		{
			name: "patches changed finalizers",
			mutate: func(p *corev1.Pod) {
				p.Finalizers = []string{testFinalizer}
			},
			wantChange: true,
		},
		{
			name:   "skips unchanged finalizers",
			mutate: func(*corev1.Pod) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newPatchTestClient(t)
			before := fetchPod(t, c)
			pod := before.DeepCopy()
			tt.mutate(pod)

			changed, err := patchFinalizersIfChanged(context.Background(), c, before, pod)
			if err != nil {
				t.Fatalf("error:\ngot:  %v", err)
			}
			if changed != tt.wantChange {
				t.Errorf("changed:\ngot:  %t\nwant: %t", changed, tt.wantChange)
			}
			if tt.wantChange {
				updated := fetchPod(t, c)
				if len(updated.Finalizers) != 1 || updated.Finalizers[0] != testFinalizer {
					t.Errorf("patched finalizers:\ngot:  %v", updated.Finalizers)
				}
			}
		})
	}
}

func TestPatchFinalizersIfChanged_ObjectDeleted(t *testing.T) {
	c := newPatchTestClient(t)
	before := fetchPod(t, c)
	if err := c.Delete(context.Background(), before.DeepCopy()); err != nil {
		t.Fatalf("delete pod: %v", err)
	}
	pod := before.DeepCopy()
	pod.Finalizers = []string{"test.example.com/protect"}

	if changed, err := patchFinalizersIfChanged(context.Background(), c, before, pod); err == nil || changed {
		t.Fatalf("error:\ngot:  %v (changed %t)\nwant: not-found error", err, changed)
	}
}
