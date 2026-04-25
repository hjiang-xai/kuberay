package pgd

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

// Helper wraps the K8s client and scheme needed to manage PodGroupDeployment
// CRs on behalf of a RayCluster reconciler. Construct one per reconciler.
type Helper struct {
	client.Client
	Scheme *runtime.Scheme
}

// New constructs a PGD Helper.
func New(c client.Client, scheme *runtime.Scheme) *Helper {
	return &Helper{Client: c, Scheme: scheme}
}

// UpsertPGDForHead creates or updates the head-group PodGroupDeployment for the
// given RayCluster. The head pod template is wrapped into pgd.Spec.Deployment.Template;
// PGD will materialize the actual head pod with nodeName pre-set.
//
// Group/GroupSize layout for the head:
//   - Groups    = 1
//   - GroupSize = 1
//   - MinGroups = 1                                          (head must always be present)
//   - RequiredTopologyKey = "kubernetes.io/hostname"         (single-node group)
//   - Movable  = false                                       (concern #13: never let defrag move us)
func (h *Helper) UpsertPGDForHead(ctx context.Context, instance *rayv1.RayCluster, headPod *corev1.Pod) error {
	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HeadPGDName(instance.Name),
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, h.Client, pgd, func() error {
		applyCommonMeta(pgd, instance, "" /* no group name for head */)
		pgd.Spec.Groups = 1
		pgd.Spec.GroupSize = 1
		pgd.Spec.MinGroups = 1
		pgd.Spec.RequiredTopologyKey = "kubernetes.io/hostname"
		pgd.Spec.Movable = false
		pgd.Spec.Priority = PriorityFor(instance)
		pgd.Spec.Queue = QueueFor(instance)
		applyGroupBy(pgd, instance)
		applyPodTemplateToPGD(pgd, headPod)
		return controllerutil.SetControllerReference(instance, pgd, h.Scheme)
	})
	if err != nil {
		return fmt.Errorf("upsert head PGD %s/%s: %w", pgd.Namespace, pgd.Name, err)
	}
	logf.FromContext(ctx).Info("PGD upserted (head)", "pgd", pgd.Name, "op", op)
	return nil
}

// UpsertPGDForGroup creates or updates a worker-group PodGroupDeployment for
// the given RayCluster + WorkerGroupSpec.
//
// Group/GroupSize layout for a worker group:
//   - Groups    = workerSpec.Replicas              (autoscaler-adjusted desired count)
//   - GroupSize = max(1, workerSpec.NumOfHosts)    (multi-host => one group is N pods)
//   - MinGroups = workerSpec.MinReplicas           (gang-schedule at least this many)
//   - Movable  = false                             (concern #13)
//
// For NumOfHosts == 1, RequiredTopologyKey is left empty (multi-node spread). For
// NumOfHosts > 1, the gang lands on one node when "kubernetes.io/hostname" is set;
// callers can override via the per-cluster topology annotation if a different
// topology key is needed.
func (h *Helper) UpsertPGDForGroup(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec, workerPod *corev1.Pod) error {
	pgd := &pgdv1alpha1.PodGroupDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkerPGDName(instance.Name, workerSpec.GroupName),
			Namespace: instance.Namespace,
		},
	}
	desiredGroups := int32(0)
	if workerSpec.Replicas != nil {
		desiredGroups = *workerSpec.Replicas
	}
	groupSize := int32(1)
	if workerSpec.NumOfHosts > 1 {
		groupSize = workerSpec.NumOfHosts
	}

	op, err := controllerutil.CreateOrPatch(ctx, h.Client, pgd, func() error {
		applyCommonMeta(pgd, instance, workerSpec.GroupName)
		pgd.Spec.Groups = desiredGroups
		pgd.Spec.GroupSize = groupSize
		if workerSpec.MinReplicas != nil {
			pgd.Spec.MinGroups = *workerSpec.MinReplicas
		}
		pgd.Spec.Movable = false
		pgd.Spec.Priority = PriorityFor(instance)
		pgd.Spec.Queue = QueueFor(instance)
		if groupSize > 1 {
			// Default to single-node gang when NumOfHosts > 1.
			pgd.Spec.RequiredTopologyKey = "kubernetes.io/hostname"
		}
		applyGroupBy(pgd, instance)
		applyPodTemplateToPGD(pgd, workerPod)
		return controllerutil.SetControllerReference(instance, pgd, h.Scheme)
	})
	if err != nil {
		return fmt.Errorf("upsert worker PGD %s/%s: %w", pgd.Namespace, pgd.Name, err)
	}
	logf.FromContext(ctx).Info("PGD upserted (worker)", "pgd", pgd.Name, "group", workerSpec.GroupName, "groups", desiredGroups, "groupSize", groupSize, "op", op)
	return nil
}

// MarkAndScaleDown handles autoscaler-driven scale-down for one worker group in
// PGD mode. The Ray autoscaler patches the RayCluster spec with:
//
//   workerGroupSpecs[i].replicas = N - K
//   workerGroupSpecs[i].scaleStrategy.workersToDelete = [<podName>, ...]
//
// In upstream KubeRay, the operator calls r.Delete(pod) directly. In PGD mode
// PGD owns the pods (with finalizers), so a direct Delete causes thrash:
// PGD's missingCount rises and it recreates the pod immediately.
//
// Instead we use PGD's first-class delete-next mechanism:
//
//   1. Label each pod the autoscaler picked with `podgroup-operator.x.ai/delete-next=""`.
//      PGD's heap (`groups.go:Less`) sorts these to the top of the eviction order;
//      `handleExcessGroups` in `reconcile.go` pops them first when excess > 0.
//
//   2. Patch pgd.Spec.Groups = workerSpec.Replicas to create the excess. PGD's
//      next reconcile cycle deterministically deletes the labeled pods.
//
// Cache-coherency safety (PGD's informer is independent of ours):
//   - both updates visible: PGD evicts the right victims                      ✓
//   - only labels visible:  excess=0, no-op cycle, retries when spec arrives  ✓
//   - only spec visible:    PGD picks wrong victims once, autoscaler's
//                           safe_to_scale check fails, retries on next cycle  ✓
//
// missingCount = Spec.Groups - terminal - groups.Len() stays balanced because
// we never call Delete() ourselves.
func (h *Helper) MarkAndScaleDown(ctx context.Context, instance *rayv1.RayCluster, workerSpec *rayv1.WorkerGroupSpec) error {
	log := logf.FromContext(ctx)

	// 1. Label each scale-down victim.
	for _, podName := range workerSpec.ScaleStrategy.WorkersToDelete {
		pod := &corev1.Pod{}
		key := types.NamespacedName{Namespace: instance.Namespace, Name: podName}
		if err := h.Get(ctx, key, pod); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get scale-down victim %s: %w", podName, err)
		}
		if _, already := pod.Labels[DeleteNextLabelKey]; already {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[DeleteNextLabelKey] = ""
		if err := h.Patch(ctx, pod, patch); err != nil {
			return fmt.Errorf("label scale-down victim %s with delete-next: %w", podName, err)
		}
		log.Info("PGD scale-down victim labeled", "pod", podName)
	}

	// 2. Sync pgd.Spec.Groups to the autoscaler-adjusted Replicas.
	pgdName := WorkerPGDName(instance.Name, workerSpec.GroupName)
	pgd := &pgdv1alpha1.PodGroupDeployment{}
	if err := h.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: pgdName}, pgd); err != nil {
		if apierrors.IsNotFound(err) {
			// Nothing to scale yet; UpsertPGDForGroup will create with the right Groups.
			return nil
		}
		return fmt.Errorf("get worker PGD %s: %w", pgdName, err)
	}
	desiredGroups := int32(0)
	if workerSpec.Replicas != nil {
		desiredGroups = *workerSpec.Replicas
	}
	if pgd.Spec.Groups == desiredGroups {
		return nil
	}
	patch := client.MergeFrom(pgd.DeepCopy())
	pgd.Spec.Groups = desiredGroups
	if err := h.Patch(ctx, pgd, patch); err != nil {
		return fmt.Errorf("patch PGD %s Spec.Groups: %w", pgdName, err)
	}
	log.Info("PGD scaled", "pgd", pgdName, "groups", desiredGroups)
	return nil
}

// applyCommonMeta sets labels/annotations shared by all PGDs we create for a
// RayCluster. groupName is "" for the head and the WorkerGroupSpec.GroupName
// for worker groups.
func applyCommonMeta(pgd *pgdv1alpha1.PodGroupDeployment, instance *rayv1.RayCluster, groupName string) {
	if pgd.Labels == nil {
		pgd.Labels = map[string]string{}
	}
	pgd.Labels[utils.RayClusterLabelKey] = instance.Name
	if groupName != "" {
		pgd.Labels[utils.RayNodeGroupLabelKey] = groupName
	} else {
		pgd.Labels[utils.RayNodeTypeLabelKey] = string(rayv1.HeadNode)
	}
}

// applyGroupBy stamps a GroupBy.Key on the PGD when the user has set the
// dataplatform.x.ai/ray-pgd-group-by-key annotation. PGDs sharing the same key are scheduled
// as one atomic unit (head + workers schedule together or not at all).
func applyGroupBy(pgd *pgdv1alpha1.PodGroupDeployment, instance *rayv1.RayCluster) {
	key := GroupByKeyFor(instance)
	if key == "" {
		pgd.Spec.GroupBy = nil
		return
	}
	pgd.Spec.GroupBy = &pgdv1alpha1.GroupBy{Key: key}
}

// applyPodTemplateToPGD copies the pod's labels, annotations, and spec onto
// pgd.Spec.Deployment.Template. PGD will materialize this template per group,
// adding its own ownerReference, finalizer, and per-pod ordinal/group labels.
//
// We also set Spec.Deployment.Selector to match the RayCluster + (optional)
// group label so PGD's StatefulSetSpec is well-formed.
func applyPodTemplateToPGD(pgd *pgdv1alpha1.PodGroupDeployment, pod *corev1.Pod) {
	// Copy labels (preserve KubeRay's ray.io/* labels so all existing label-based
	// pod listing in the operator continues to work).
	labels := map[string]string{}
	for k, v := range pod.Labels {
		labels[k] = v
	}
	annotations := map[string]string{}
	for k, v := range pod.Annotations {
		annotations[k] = v
	}

	pgd.Spec.Deployment.Template.ObjectMeta.Labels = labels
	pgd.Spec.Deployment.Template.ObjectMeta.Annotations = annotations
	pgd.Spec.Deployment.Template.Spec = *pod.Spec.DeepCopy()

	// Selector for the StatefulSetSpec: any subset of immutable labels works.
	// Use the cluster + node-type/group labels so the selector is stable across
	// reconciles.
	selectorLabels := map[string]string{}
	if v, ok := labels[utils.RayClusterLabelKey]; ok {
		selectorLabels[utils.RayClusterLabelKey] = v
	}
	if v, ok := labels[utils.RayNodeGroupLabelKey]; ok {
		selectorLabels[utils.RayNodeGroupLabelKey] = v
	} else if v, ok := labels[utils.RayNodeTypeLabelKey]; ok {
		selectorLabels[utils.RayNodeTypeLabelKey] = v
	}
	pgd.Spec.Deployment.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels}
}
