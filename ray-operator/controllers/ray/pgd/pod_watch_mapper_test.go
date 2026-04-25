package pgd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
)

func TestMapPodToRayCluster(t *testing.T) {
	tests := []struct {
		name string
		obj  client.Object
		want []reconcile.Request
	}{
		{
			name: "non-pod returns nil",
			obj:  &corev1.Service{},
			want: nil,
		},
		{
			name: "pod with no labels returns nil",
			obj:  &corev1.Pod{},
			want: nil,
		},
		{
			name: "pod with cluster label returns matching request",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1",
					Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
				},
			},
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "myjob", Namespace: "ns1"}}},
		},
		{
			name: "pod with empty cluster label returns nil",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1",
					Labels:    map[string]string{utils.RayClusterLabelKey: ""},
				},
			},
			want: nil,
		},
		{
			name: "pod without cluster label (other labels present) returns nil",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1",
					Labels:    map[string]string{"foo": "bar"},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapPodToRayCluster(context.Background(), tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}
