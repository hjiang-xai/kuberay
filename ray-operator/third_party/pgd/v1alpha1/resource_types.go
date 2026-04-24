/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeShape string

const (
	NodeShapeH100H200 NodeShape = "H100H200"
	NodeShapeGB200    NodeShape = "GB200"
	NodeShapeGB300    NodeShape = "GB300"
	NodeShapeMI300    NodeShape = "MI300"
)

// ResourceStatus defines the observed scheduling estimation for one queue.
// All data is derived from the scheduler snapshot (actual pods on nodes).
type ResourceStatus struct {
	// LastUpdated is the timestamp of the last estimation cycle.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// NodeShape is the shape of the node on which the workload could be scheduled on.
	NodeShape NodeShape `json:"nodeShape,omitempty"`
	// groupSize defines the minsize of one individual podGroup
	// +kubebuilder:validation:Minimum=1
	GroupSize int32 `json:"groupSize"`
	// Queue specifies the name of the Queue resource to associate with this PodGroupDeployment.
	// Queues provide both a priority level (which affects scheduling order) and GPU capacity limits.
	// The scheduler will check if the queue has sufficient GPU capacity before scheduling groups.
	// +optional
	Queue string `json:"queue,omitempty"`
	// MaxSchedulableGroups is the number of groups schedulable right now on free nodes.
	MaxSchedulableGroups int32 `json:"maxSchedulableGroups"`
	// PreemptableGroups is the number of additional groups obtainable by
	// preempting lower-priority workloads. The total with preemption is
	// MaxSchedulableGroups + PreemptableGroups.
	PreemptableGroups int32 `json:"preemptableGroups"`
	// GroupedNodes is the ordered list of nodes in groups on which a workload of the
	// given size, shape and queue would be scheduled on.
	GroupedNodes [][]string `json:"groupedNodes,omitempty"`
	// UncappedMaxSchedulable is the max schedulable groups (including via preemption)
	// ignoring queue GPU limits. Only set when the queue limit is the bottleneck
	// (UncappedMaxSchedulable > MaxSchedulableGroups).
	// +optional
	UncappedMaxSchedulable int32 `json:"uncappedMaxSchedulable,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=res
// +kubebuilder:printcolumn:name="Last Updated",type=date,JSONPath=`.status.lastUpdated`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Resource is a cluster-scoped resource (one per queue) that publishes
// scheduling estimations after every scheduling cycle. It answers
// "what if I submitted a PGD of size X into this queue?"
type Resource struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Status ResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceList contains a list of Resource.
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
