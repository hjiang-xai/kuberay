package pgd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	schedulerinterface "github.com/ray-project/kuberay/ray-operator/controllers/ray/batchscheduler/interface"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/pgd"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

const testNS = "default"

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, rayv1.AddToScheme(s))
	require.NoError(t, pgdv1alpha1.AddToScheme(s))
	return s
}

func newScheduler(t *testing.T, objs ...runtime.Object) (*Scheduler, client.Client) {
	t.Helper()
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Scheduler{helper: pgd.New(c, scheme)}, c
}

func newRayCluster(name string) *rayv1.RayCluster {
	return &rayv1.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNS,
			UID:       types.UID("test-uid-" + name),
		},
	}
}

func newHeadPod(clusterName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				utils.RayClusterLabelKey:  clusterName,
				utils.RayNodeTypeLabelKey: string(rayv1.HeadNode),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "ray-head", Image: "ray:2.54.0"}},
		},
	}
}

// Both interfaces are satisfied at compile time. If either drifts, the type
// assertion fails with a clear message instead of a wall of method-missing
// errors deep inside the controller wiring.
func TestSchedulerImplementsBothInterfaces(t *testing.T) {
	var s any = &Scheduler{}
	_, isBatch := s.(schedulerinterface.BatchScheduler)
	assert.True(t, isBatch, "Scheduler must implement BatchScheduler")
	_, isPLS := s.(schedulerinterface.PodLifecycleScheduler)
	assert.True(t, isPLS, "Scheduler must implement PodLifecycleScheduler")
}

func TestNameAndPluginName(t *testing.T) {
	s := &Scheduler{}
	assert.Equal(t, "pgd", s.Name())
	assert.Equal(t, "pgd", GetPluginName())
}

func TestDoBatchSchedulingOnSubmissionIsNoop(t *testing.T) {
	s, _ := newScheduler(t)
	// Pass RayCluster, RayJob, and Pod to make sure the no-op applies regardless
	// of object kind. Volcano errors on unknown kinds; PGD must accept all.
	require.NoError(t, s.DoBatchSchedulingOnSubmission(context.Background(), newRayCluster("c1")))
	require.NoError(t, s.DoBatchSchedulingOnSubmission(context.Background(), &rayv1.RayJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: testNS},
	}))
	require.NoError(t, s.DoBatchSchedulingOnSubmission(context.Background(), &corev1.Pod{}))
}

func TestAddMetadataToChildResourceIsNoop(t *testing.T) {
	// AddMetadataToChildResource must NOT mutate the child's labels,
	// annotations, or SchedulerName. PGD owns pod metadata via the gang CR
	// template, not via per-pod stamping.
	s, _ := newScheduler(t)
	parent := newRayCluster("c1")
	child := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"existing": "label"},
			Annotations: map[string]string{"existing": "annotation"},
		},
		Spec: corev1.PodSpec{SchedulerName: "default-scheduler"},
	}
	s.AddMetadataToChildResource(context.Background(), parent, child, "worker")
	assert.Equal(t, map[string]string{"existing": "label"}, child.Labels)
	assert.Equal(t, map[string]string{"existing": "annotation"}, child.Annotations)
	assert.Equal(t, "default-scheduler", child.Spec.SchedulerName)
}

func TestCleanupOnCompletion_NonRayJobIsNoop(t *testing.T) {
	// PGD CleanupOnCompletion only applies to RayJob; a RayCluster passed
	// directly returns (false, nil) so cascade GC handles it.
	s, _ := newScheduler(t)
	didCleanup, err := s.CleanupOnCompletion(context.Background(), newRayCluster("c1"))
	require.NoError(t, err)
	assert.False(t, didCleanup)
}

func TestCleanupOnCompletion_NoRayClusterNameIsNoop(t *testing.T) {
	// RayJob with empty Status.RayClusterName means the cluster was never
	// created; nothing to clean up.
	s, _ := newScheduler(t)
	rj := &rayv1.RayJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: testNS},
		Status:     rayv1.RayJobStatus{RayClusterName: ""},
	}
	didCleanup, err := s.CleanupOnCompletion(context.Background(), rj)
	require.NoError(t, err)
	assert.False(t, didCleanup)
}

func TestCleanupOnCompletion_DeletesAllOwnedPGDs(t *testing.T) {
	// Two PGDs labeled with cluster=myjob plus one PGD belonging to another
	// cluster. Only the myjob ones must be deleted.
	headPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-h",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 1, GroupSize: 1},
	}
	workerPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-w-worker",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 4, GroupSize: 1},
	}
	otherPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-h",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "other"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 1, GroupSize: 1},
	}
	s, c := newScheduler(t, headPGD, workerPGD, otherPGD)

	rj := &rayv1.RayJob{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob-rj", Namespace: testNS},
		Status:     rayv1.RayJobStatus{RayClusterName: "myjob"},
	}
	didCleanup, err := s.CleanupOnCompletion(context.Background(), rj)
	require.NoError(t, err)
	assert.True(t, didCleanup)

	// Both myjob PGDs must be gone.
	for _, n := range []string{"myjob-h", "myjob-w-worker"} {
		got := &pgdv1alpha1.PodGroupDeployment{}
		err := c.Get(context.Background(), types.NamespacedName{Name: n, Namespace: testNS}, got)
		assert.True(t, apierrors.IsNotFound(err), "%s must be deleted, got err=%v", n, err)
	}

	// Other cluster's PGD must remain (label-scoped delete, not blanket).
	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "other-h", Namespace: testNS}, got))
}

func TestUpsertGangForHead_DelegatesToHelper(t *testing.T) {
	// Smoke test: UpsertGangForHead must produce a head PGD CR with Groups=1.
	s, c := newScheduler(t)
	rc := newRayCluster("myjob")
	head := newHeadPod("myjob")
	require.NoError(t, s.UpsertGangForHead(context.Background(), rc, head))

	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, got))
	assert.Equal(t, int32(1), got.Spec.Groups, "head PGD must have Groups=1")
	assert.Equal(t, int32(1), got.Spec.GroupSize)
	assert.False(t, got.Spec.Movable, "head PGD must be non-movable to keep stable identity")
}

func TestUpsertGangForWorker_DelegatesToHelper(t *testing.T) {
	s, c := newScheduler(t)
	rc := newRayCluster("myjob")
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(3)),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				utils.RayClusterLabelKey:   "myjob",
				utils.RayNodeGroupLabelKey: "worker",
			},
		},
	}
	require.NoError(t, s.UpsertGangForWorker(context.Background(), rc, worker, pod))

	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, got))
	assert.Equal(t, int32(3), got.Spec.Groups, "worker PGD Groups must mirror Replicas")
}

func TestSuspendAllGangs_DelegatesToHelper(t *testing.T) {
	headPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-h",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 1, GroupSize: 1, MinGroups: 1},
	}
	s, c := newScheduler(t, headPGD)
	require.NoError(t, s.SuspendAllGangs(context.Background(), newRayCluster("myjob")))

	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, got))
	assert.Equal(t, int32(0), got.Spec.Groups)
	assert.Equal(t, int32(0), got.Spec.MinGroups)
}

func TestSuspendWorkerGang_DelegatesToHelper(t *testing.T) {
	workerPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-w-worker",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 4, GroupSize: 1, MinGroups: 2},
	}
	s, c := newScheduler(t, workerPGD)
	require.NoError(t, s.SuspendWorkerGang(context.Background(), newRayCluster("myjob"), "worker"))

	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, got))
	assert.Equal(t, int32(0), got.Spec.Groups)
	assert.Equal(t, int32(0), got.Spec.MinGroups)
}

func TestMarkAndScaleDownGang_DelegatesToHelper(t *testing.T) {
	// MarkAndScaleDownGang is the only PodLifecycleScheduler method that
	// previously had NO adapter-level test. Helper-level coverage in
	// pgd_helper_test.go is comprehensive; this test pins the adapter's
	// delegation contract: the call must reach the helper, the helper's
	// Patch must take effect, and Spec.Groups must reflect the new Replicas.
	workerPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-w-worker",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 4, GroupSize: 1},
	}
	s, c := newScheduler(t, workerPGD)
	rc := newRayCluster("myjob")
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(3)),
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: []string{}, // no victims; just exercising the Groups patch
		},
	}
	require.NoError(t, s.MarkAndScaleDownGang(context.Background(), rc, worker))

	got := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, got))
	assert.Equal(t, int32(3), got.Spec.Groups, "adapter must propagate Replicas through to the helper's Groups patch")
}

func TestDeleteAllGangs_DelegatesToHelper(t *testing.T) {
	headPGD := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-h",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
	}
	s, c := newScheduler(t, headPGD)
	require.NoError(t, s.DeleteAllGangs(context.Background(), newRayCluster("myjob")))

	got := &pgdv1alpha1.PodGroupDeployment{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, got)
	assert.True(t, apierrors.IsNotFound(err), "head PGD must be deleted")
}

func TestQueueStatusReason_DelegatesToHelper(t *testing.T) {
	pgdCR := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myjob-h",
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: "myjob"},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{Groups: 1, Queue: "freebie"},
		Status: pgdv1alpha1.PodGroupDeploymentStatus{
			Summary: pgdv1alpha1.GroupsSummary{AllocatedGroups: 0},
		},
	}
	s, _ := newScheduler(t, pgdCR)
	got, err := s.QueueStatusReason(context.Background(), newRayCluster("myjob"))
	require.NoError(t, err)
	assert.Equal(t, "PGD queued: myjob-h=0/1 (queue=freebie)", got)
}

func TestFactoryAddToScheme(t *testing.T) {
	// Factory.AddToScheme must register the PGD types so the runtime can
	// decode PodGroupDeployment objects from the API server.
	scheme := runtime.NewScheme()
	(&Factory{}).AddToScheme(scheme)
	gvk := pgdv1alpha1.GroupVersion.WithKind("PodGroupDeployment")
	_, err := scheme.New(gvk)
	require.NoError(t, err, "PGD GVK must be registered in scheme")
}
