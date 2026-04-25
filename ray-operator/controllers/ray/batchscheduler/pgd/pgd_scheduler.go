// Package pgd plugs xAI's PodGroupDeployment (PGD) operator into KubeRay's
// BatchScheduler interface. PGD is unusual among batch schedulers because it
// owns the pod lifecycle entirely: the scheduler's PodGroupDeployment CR
// carries the pod template, and the PGD operator materializes pods itself.
//
// To express that, this scheduler implements the extended
// schedulerinterface.PodLifecycleScheduler interface in addition to the base
// BatchScheduler interface. KubeRay type-asserts the configured scheduler to
// PodLifecycleScheduler at every site that would otherwise call r.Create /
// r.Delete on a pod, and delegates the operation to this adapter when the
// assertion succeeds.
//
// The actual logic lives in the controllers/ray/pgd helper package; this file
// is a thin shim that translates BatchScheduler calls into helper calls.
package pgd

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	schedulerinterface "github.com/ray-project/kuberay/ray-operator/controllers/ray/batchscheduler/interface"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/pgd"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

const pluginName = "pgd"

// GetPluginName returns the scheduler plugin name. Match the convention used
// by the other plugins (volcano.GetPluginName(), yunikorn.GetPluginName(), ...)
// so schedulermanager.go's getSchedulerFactory can dispatch on it.
func GetPluginName() string { return pluginName }

// Scheduler is the BatchScheduler / PodLifecycleScheduler adapter for PGD.
// It delegates all real work to a *pgd.Helper.
type Scheduler struct {
	helper *pgd.Helper
}

// Compile-time assertions that we implement both interfaces.
var (
	_ schedulerinterface.BatchScheduler        = (*Scheduler)(nil)
	_ schedulerinterface.PodLifecycleScheduler = (*Scheduler)(nil)
)

// Factory constructs a Scheduler.
type Factory struct{}

// Name implements schedulerinterface.BatchScheduler.
func (s *Scheduler) Name() string { return pluginName }

// DoBatchSchedulingOnSubmission is a no-op for PGD.
//
// Most schedulers create their gang CR (Volcano PodGroup, scheduler-plugins
// PodGroup, ...) here, before pods are created, because they only need
// MinMember + total resources at submission time. PGD is different: the
// PodGroupDeployment CR carries the entire pod template, which KubeRay only
// builds inside createHeadPod / createWorkerPod via buildHeadPod / buildWorkerPod.
// So the PGD adapter creates its CRs lazily from UpsertGangForHead /
// UpsertGangForWorker (called by KubeRay's reconcilePods), not from this hook.
func (s *Scheduler) DoBatchSchedulingOnSubmission(_ context.Context, _ metav1.Object) error {
	return nil
}

// AddMetadataToChildResource is a no-op for PGD.
//
// Other schedulers stamp Spec.SchedulerName + scheduler-specific labels on
// every pod here so a third-party scheduler binds them. PGD doesn't bind via
// SchedulerName -- it sets Pod.Spec.NodeName directly when materializing pods
// from its template. Pod metadata (labels/annotations) is taken from the PGD
// CR's Spec.Deployment.Template, which we populate inside UpsertGangForHead /
// UpsertGangForWorker.
func (s *Scheduler) AddMetadataToChildResource(_ context.Context, _ metav1.Object, _ metav1.Object, _ string) {
}

// CleanupOnCompletion deletes all gangs owned by the RayJob's RayCluster when
// the RayJob reaches terminal state, so PGD's queue accounting releases
// resources promptly. This mirrors what Volcano's CleanupOnCompletion does
// for its PodGroup CR.
//
// Returns (false, nil) for non-RayJob objects (no-op for RayCluster directly,
// since cascade GC handles those when the RayCluster is deleted).
func (s *Scheduler) CleanupOnCompletion(ctx context.Context, object metav1.Object) (bool, error) {
	rayJob, ok := object.(*rayv1.RayJob)
	if !ok {
		return false, nil
	}
	// If the RayJob hasn't been associated with a RayCluster yet, there's nothing to clean.
	if rayJob.Status.RayClusterName == "" {
		return false, nil
	}
	cluster := &rayv1.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rayJob.Status.RayClusterName,
			Namespace: rayJob.Namespace,
		},
	}
	if err := s.helper.DeletePGDs(ctx, cluster); err != nil {
		return false, fmt.Errorf("PGD CleanupOnCompletion delete: %w", err)
	}
	return true, nil
}

// UpsertGangForHead implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) UpsertGangForHead(ctx context.Context, instance *rayv1.RayCluster, headPod *corev1.Pod) error {
	return s.helper.UpsertPGDForHead(ctx, instance, headPod)
}

// UpsertGangForWorker implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) UpsertGangForWorker(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec, workerPod *corev1.Pod) error {
	return s.helper.UpsertPGDForGroup(ctx, instance, workerSpec, workerPod)
}

// SuspendAllGangs implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) SuspendAllGangs(ctx context.Context, instance *rayv1.RayCluster) error {
	return s.helper.SuspendPGDs(ctx, instance)
}

// SuspendWorkerGang implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) SuspendWorkerGang(ctx context.Context, instance *rayv1.RayCluster, groupName string) error {
	return s.helper.SuspendWorkerPGD(ctx, instance, groupName)
}

// DeleteAllGangs implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) DeleteAllGangs(ctx context.Context, instance *rayv1.RayCluster) error {
	return s.helper.DeletePGDs(ctx, instance)
}

// MarkAndScaleDownGang implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) MarkAndScaleDownGang(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec) error {
	return s.helper.MarkAndScaleDown(ctx, instance, workerSpec)
}

// QueueStatusReason implements schedulerinterface.PodLifecycleScheduler.
func (s *Scheduler) QueueStatusReason(ctx context.Context, instance *rayv1.RayCluster) (string, error) {
	return s.helper.QueueStatusReason(ctx, instance)
}

// New constructs the Scheduler. Implements schedulerinterface.BatchSchedulerFactory.
func (f *Factory) New(_ context.Context, _ *rest.Config, cli client.Client) (schedulerinterface.BatchScheduler, error) {
	if err := pgdv1alpha1.AddToScheme(cli.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to add PGD types to scheme: %w", err)
	}
	return &Scheduler{helper: pgd.New(cli, cli.Scheme())}, nil
}

// AddToScheme registers the PGD types with the global runtime scheme.
// Implements schedulerinterface.BatchSchedulerFactory.
func (f *Factory) AddToScheme(scheme *runtime.Scheme) {
	utilruntime.Must(pgdv1alpha1.AddToScheme(scheme))
}

// ConfigureReconciler wires the RayCluster reconciler watches that PGD mode
// requires:
//
//   - Owns(PodGroupDeployment) so the reconciler runs on PGD spec/status changes.
//   - Watches(Pod, MapPodToRayCluster) so PGD-owned pods (which are NOT owned
//     by the RayCluster directly) still trigger reconciles via the
//     ray.io/cluster label on each pod.
//
// Implements schedulerinterface.BatchSchedulerFactory.
func (f *Factory) ConfigureReconciler(b *builder.Builder) *builder.Builder {
	return b.
		Owns(&pgdv1alpha1.PodGroupDeployment{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(pgd.MapPodToRayCluster))
}
