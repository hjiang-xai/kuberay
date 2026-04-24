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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QueueSpec defines the desired state of Queue
type QueueSpec struct {
	// GPUs is the maximum number of GPUs that can be used by workloads in this queue.
	// If nil, the queue does not limit GPU usage.
	// +optional
	GPUs *int32 `json:"gpus,omitempty"`

	// CPUs is the maximum CPU capacity for workloads in this queue.
	// Accepts standard Kubernetes quantity values (e.g. "8", "500m").
	// If nil, the queue does not limit CPU usage.
	// +optional
	CPUs *resource.Quantity `json:"cpus,omitempty"`

	// Memory is the maximum memory capacity for workloads in this queue.
	// Accepts standard Kubernetes quantity values (e.g. "1Gi", "512Mi").
	// If nil, the queue does not limit memory usage.
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`

	Priority int32 `json:"priority,omitempty"`

	// Owner is the user name of the direct responsible person for this queue.
	Owner string `json:"owner,omitempty"`
}

// QueueStatus defines the observed state of Queue.
type QueueStatus struct {
	// Allocated is the amount of GPUs that have a current allocation in this queue.
	Allocated int64 `json:"allocated,omitempty"`
	// Queued is the amount of GPUs that are currently waiting to get an allocation in this queue.
	Queued int64 `json:"queued,omitempty"`

	// AllocatedCPUs is the CPU capacity currently allocated in this queue.
	// +optional
	AllocatedCPUs *resource.Quantity `json:"allocatedCPUs,omitempty"`
	// QueuedCPUs is the CPU capacity waiting for allocation in this queue.
	// +optional
	QueuedCPUs *resource.Quantity `json:"queuedCPUs,omitempty"`

	// AllocatedMemory is the memory capacity currently allocated in this queue.
	// +optional
	AllocatedMemory *resource.Quantity `json:"allocatedMemory,omitempty"`
	// QueuedMemory is the memory capacity waiting for allocation in this queue.
	// +optional
	QueuedMemory *resource.Quantity `json:"queuedMemory,omitempty"`

	// Workloads is a list of workloads that are currently requesting resources in this queue.
	Workloads []Workload `json:"workloads,omitempty"`
}

type Workload struct {
	Namespace       string             `json:"namespace"`
	Name            string             `json:"name"`
	Allocated       int64              `json:"allocated,omitempty"`
	Queued          int64              `json:"queued,omitempty"`
	AllocatedCPUs   *resource.Quantity `json:"allocatedCPUs,omitempty"`
	QueuedCPUs      *resource.Quantity `json:"queuedCPUs,omitempty"`
	AllocatedMemory *resource.Quantity `json:"allocatedMemory,omitempty"`
	QueuedMemory    *resource.Quantity `json:"queuedMemory,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="GPUs",type=integer,JSONPath=`.spec.gpus`,description="The maximum number of GPUs"
// +kubebuilder:printcolumn:name="CPUs",type=string,JSONPath=`.spec.cpus`,description="The maximum CPU capacity",priority=1
// +kubebuilder:printcolumn:name="Memory",type=string,JSONPath=`.spec.memory`,description="The maximum memory capacity",priority=1
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`,description="The GPUs currently allocated"
// +kubebuilder:printcolumn:name="Queued",type=integer,JSONPath=`.status.queued`,description="The GPUs currently queued"
// +kubebuilder:printcolumn:name="Alloc CPUs",type=string,JSONPath=`.status.allocatedCPUs`,description="CPU allocated",priority=1
// +kubebuilder:printcolumn:name="Alloc Mem",type=string,JSONPath=`.status.allocatedMemory`,description="Memory allocated",priority=1
// +kubebuilder:printcolumn:name="Priority",type=integer,JSONPath=`.spec.priority`,description="The priority of the queue"

// Queue is the Schema for the queues API
type Queue struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Queue
	// +required
	Spec QueueSpec `json:"spec"`

	// status defines the observed state of Queue
	// +optional
	Status QueueStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
