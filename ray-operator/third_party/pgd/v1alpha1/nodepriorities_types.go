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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodePrioritiesSpec defines the desired state of NodePriorities
type NodePrioritiesSpec struct{}

// NodePriorityInfo contains priority information for a node based on its scheduling group.
// All nodes in the same group share the same priority fields, derived from the
// most important pod across the entire group.
type NodePriorityInfo struct {
	// NodeName is the name of the node.
	NodeName string `json:"nodeName"`
	// GroupSize is the number of nodes in this node's scheduling group.
	// Empty nodes (no non-daemonset pods) have GroupSize 0.
	// +optional
	GroupSize int32 `json:"groupSize,omitempty"`
	// QueuePriority is the queue priority of the group's most important pod's PGD.
	// +optional
	QueuePriority int32 `json:"queuePriority,omitempty"`
	// PGDPriority is the PGD priority of the group's most important pod's PGD.
	// +optional
	PGDPriority int32 `json:"pgdPriority,omitempty"`
	// PGDModifiedAt is the modified timestamp of the group's most important pod's PGD.
	// +optional
	PGDModifiedAt *metav1.Time `json:"pgdModifiedAt,omitempty"`
	// PodPriority is the pod spec priority of the group's most important pod.
	// +optional
	PodPriority int32 `json:"podPriority,omitempty"`
}

// NodePrioritiesStatus defines the observed state of NodePriorities.
type NodePrioritiesStatus struct {
	// Nodes in ascending priority based on the pods they are running.
	Nodes []NodePriorityInfo `json:"nodes"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// NodePriorities is the Schema for the nodepriorities API
type NodePriorities struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// status defines the observed state of NodePriorities
	// +optional
	Status NodePrioritiesStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// NodePrioritiesList contains a list of NodePriorities
type NodePrioritiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NodePriorities `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodePriorities{}, &NodePrioritiesList{})
}
