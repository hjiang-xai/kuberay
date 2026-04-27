package pgd

import (
	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
)

// QueueFor returns the PGD queue name set on the RayCluster, or "" if unset.
func QueueFor(instance *rayv1.RayCluster) string {
	if instance == nil || instance.Annotations == nil {
		return ""
	}
	return instance.Annotations[PGDQueueAnnotation]
}

// PriorityFor returns the PGD priority set on the RayCluster (parsed from the
// annotation as int32), or 0 if unset/invalid.
func PriorityFor(instance *rayv1.RayCluster) int32 {
	if instance == nil || instance.Annotations == nil {
		return 0
	}
	v, ok := instance.Annotations[PGDPriorityAnnotation]
	if !ok {
		return 0
	}
	n, err := parseInt32(v)
	if err != nil {
		return 0
	}
	return n
}

// GroupByKeyFor returns the PGD GroupBy key set on the RayCluster, or "" if unset.
// When set, all PGDs created for this RayCluster share this key so PGD schedules
// them as one atomic unit.
func GroupByKeyFor(instance *rayv1.RayCluster) string {
	if instance == nil || instance.Annotations == nil {
		return ""
	}
	return instance.Annotations[PGDGroupByKeyAnnotation]
}
