package pgd

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
)

// MapPodToRayCluster is a controller-runtime mapping function that finds the
// RayCluster owning the given Pod by reading the ray.io/cluster label.
//
// Background: in PGD mode, pods are owned by a PodGroupDeployment, not the
// RayCluster directly (PGD's reconciler sets the controller owner ref via
// `controllerutil.SetControllerReference(pgd, pod, ...)`). controller-runtime's
// `Owns(&corev1.Pod{})` walks only one level of ownerReferences, so it will
// never enqueue our reconciler for these pod events. We replace it with a
// `Watches(&corev1.Pod{}, EnqueueRequestsFromMapFunc(MapPodToRayCluster))`
// which finds the cluster by label.
//
// Mirrors the pattern used by xAI's experiment-operator at
// cluster/experiment-operator/internal/controller/experiment_controller.go.
func MapPodToRayCluster(_ context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok || pod.Labels == nil {
		return nil
	}
	clusterName, ok := pod.Labels[utils.RayClusterLabelKey]
	if !ok || clusterName == "" {
		return nil
	}
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: pod.Namespace}},
	}
}
