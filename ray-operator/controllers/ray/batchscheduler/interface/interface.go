package schedulerinterface

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
)

// BatchScheduler manages submitting RayCluster pods to a third-party scheduler.
type BatchScheduler interface {
	// Name corresponds to the schedulerName in Kubernetes:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/
	Name() string

	// DoBatchSchedulingOnSubmission handles submitting the RayCluster/RayJob to the batch scheduler on creation / update
	// For most batch schedulers, this results in the creation of a PodGroup.
	DoBatchSchedulingOnSubmission(ctx context.Context, object metav1.Object) error

	// AddMetadataToChildResource enriches the child resource (batchv1.Job, rayv1.RayCluster) with metadata necessary to tie it to the scheduler.
	// For example, setting labels for queues / priority, and setting schedulerName.
	AddMetadataToChildResource(ctx context.Context, parent metav1.Object, child metav1.Object, groupName string)

	// CleanupOnCompletion handles cleanup when the RayJob reaches terminal state (Complete/Failed).
	// For batch schedulers like Volcano, this deletes the PodGroup to release queue resources.
	// This is a no-op for schedulers that don't need cleanup.
	// Returns (didCleanup, error) where didCleanup indicates whether actual cleanup was performed.
	CleanupOnCompletion(ctx context.Context, object metav1.Object) (didCleanup bool, err error)
}

// PodLifecycleScheduler is the extended interface implemented by BatchSchedulers
// that own the RayCluster's pod lifecycle entirely, rather than just gating
// scheduling decisions on pods that KubeRay creates. Most schedulers in this
// repo (Volcano, YuniKorn, Kai, scheduler-plugins) do NOT implement it: they
// let KubeRay create pods via r.Create(pod) and bind them via Spec.SchedulerName.
//
// A scheduler that DOES implement this interface signals to KubeRay that it
// owns a "gang" abstraction (a CR that carries the pod template and produces
// the actual pods itself). KubeRay type-asserts the configured scheduler to
// this interface at every site that would otherwise call r.Create(pod) or
// r.Delete(pod), and delegates the operation to the scheduler when the
// assertion succeeds. Pods created via this path are owned by the scheduler's
// gang CR, not by the RayCluster directly.
//
// To add a new pod-lifecycle scheduler:
//  1. Make its scheduler type implement these methods alongside BatchScheduler.
//  2. Register it in batchscheduler/schedulermanager.go like any other scheduler.
//  3. Add a Watches(Pod, ...) on the cluster label in ConfigureReconciler so
//     RayCluster reconciles still fire for pod events when pods aren't directly
//     owned by the RayCluster.
type PodLifecycleScheduler interface {
	BatchScheduler

	// UpsertGangForHead is the head-pod-creation analog of r.Create(headPod).
	// The scheduler creates or patches its gang CR carrying the head pod's
	// template; the scheduler's own controller materializes the pod from
	// that template.
	UpsertGangForHead(ctx context.Context, instance *rayv1.RayCluster, headPod *corev1.Pod) error

	// UpsertGangForWorker is the worker-pod-creation analog of r.Create(workerPod).
	// Per-pod creates from KubeRay are coalesced into a single gang CR per
	// worker group; the implementation must be idempotent across multiple
	// calls within a single reconcile cycle.
	UpsertGangForWorker(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec, workerPod *corev1.Pod) error

	// SuspendAllGangs drains every gang owned by this RayCluster (e.g. by
	// scaling each gang's desired pod count to 0) so the scheduler removes
	// all pods. KubeRay calls this from the cluster-wide Suspend code path
	// and the GCS-FT Redis cleanup path during RayCluster deletion.
	SuspendAllGangs(ctx context.Context, instance *rayv1.RayCluster) error

	// SuspendWorkerGang drains a single worker group's gang. KubeRay calls
	// this from the per-worker-group Suspend code path.
	SuspendWorkerGang(ctx context.Context, instance *rayv1.RayCluster, groupName string) error

	// DeleteAllGangs removes every gang CR owned by this RayCluster so the
	// scheduler tears down all pods via cascade GC. KubeRay calls this from
	// the Recreate-strategy upgrade code path; the next reconcile recreates
	// fresh gangs from the updated pod template.
	DeleteAllGangs(ctx context.Context, instance *rayv1.RayCluster) error

	// MarkAndScaleDownGang handles autoscaler-driven scale-down for one worker
	// group. The Ray autoscaler patches the cluster spec with a smaller
	// Replicas + a list of WorkersToDelete. Implementations should mark the
	// listed pods as preferred eviction victims AND adjust the gang's desired
	// pod count, in a way that is observable by the scheduler so it deletes
	// the right pods first.
	MarkAndScaleDownGang(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec) error

	// QueueStatusReason returns a human-readable summary of any gangs currently
	// queued (not yet admitted) for this RayCluster, suitable for surfacing in
	// RayCluster.Status.Reason. Returns "" when every gang is fully admitted.
	QueueStatusReason(ctx context.Context, instance *rayv1.RayCluster) (string, error)
}

// BatchSchedulerFactory handles initial setup of the scheduler plugin by registering the
// necessary callbacks with the operator, and the creation of the BatchScheduler itself.
type BatchSchedulerFactory interface {
	// New creates a new BatchScheduler for the scheduler plugin.
	New(ctx context.Context, config *rest.Config, cli client.Client) (BatchScheduler, error)

	// AddToScheme adds the types in this scheduler to the given scheme (runs during init).
	AddToScheme(scheme *runtime.Scheme)

	// ConfigureReconciler configures the RayCluster Reconciler in the process of being built by
	// adding watches for its scheduler-specific custom resource types, and any other needed setup.
	ConfigureReconciler(b *builder.Builder) *builder.Builder
}

type DefaultBatchScheduler struct{}

type DefaultBatchSchedulerFactory struct{}

func GetDefaultPluginName() string {
	return "default"
}

func (d *DefaultBatchScheduler) Name() string {
	return GetDefaultPluginName()
}

func (d *DefaultBatchScheduler) DoBatchSchedulingOnSubmission(_ context.Context, _ metav1.Object) error {
	return nil
}

func (d *DefaultBatchScheduler) AddMetadataToChildResource(_ context.Context, _ metav1.Object, _ metav1.Object, _ string) {
}

func (d *DefaultBatchScheduler) CleanupOnCompletion(_ context.Context, _ metav1.Object) (bool, error) {
	return false, nil
}

func (df *DefaultBatchSchedulerFactory) New(_ context.Context, _ *rest.Config, _ client.Client) (BatchScheduler, error) {
	return &DefaultBatchScheduler{}, nil
}

func (df *DefaultBatchSchedulerFactory) AddToScheme(_ *runtime.Scheme) {
}

func (df *DefaultBatchSchedulerFactory) ConfigureReconciler(b *builder.Builder) *builder.Builder {
	return b
}
