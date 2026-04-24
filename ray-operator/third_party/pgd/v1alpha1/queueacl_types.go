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

// QueueACLSpec defines the desired state of QueueACL.
type QueueACLSpec struct {
	// Namespaces is the list of namespaces allowed to use this queue.
	// PodGroupDeployments in unlisted namespaces will be rejected as unschedulable.
	Namespaces []string `json:"namespaces"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// QueueACL restricts which namespaces may use a given queue.
// The resource name must match the Queue name it controls.
type QueueACL struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the allowed namespaces for the queue
	// +required
	Spec QueueACLSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// QueueACLList contains a list of QueueACL
type QueueACLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueueACL `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QueueACL{}, &QueueACLList{})
}
