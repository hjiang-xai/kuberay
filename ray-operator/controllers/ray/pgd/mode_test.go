package pgd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
)

func rcWithAnnotations(ann map[string]string) *rayv1.RayCluster {
	return &rayv1.RayCluster{ObjectMeta: metav1.ObjectMeta{Annotations: ann}}
}

func TestQueueFor(t *testing.T) {
	tests := []struct {
		name     string
		instance *rayv1.RayCluster
		want     string
	}{
		{name: "nil instance", instance: nil, want: ""},
		{name: "no annotations", instance: &rayv1.RayCluster{}, want: ""},
		{name: "annotation absent", instance: rcWithAnnotations(map[string]string{}), want: ""},
		{name: "annotation set", instance: rcWithAnnotations(map[string]string{PGDQueueAnnotation: "freebie"}), want: "freebie"},
		{name: "annotation empty", instance: rcWithAnnotations(map[string]string{PGDQueueAnnotation: ""}), want: ""},
		{name: "annotation with hyphens", instance: rcWithAnnotations(map[string]string{PGDQueueAnnotation: "training-data"}), want: "training-data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, QueueFor(tt.instance))
		})
	}
}

func TestPriorityFor(t *testing.T) {
	tests := []struct {
		name     string
		instance *rayv1.RayCluster
		want     int32
	}{
		{name: "nil instance", instance: nil, want: 0},
		{name: "no annotations", instance: &rayv1.RayCluster{}, want: 0},
		{name: "annotation absent", instance: rcWithAnnotations(map[string]string{}), want: 0},
		{name: "annotation 0", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "0"}), want: 0},
		{name: "annotation 100", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "100"}), want: 100},
		{name: "annotation negative", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "-50"}), want: -50},
		{name: "annotation max int32", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "2147483647"}), want: 2147483647},
		{name: "annotation overflows int32 -> 0", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "9999999999999"}), want: 0},
		{name: "annotation non-numeric -> 0", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "high"}), want: 0},
		{name: "annotation float -> 0", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: "1.5"}), want: 0},
		{name: "annotation empty -> 0", instance: rcWithAnnotations(map[string]string{PGDPriorityAnnotation: ""}), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PriorityFor(tt.instance))
		})
	}
}

func TestGroupByKeyFor(t *testing.T) {
	tests := []struct {
		name     string
		instance *rayv1.RayCluster
		want     string
	}{
		{name: "nil instance", instance: nil, want: ""},
		{name: "no annotations", instance: &rayv1.RayCluster{}, want: ""},
		{name: "annotation absent", instance: rcWithAnnotations(map[string]string{}), want: ""},
		{name: "annotation set", instance: rcWithAnnotations(map[string]string{PGDGroupByKeyAnnotation: "abc-123"}), want: "abc-123"},
		{name: "annotation empty", instance: rcWithAnnotations(map[string]string{PGDGroupByKeyAnnotation: ""}), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GroupByKeyFor(tt.instance))
		})
	}
}
