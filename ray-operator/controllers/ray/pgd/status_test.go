package pgd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, rayv1.AddToScheme(s))
	require.NoError(t, pgdv1alpha1.AddToScheme(s))
	return s
}

// newPGD reuses the package-shared `testNS` constant defined in
// pgd_helper_test.go; both helpers operate in the same namespace.
func newPGD(name, clusterName, queue string, groups, allocated int32) *pgdv1alpha1.PodGroupDeployment {
	return &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: clusterName},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{
			Groups: groups,
			Queue:  queue,
		},
		Status: pgdv1alpha1.PodGroupDeploymentStatus{
			Summary: pgdv1alpha1.GroupsSummary{
				AllocatedGroups: allocated,
			},
		},
	}
}

func TestQueueStatusReason(t *testing.T) {
	const cluster = "myjob"

	tests := []struct {
		name string
		pgds []*pgdv1alpha1.PodGroupDeployment
		want string
	}{
		{
			name: "no PGDs returns empty",
			pgds: nil,
			want: "",
		},
		{
			name: "all PGDs fully allocated returns empty",
			pgds: []*pgdv1alpha1.PodGroupDeployment{
				newPGD(cluster+"-h", cluster, "freebie", 1, 1),
				newPGD(cluster+"-w-worker", cluster, "freebie", 4, 4),
			},
			want: "",
		},
		{
			name: "head pending shows in reason",
			pgds: []*pgdv1alpha1.PodGroupDeployment{
				newPGD(cluster+"-h", cluster, "freebie", 1, 0),
			},
			want: "PGD queued: myjob-h=0/1 (queue=freebie)",
		},
		{
			name: "multiple pending sorted by name",
			pgds: []*pgdv1alpha1.PodGroupDeployment{
				newPGD(cluster+"-w-worker", cluster, "batch", 4, 1),
				newPGD(cluster+"-h", cluster, "freebie", 1, 0),
			},
			want: "PGD queued: myjob-h=0/1 (queue=freebie), myjob-w-worker=1/4 (queue=batch)",
		},
		{
			name: "queue empty shows <none>",
			pgds: []*pgdv1alpha1.PodGroupDeployment{
				newPGD(cluster+"-h", cluster, "", 1, 0),
			},
			want: "PGD queued: myjob-h=0/1 (queue=<none>)",
		},
		{
			name: "head ready, worker pending — only worker shown",
			pgds: []*pgdv1alpha1.PodGroupDeployment{
				newPGD(cluster+"-h", cluster, "freebie", 1, 1),
				newPGD(cluster+"-w-worker", cluster, "freebie", 4, 2),
			},
			want: "PGD queued: myjob-w-worker=2/4 (queue=freebie)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme(t)
			objs := make([]runtime.Object, 0, len(tt.pgds))
			for _, p := range tt.pgds {
				objs = append(objs, p)
			}
			c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
			h := New(c, scheme)
			got, err := h.QueueStatusReason(context.Background(), &rayv1.RayCluster{
				ObjectMeta: metav1.ObjectMeta{Name: cluster, Namespace: testNS},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestQueueStatusReason_OtherClusterIgnored verifies the label selector scopes to
// the requested cluster only.
func TestQueueStatusReason_OtherClusterIgnored(t *testing.T) {
	scheme := newTestScheme(t)
	objs := []runtime.Object{
		newPGD("ours-h", "ours", "freebie", 1, 0),   // ours, pending
		newPGD("other-h", "other", "freebie", 1, 0), // someone else's, also pending
	}
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	h := New(c, scheme)
	got, err := h.QueueStatusReason(context.Background(), &rayv1.RayCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "ours", Namespace: testNS},
	})
	require.NoError(t, err)
	assert.Equal(t, "PGD queued: ours-h=0/1 (queue=freebie)", got, "must ignore other cluster's PGD")
}
