package pgd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	pgdv1alpha1 "github.com/ray-project/kuberay/ray-operator/third_party/pgd/v1alpha1"
)

// QueueStatusReason inspects all PGDs owned by the given RayCluster and returns
// a one-line summary suitable for `RayCluster.Status.Reason` while pods are not
// yet materialized. Returns "" if every PGD is fully scheduled (in which case
// the caller should leave the existing Reason alone).
//
// This addresses concern #1 (queue visibility): KubeRay's existing empty
// `Status.State` already represents "pods not running yet"; this just attaches
// a human-readable reason so users can see WHY without having to
// `kubectl get pgd`.
func (h *Helper) QueueStatusReason(ctx context.Context, instance *rayv1.RayCluster) (string, error) {
	pgds := &pgdv1alpha1.PodGroupDeploymentList{}
	sel := labels.SelectorFromSet(labels.Set{utils.RayClusterLabelKey: instance.Name})
	if err := h.List(ctx, pgds, &client.ListOptions{Namespace: instance.Namespace, LabelSelector: sel}); err != nil {
		return "", fmt.Errorf("list PGDs for RayCluster %s/%s: %w", instance.Namespace, instance.Name, err)
	}
	if len(pgds.Items) == 0 {
		return "", nil
	}

	// Sort by name for stable output (kubectl describe diffs don't churn).
	sort.Slice(pgds.Items, func(i, j int) bool { return pgds.Items[i].Name < pgds.Items[j].Name })

	var pending []string
	allReady := true
	for i := range pgds.Items {
		p := &pgds.Items[i]
		applied := p.Status.Summary.AllocatedGroups
		desired := p.Spec.Groups
		if applied < desired {
			allReady = false
			queue := p.Spec.Queue
			if queue == "" {
				queue = "<none>"
			}
			pending = append(pending, fmt.Sprintf("%s=%d/%d (queue=%s)", p.Name, applied, desired, queue))
		}
	}
	if allReady {
		return "", nil
	}
	return "PGD queued: " + strings.Join(pending, ", "), nil
}
