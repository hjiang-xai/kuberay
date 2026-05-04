// Package pgd contains KubeRay's PodGroupDeployment integration helpers.
//
// PGD is wired into KubeRay as a BatchScheduler plugin. To activate it, run
// the operator with `--batch-scheduler=pgd`. When activated, KubeRay emits a
// PodGroupDeployment CR per pod group instead of creating pods directly,
// and the PGD operator materializes the actual pods with queue accounting,
// gang scheduling, and priority-based preemption.
//
// This package owns the per-pod-group helpers (UpsertPGDForHead /
// UpsertPGDForGroup / SuspendPGDs / DeletePGDs / MarkAndScaleDown / ...).
// The adapter at controllers/ray/batchscheduler/pgd wraps these methods to
// satisfy schedulerinterface.PodLifecycleScheduler.
//
// See ray-operator/third_party/pgd/v1alpha1/README.md for the vendored types.
package pgd

// PGD-side label keys (mirrored from
// github.com/xai-org/xai/cluster/podgroup-operator/internal/consts).
//
// These strings are part of PGD's stable wire contract: PGD reads them off
// pods/PGDs verbatim. If PGD ever renames one of these keys, autoscaler-driven
// scale-down silently breaks here -- the operator labels the autoscaler's
// chosen victims with the wrong key, PGD picks different pods, and
// safe_to_scale fails forever.
//
// Drift detection: pgd_drift_test.go pins each value with an explicit literal
// assertion so any rename in this file shows up as a test diff.
const (
	// DeleteNextLabelKey marks a pod as "delete this group first" when PGD scales
	// down. PGD's heap (`groups.go:Less`) sorts groups carrying this label
	// to the top of the eviction order. We use this for autoscaler-driven
	// scale-down so PGD evicts exactly the pods Ray's autoscaler picked.
	DeleteNextLabelKey = "podgroup-operator.x.ai/delete-next"
)

// KubeRay-side annotations that configure PGD scheduling per RayCluster.
// (Activation of PGD itself is operator-wide via `--batch-scheduler=pgd`.)
// Set on the RayCluster.
//
// Namespace: `dataplatform.x.ai/` — chosen specifically to avoid collision
// with upstream Ray's `ray.io/` namespace, which we don't own. If upstream
// KubeRay ever adds a `ray.io/pgd-*` annotation we don't get a name clash.
const (
	// PGDQueueAnnotation specifies the PGD Queue name for this RayCluster's pods.
	PGDQueueAnnotation = "dataplatform.x.ai/ray-pgd-queue"

	// PGDPriorityAnnotation specifies the PGD priority (int32) for this RayCluster.
	PGDPriorityAnnotation = "dataplatform.x.ai/ray-pgd-priority"

	// PGDGroupByKeyAnnotation, when set, makes all PGDs created for this
	// RayCluster share a GroupBy key so PGD treats them as one atomic
	// scheduling unit (head + workers schedule together or not at all).
	PGDGroupByKeyAnnotation = "dataplatform.x.ai/ray-pgd-group-by-key"
)
