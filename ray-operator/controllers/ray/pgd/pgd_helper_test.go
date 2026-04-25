package pgd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

const testNS = "default"

func newRayCluster(name string, ann map[string]string) *rayv1.RayCluster {
	return &rayv1.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   testNS,
			UID:         types.UID("test-uid-" + name),
			Annotations: ann,
		},
	}
}

func newSamplePod(clusterName, groupName string) *corev1.Pod {
	labels := map[string]string{utils.RayClusterLabelKey: clusterName}
	if groupName != "" {
		labels[utils.RayNodeGroupLabelKey] = groupName
	} else {
		labels[utils.RayNodeTypeLabelKey] = string(rayv1.HeadNode)
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: map[string]string{"ray.io/test": "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "ray-worker", Image: "rayproject/ray:2.54.0"},
			},
		},
	}
}

func newWorkerSpec(group string, replicas, minReplicas int32, numOfHosts int32) *rayv1.WorkerGroupSpec {
	return &rayv1.WorkerGroupSpec{
		GroupName:   group,
		Replicas:    ptr.To(replicas),
		MinReplicas: ptr.To(minReplicas),
		NumOfHosts:  numOfHosts,
	}
}

func TestUpsertPGDForHead_Create(t *testing.T) {
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", map[string]string{
		PGDQueueAnnotation:    "freebie",
		PGDPriorityAnnotation: "100",
	})
	headPod := newSamplePod("myjob", "")

	require.NoError(t, h.UpsertPGDForHead(context.Background(), rc, headPod))

	pgd := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, pgd))

	assert.Equal(t, int32(1), pgd.Spec.Groups, "head Groups must be 1")
	assert.Equal(t, int32(1), pgd.Spec.GroupSize, "head GroupSize must be 1")
	assert.Equal(t, int32(1), pgd.Spec.MinGroups, "head MinGroups must be 1 (always present)")
	assert.Equal(t, "kubernetes.io/hostname", pgd.Spec.RequiredTopologyKey, "head must be single-node")
	assert.False(t, pgd.Spec.Movable, "head must not be Movable (concern #13)")
	assert.Equal(t, "freebie", pgd.Spec.Queue)
	assert.Equal(t, int32(100), pgd.Spec.Priority)
	assert.Equal(t, "myjob", pgd.Labels[utils.RayClusterLabelKey])
	assert.Equal(t, string(rayv1.HeadNode), pgd.Labels[utils.RayNodeTypeLabelKey])

	// OwnerReference must point at the RayCluster.
	require.Len(t, pgd.OwnerReferences, 1)
	assert.Equal(t, "myjob", pgd.OwnerReferences[0].Name)
	assert.Equal(t, "RayCluster", pgd.OwnerReferences[0].Kind)
}

func TestUpsertPGDForHead_Idempotent(t *testing.T) {
	// Two consecutive calls with the same input must NOT bump generation
	// (otherwise PGD's version-hash drifts and triggers pod recreation).
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", map[string]string{PGDQueueAnnotation: "freebie"})
	headPod := newSamplePod("myjob", "")

	require.NoError(t, h.UpsertPGDForHead(context.Background(), rc, headPod))
	pgd1 := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, pgd1))
	gen1 := pgd1.Generation
	rv1 := pgd1.ResourceVersion

	require.NoError(t, h.UpsertPGDForHead(context.Background(), rc, headPod))
	pgd2 := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, pgd2))

	assert.Equal(t, gen1, pgd2.Generation, "second identical upsert must not bump generation")
	assert.Equal(t, rv1, pgd2.ResourceVersion, "second identical upsert must not bump resourceVersion")
}

func TestUpsertPGDForGroup_Create(t *testing.T) {
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", map[string]string{
		PGDQueueAnnotation:    "freebie",
		PGDPriorityAnnotation: "50",
	})
	worker := newWorkerSpec("worker", 4, 1, 1)
	pod := newSamplePod("myjob", "worker")

	require.NoError(t, h.UpsertPGDForGroup(context.Background(), rc, worker, pod))

	pgd := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, pgd))

	assert.Equal(t, int32(4), pgd.Spec.Groups, "worker Groups must equal Replicas")
	assert.Equal(t, int32(1), pgd.Spec.GroupSize, "single-host => GroupSize=1")
	assert.Equal(t, int32(1), pgd.Spec.MinGroups, "MinGroups must equal MinReplicas")
	assert.Empty(t, pgd.Spec.RequiredTopologyKey, "single-host should not require topology")
	assert.False(t, pgd.Spec.Movable)
	assert.Equal(t, "freebie", pgd.Spec.Queue)
	assert.Equal(t, int32(50), pgd.Spec.Priority)
	assert.Equal(t, "worker", pgd.Labels[utils.RayNodeGroupLabelKey])
}

func TestUpsertPGDForGroup_MultiHostGangScheduling(t *testing.T) {
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := newWorkerSpec("worker", 2, 1, 4) // 2 groups × 4 hosts each
	pod := newSamplePod("myjob", "worker")

	require.NoError(t, h.UpsertPGDForGroup(context.Background(), rc, worker, pod))

	pgd := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, pgd))

	assert.Equal(t, int32(2), pgd.Spec.Groups)
	assert.Equal(t, int32(4), pgd.Spec.GroupSize, "NumOfHosts > 1 => GroupSize == NumOfHosts")
	assert.Equal(t, "kubernetes.io/hostname", pgd.Spec.RequiredTopologyKey, "multi-host gang must require topology")
}

func TestUpsertPGDForGroup_GroupByKey(t *testing.T) {
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", map[string]string{PGDGroupByKeyAnnotation: "atomic-key"})
	worker := newWorkerSpec("worker", 1, 1, 1)
	pod := newSamplePod("myjob", "worker")

	require.NoError(t, h.UpsertPGDForGroup(context.Background(), rc, worker, pod))

	pgd := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, pgd))

	require.NotNil(t, pgd.Spec.GroupBy, "GroupBy must be set when annotation present")
	assert.Equal(t, "atomic-key", pgd.Spec.GroupBy.Key)
}

func TestUpsertPGDForGroup_NoReplicas(t *testing.T) {
	// Replicas == nil should map to Groups=0 (not panic).
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{GroupName: "worker", Replicas: nil}
	pod := newSamplePod("myjob", "worker")

	require.NoError(t, h.UpsertPGDForGroup(context.Background(), rc, worker, pod))

	pgd := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, pgd))
	assert.Equal(t, int32(0), pgd.Spec.Groups)
}

func TestMarkAndScaleDown_LabelsVictimAndDecrementsGroups(t *testing.T) {
	scheme := newTestScheme(t)

	// Existing PGD with Groups=4 and a victim pod that should get labeled.
	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob-w-worker", Namespace: testNS},
		Spec:       pgdv1alpha1.PodGroupDeploymentSpec{Groups: 4, GroupSize: 1},
	}
	victim := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "victim-pod-0", Namespace: testNS},
	}
	other := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other-pod-0", Namespace: testNS},
	}
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pgd, victim, other).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(3)), // scale down 4 → 3
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: []string{"victim-pod-0"},
		},
	}

	require.NoError(t, h.MarkAndScaleDown(context.Background(), rc, worker))

	// Victim pod must have delete-next label.
	updated := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "victim-pod-0", Namespace: testNS}, updated))
	_, labeled := updated.Labels[DeleteNextLabelKey]
	assert.True(t, labeled, "victim pod must have delete-next label")

	// Other pod must NOT have delete-next label.
	otherUpdated := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "other-pod-0", Namespace: testNS}, otherUpdated))
	_, otherLabeled := otherUpdated.Labels[DeleteNextLabelKey]
	assert.False(t, otherLabeled, "non-victim pod must NOT have delete-next label")

	// PGD Groups must be decremented to 3.
	updatedPGD := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, updatedPGD))
	assert.Equal(t, int32(3), updatedPGD.Spec.Groups)
}

func TestMarkAndScaleDown_NoOpWhenGroupsAlreadyMatch(t *testing.T) {
	scheme := newTestScheme(t)

	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob-w-worker", Namespace: testNS},
		Spec:       pgdv1alpha1.PodGroupDeploymentSpec{Groups: 3, GroupSize: 1},
	}
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pgd).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(3)), // already matches
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: nil,
		},
	}
	require.NoError(t, h.MarkAndScaleDown(context.Background(), rc, worker))

	updatedPGD := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, updatedPGD))
	assert.Equal(t, int32(3), updatedPGD.Spec.Groups, "no change expected")
}

func TestMarkAndScaleDown_NoPGDYet(t *testing.T) {
	// PGD doesn't exist yet — MarkAndScaleDown must not error (UpsertPGDForGroup
	// will create with the right Groups on the next reconcile).
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(2)),
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: []string{"nonexistent-pod"},
		},
	}
	require.NoError(t, h.MarkAndScaleDown(context.Background(), rc, worker), "missing PGD or pod must not error")
}

func TestMarkAndScaleDown_VictimPodGone(t *testing.T) {
	// One of the victims is already gone; should not error and should still patch the PGD.
	scheme := newTestScheme(t)
	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob-w-worker", Namespace: testNS},
		Spec:       pgdv1alpha1.PodGroupDeploymentSpec{Groups: 2, GroupSize: 1},
	}
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pgd).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(1)),
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: []string{"already-gone-pod"},
		},
	}
	require.NoError(t, h.MarkAndScaleDown(context.Background(), rc, worker))

	updatedPGD := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, updatedPGD))
	assert.Equal(t, int32(1), updatedPGD.Spec.Groups, "PGD must still be patched even if victim already gone")
}

func TestMarkAndScaleDown_AlreadyLabeledIsNoOp(t *testing.T) {
	// Pod already has delete-next label; should not re-patch.
	scheme := newTestScheme(t)
	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob-w-worker", Namespace: testNS},
		Spec:       pgdv1alpha1.PodGroupDeploymentSpec{Groups: 2, GroupSize: 1},
	}
	victim := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "victim-pod-0",
			Namespace: testNS,
			Labels:    map[string]string{DeleteNextLabelKey: ""},
		},
	}
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pgd, victim).Build()
	h := New(c, scheme)

	rc := newRayCluster("myjob", nil)
	worker := &rayv1.WorkerGroupSpec{
		GroupName: "worker",
		Replicas:  ptr.To(int32(1)),
		ScaleStrategy: rayv1.ScaleStrategy{
			WorkersToDelete: []string{"victim-pod-0"},
		},
	}
	require.NoError(t, h.MarkAndScaleDown(context.Background(), rc, worker))

	updated := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "victim-pod-0", Namespace: testNS}, updated))
	_, labeled := updated.Labels[DeleteNextLabelKey]
	assert.True(t, labeled, "label must still be present")
}

// newOwnedPGD returns a PGD labeled as belonging to the given RayCluster, ready
// to be loaded into the fake client.
func newOwnedPGD(name, clusterName string, groups, minGroups int32) *pgdv1alpha1.PodGroupDeployment {
	return &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNS,
			Labels:    map[string]string{utils.RayClusterLabelKey: clusterName},
		},
		Spec: pgdv1alpha1.PodGroupDeploymentSpec{
			Groups:    groups,
			GroupSize: 1,
			MinGroups: minGroups,
		},
	}
}

func TestSuspendPGDs_SetsGroupsAndMinGroupsToZero(t *testing.T) {
	scheme := newTestScheme(t)
	head := newOwnedPGD("myjob-h", "myjob", 1, 1)
	worker := newOwnedPGD("myjob-w-worker", "myjob", 4, 2)
	other := newOwnedPGD("notmine-h", "notmine", 1, 1) // different cluster, must not be touched
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(head, worker, other).Build()
	h := New(c, scheme)

	require.NoError(t, h.SuspendPGDs(context.Background(), newRayCluster("myjob", nil)))

	// Both myjob PGDs go to Groups=0, MinGroups=0.
	for _, name := range []string{"myjob-h", "myjob-w-worker"} {
		got := &pgdv1alpha1.PodGroupDeployment{}
		require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNS}, got))
		assert.Equal(t, int32(0), got.Spec.Groups, "%s: Groups must be zeroed during suspend", name)
		assert.Equal(t, int32(0), got.Spec.MinGroups, "%s: MinGroups must be zeroed (else PGD reports MinGroupsNotMet)", name)
	}

	// Other cluster's PGD must be untouched.
	gotOther := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "notmine-h", Namespace: testNS}, gotOther))
	assert.Equal(t, int32(1), gotOther.Spec.Groups, "other cluster's PGD must not be touched")
	assert.Equal(t, int32(1), gotOther.Spec.MinGroups)
}

func TestSuspendPGDs_NoOwnedPGDs(t *testing.T) {
	// No PGDs exist for this cluster — must not error.
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)
	require.NoError(t, h.SuspendPGDs(context.Background(), newRayCluster("myjob", nil)))
}

func TestSuspendPGDs_AlreadySuspendedIsNoop(t *testing.T) {
	// PGD already at Groups=0/MinGroups=0 must not bump generation/resourceVersion.
	scheme := newTestScheme(t)
	head := newOwnedPGD("myjob-h", "myjob", 0, 0)
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(head).Build()
	h := New(c, scheme)

	before := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, before))

	require.NoError(t, h.SuspendPGDs(context.Background(), newRayCluster("myjob", nil)))

	after := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, after))
	assert.Equal(t, before.ResourceVersion, after.ResourceVersion, "no patch should be issued when already at 0/0")
}

func TestSuspendWorkerPGD_SetsGroupsAndMinGroupsToZero(t *testing.T) {
	scheme := newTestScheme(t)
	head := newOwnedPGD("myjob-h", "myjob", 1, 1) // must NOT be touched
	worker := newOwnedPGD("myjob-w-worker", "myjob", 4, 2)
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(head, worker).Build()
	h := New(c, scheme)

	require.NoError(t, h.SuspendWorkerPGD(context.Background(), newRayCluster("myjob", nil), "worker"))

	gotWorker := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-w-worker", Namespace: testNS}, gotWorker))
	assert.Equal(t, int32(0), gotWorker.Spec.Groups)
	assert.Equal(t, int32(0), gotWorker.Spec.MinGroups)

	// Head PGD must be untouched (only the named worker is suspended).
	gotHead := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "myjob-h", Namespace: testNS}, gotHead))
	assert.Equal(t, int32(1), gotHead.Spec.Groups, "head must not be touched by SuspendWorkerPGD")
	assert.Equal(t, int32(1), gotHead.Spec.MinGroups)
}

func TestSuspendWorkerPGD_MissingPGDIsNoop(t *testing.T) {
	// PGD not yet created — must not error.
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)
	require.NoError(t, h.SuspendWorkerPGD(context.Background(), newRayCluster("myjob", nil), "worker"))
}

func TestDeletePGDs_RemovesAllOwned(t *testing.T) {
	scheme := newTestScheme(t)
	head := newOwnedPGD("myjob-h", "myjob", 1, 1)
	worker := newOwnedPGD("myjob-w-worker", "myjob", 4, 2)
	other := newOwnedPGD("notmine-h", "notmine", 1, 1)
	c := clientFake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(head, worker, other).Build()
	h := New(c, scheme)

	require.NoError(t, h.DeletePGDs(context.Background(), newRayCluster("myjob", nil)))

	for _, name := range []string{"myjob-h", "myjob-w-worker"} {
		got := &pgdv1alpha1.PodGroupDeployment{}
		err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: testNS}, got)
		assert.True(t, apierrors.IsNotFound(err), "%s must be deleted, got err=%v", name, err)
	}

	// Other cluster's PGD must remain.
	gotOther := &pgdv1alpha1.PodGroupDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "notmine-h", Namespace: testNS}, gotOther))
}

func TestDeletePGDs_EmptyIsNoop(t *testing.T) {
	scheme := newTestScheme(t)
	c := clientFake.NewClientBuilder().WithScheme(scheme).Build()
	h := New(c, scheme)
	require.NoError(t, h.DeletePGDs(context.Background(), newRayCluster("myjob", nil)))
}
