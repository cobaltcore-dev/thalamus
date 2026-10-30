// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/cobaltcore-dev/thalamus/internal/operator/testutil"
)

func TestGetObject(t *testing.T) {
	pod := &corev1.Pod{Name: "test-pod", Namespace: "default"}

	tests := []struct {
		name    string
		scheme  *runtime.Scheme
		objects []client.Object
		want    string
		wantErr bool
	}{
		{
			name:    "returns the object",
			objects: []client.Object{pod},
			want:    pod.Name,
		},
		{
			name: "returns nil when missing",
		},
		{
			name:    "returns errors from the client",
			scheme:  runtime.NewScheme(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := tt.scheme
			if scheme == nil {
				scheme = testutil.NewScheme(t)
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

			got, err := getObject(context.Background(), c, pod.Namespace, pod.Name, &corev1.Pod{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error:\ngot:  %v\nwant error: %t", err, tt.wantErr)
			}
			if tt.want == "" {
				if got != nil {
					t.Fatalf("expected nil object, got %v", got)
				}
				return
			}
			if got == nil || got.Name != tt.want {
				t.Fatalf("object:\ngot:  %v\nwant name: %q", got, tt.want)
			}
		})
	}
}
