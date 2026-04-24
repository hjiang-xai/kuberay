// Package pgd contains KubeRay's PodGroupDeployment-mode integration.
//
// In PGD mode (gated by the `ray.io/pgd-mode` annotation on a RayCluster),
// the operator emits a PodGroupDeployment CR per pod group instead of creating
// pods directly. The PGD operator then materializes the actual pods with
// queue accounting, gang scheduling, and priority-based preemption.
//
// See ray-operator/third_party/pgd/v1alpha1/README.md for the vendored types.
package pgd

// PGD-side label keys (mirrored from
// github.com/xai-org/xai/cluster/podgroup-operator/internal/consts).
// These are intentionally kept in sync with PGD's internal constants;
// PGD reads them off pods/PGDs via these exact strings.
const (
	// DeleteNextLabelKey marks a pod as "delete this group first" when PGD scales
	// down. PGD's heap (`groups.go:Less`) sorts groups carrying this label
	// to the top of the eviction order. We use this for autoscaler-driven
	// scale-down so PGD evicts exactly the pods Ray's autoscaler picked.
	DeleteNextLabelKey = "podgroup-operator.x.ai/delete-next"
)

// KubeRay-side annotations that gate PGD-mode behavior. Set on the RayCluster.
const (
	// PGDModeAnnotation enables PGD-mode for a RayCluster when set to "true".
	PGDModeAnnotation = "ray.io/pgd-mode"

	// PGDQueueAnnotation specifies the PGD Queue name for this RayCluster's pods.
	PGDQueueAnnotation = "ray.io/pgd-queue"

	// PGDPriorityAnnotation specifies the PGD priority (int32) for this RayCluster.
	PGDPriorityAnnotation = "ray.io/pgd-priority"

	// PGDGroupByKeyAnnotation, when set, makes all PGDs created for this
	// RayCluster share a GroupBy key so PGD treats them as one atomic
	// scheduling unit (head + workers schedule together or not at all).
	PGDGroupByKeyAnnotation = "ray.io/pgd-group-by-key"
)
