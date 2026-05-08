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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeploymentStrategy describes how to replace existing pods with new ones.
type DeploymentStrategy struct {
	// Type of deployment. Can be "Recreate" or "RollingUpdate" or "OnDelete". Default is RollingUpdate.
	// +optional
	Type DeploymentStrategyType `json:"type,omitempty" protobuf:"bytes,1,opt,name=type,casttype=DeploymentStrategyType"`

	// Rolling update config params. Present only if DeploymentStrategyType =
	// RollingUpdate.
	// ---
	// +optional
	RollingUpdate *RollingUpdateDeployment `json:"rollingUpdate,omitempty" protobuf:"bytes,2,opt,name=rollingUpdate"`
}

// +enum
type DeploymentStrategyType string

const (
	// Kill all existing pods before creating new ones.
	RecreateDeploymentStrategyType DeploymentStrategyType = "Recreate"

	// Replace the old ReplicaSets by new one using rolling update i.e gradually scale down the old ReplicaSets and scale up the new one.
	RollingUpdateDeploymentStrategyType DeploymentStrategyType = "RollingUpdate"

	// OnDeleteDeploymentStrategyType ignores changes to currently existing podgroups and will only apply changes to newly created ones.
	OnDeleteDeploymentStrategyType DeploymentStrategyType = "OnDelete"
)

// Spec to control the desired behavior of rolling update.
type RollingUpdateDeployment struct {

	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,1,opt,name=maxUnavailable"`

	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty" protobuf:"bytes,2,opt,name=maxSurge"`
}

// PodGroupDeploymentSpec defines the desired state of PodGroupDeployment.
// +kubebuilder:pruning:PreserveUnknownFields
type PodGroupDeploymentSpec struct {
	// statefulspec that will be created per group
	Deployment appsv1.StatefulSetSpec `json:"deployment"`

	// EnableServiceManagement defines if the headless services should be managed by the podgroupdeployment operator
	// This also enables featureFlags.enableGroupHashes due to historic reasons.
	// +optional
	EnableServiceManagement bool `json:"enableServiceManagement,omitempty" default:"false"`

	// ModifiedAt can be set by a client upon updating the spec.
	// When sorting workloads by priority for scheduling we will prefer older modified workloads over newer ones.
	// This field is optional and
	// +optional
	ModifiedAt *metav1.Time `json:"modifiedAt,omitempty"`

	// NoPreemptions disables preemption for this deployment.
	// When true, only groups that fit without preempting other pods are scheduled.
	// +optional
	NoPreemptions bool `json:"noPreemptions,omitempty" default:"false"`

	// InjectDefaultEnvVars defines if the default environment variables should be injected into the pods of this workload
	InjectDefaultEnvVars bool `json:"injectDefaultEnvVars,omitempty" default:"false"`

	// Priority defines the scheduling priority of this PodGroupDeployment. Higher values indicate
	// higher priority and will be scheduled before lower priority deployments. This is used in
	// combination with queue priority to determine overall scheduling order.
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Queue specifies the name of the Queue resource to associate with this PodGroupDeployment.
	// Queues provide both a priority level (which affects scheduling order) and GPU capacity limits.
	// The scheduler will check if the queue has sufficient GPU capacity before scheduling groups.
	// +optional
	Queue string `json:"queue,omitempty"`

	// GroupBy allows multiple PodGroupDeployments to be treated as a single atomic scheduling unit.
	// When set, PodGroupDeployments with the same grouping key will be scheduled together based on
	// the highest priority within the group. Exactly one of Owner or Key must be specified.
	// +optional
	GroupBy *GroupBy `json:"groupBy,omitempty"`

	// Subdomain sets the subdomain for pods in this deployment, used for DNS-based pod discovery.
	// Pods will be addressable as <pod-name>.<subdomain>.<namespace>.svc.cluster.local.
	// When EnableServiceManagement is true, this field is overridden by the group name.
	// +optional
	Subdomain string `json:"subdomain,omitempty"`

	// ServicePorts defines the ports that will be exposed by the service for each group.
	// This field is only used if EnableServiceManagement is set to true.
	// +optional
	ServicePorts []corev1.ServicePort `json:"servicePorts,omitempty"`

	// MinGroups defines the minimum number of groups required for this deployment to be
	// considered runnable. If the scheduler cannot schedule at least MinGroups, it will
	// remove all scheduled groups and mark the deployment as failed. If not set, defaults
	// to 0 (opportunistic scheduling). The scheduler will attempt to allocate at least
	// `MinGroups` and at most `Groups` groups.
	// +optional
	MinGroups int32 `json:"minGroups,omitempty"`

	// Fields inspired by PodGroup
	// groups defines the number of individual groups
	Groups int32 `json:"groups"`

	// groupSize defines the minsize of one individual podGroup
	// +kubebuilder:validation:Minimum=1
	GroupSize int32 `json:"groupSize"`

	// The deployment strategy to use to replace existing podgroups with new podgroups.
	// +optional
	// +patchStrategy=retainKeys
	Strategy DeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys" protobuf:"bytes,4,opt,name=strategy"`

	// RequiredTopologyKey defines the topology key to use for the podgroup deployment's pod affinity, specifically for
	// the requiredDuringSchedulingIgnoredDuringExecution field. If not set, there will be no required topology key.
	// +optional
	RequiredTopologyKey string `json:"requiredTopologyKey,omitempty"`

	// SchedulerName defines the name of the scheduler to use for the podgroup deployment.
	SchedulerName string `json:"schedulerName,omitempty"`

	// BlacklistedNodes allows to specify nodes that just this podgroup deployment should not be scheduled on.
	BlacklistedNodes []BlacklistedNode `json:"blacklistedNodes,omitempty"`

	// Movable indicates this workload can be relocated across racks by the
	// scheduler to reduce priority fragmentation. When set, the scheduler may
	// delete and reschedule groups of this PGD to consolidate same-priority
	// workloads onto fewer racks, making full racks available for larger
	// multi-node training jobs. Only movable PGDs may be preempted by the
	// defragmentation logic; non-movable PGDs are never displaced for defrag.
	// +optional
	Movable bool `json:"movable,omitempty"`

	// FeatureFlags enable or disable new features. When implementing a feature flag, make sure that the
	// new behavior corresponds to the `true` value, while the former behavior corresponds to the `false` value.
	// This is to ensure that all newly created objects will receive the feature flag.
	FeatureFlags FeatureFlags `json:"featureFlags,omitempty"`
}

// GroupBy defines how multiple PodGroupDeployments can be grouped together as a single
// scheduling unit. This enables atomic scheduling of related deployments, where all
// deployments in a group are scheduled together based on the highest priority within
// the group. Exactly one of Owner or Key must be specified.
type GroupBy struct {
	// Owner groups PodGroupDeployments that share the same owner reference (controller).
	// This is useful when a higher-level controller creates multiple PodGroupDeployments
	// that should be scheduled as a unit.
	// +optional
	Owner *Owner `json:"owner,omitempty"`

	// Key groups PodGroupDeployments by an arbitrary string identifier.
	// All PodGroupDeployments with the same Key value in the same namespace
	// will be treated as a single scheduling unit.
	// +optional
	Key string `json:"key,omitempty"`
}

// Owner is a marker type indicating that PodGroupDeployments should be grouped
// by their owner reference. When used, all PodGroupDeployments controlled by the
// same parent resource will be scheduled together as a unit.
type Owner struct{}

type FeatureFlags struct {
	// EnableGroupHashes generates group names with an alphanumeric suffix.
	EnableGroupHashes bool `json:"enableGroupHashes,omitempty"`
}

// +kubebuilder:validation:Enum=NodeSelector;NodeAffinity;Tolerations;ResourceRequests;WorkloadAntiAffinity;BlacklistedNodes
// +enum
type SchedulingCategory string

const (
	SchedulingCategoryNodeSelector         SchedulingCategory = "NodeSelector"
	SchedulingCategoryNodeAffinity         SchedulingCategory = "NodeAffinity"
	SchedulingCategoryTolerations          SchedulingCategory = "Tolerations"
	SchedulingCategoryResourceRequests     SchedulingCategory = "ResourceRequests"
	SchedulingCategoryWorkloadAntiAffinity SchedulingCategory = "WorkloadAntiAffinity"
	SchedulingCategoryBlacklistedNodes     SchedulingCategory = "BlacklistedNodes"
)

// NodeDetails defines why specific nodes are unschedulable for scheduling a given PodGroupDeployment
// +kubebuilder:validation:XPreserveUnknownFields
type NodeDetails struct {
	// Detail contains a detailed, structured explanation of why the nodes are unschedulable.
	// This is a JSON-encoded string that contains structured data, see: scheduling_detail.go potential values.
	// +optional
	Detail string `json:"detail,omitempty"`
	// Nodes is a comma-separated list of node names that are ineligible for scheduling the PodGroupDeployment.
	Nodes string `json:"nodes"`
	// Topologies is a comma-separated list of topologies that the nodes are in.
	Topologies string `json:"topologies,omitempty"`
}

// Deprecated: Use UnschedulableNodes instead.
type IneligibleNodes map[string]map[SchedulingCategory]NodeDetails

// UnschedulableNodes is a map of category (e.g. ResourceRequests) to summary string to list of NodeDetails sharing that reason for unschedulability.
type UnschedulableNodes map[SchedulingCategory]map[string][]NodeDetails

// PodGroupDeploymentStatus defines the observed state of PodGroupDeployment.
type PodGroupDeploymentStatus struct {
	// Selector is a serialized label selector string that identifies the pods belonging to this PodGroupDeployment
	// +optional
	Selector string `json:"selector,omitempty"`

	// Summary provides aggregate counts of StatefulSet groups
	// +optional
	Summary GroupsSummary `json:"summary,omitempty"`

	// Conditions represents the latest available observations of the PodGroupDeployment's current state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReconcileError holds reasons why groups failed to be created
	// +optional
	// +kubebuilder:example="Multiple groups failed to schedule:\n  * group-abc: insufficient cpu\n  * group-def: node affinity not satisfied"
	ReconcileError string `json:"reconcileError,omitempty"`

	// IneligibleNodes contains a summary on which nodes could not fulfil the request.
	// +optional
	// Deprecated: Use UnschedulableNodes instead.
	IneligibleNodes IneligibleNodes `json:"ineligibleNodes,omitempty"`

	// UnschedulableNodes contains a summary on which nodes could not be scheduled.
	// +optional
	UnschedulableNodes UnschedulableNodes `json:"unschedulableNodes,omitempty"`

	// NotReadyGroups contains status information only for StatefulSets that are not ready
	// +optional
	NotReadyGroups []GroupStatus `json:"notReadyGroups,omitempty"`

	// ApplyErrors counts consecutive scheduling cycles where all pod creations for this
	// PodGroupDeployment failed during the Execute phase. When this count reaches
	// MaxApplyRetries, the PGD is treated as a fatal error.
	// This counter is reset to 0 when a server-side dry-run pod creation succeeds,
	// indicating the underlying issue has been resolved.
	// +optional
	ApplyErrors int32 `json:"applyErrors,omitempty"`

	// LastDefragTime is when handleMove last performed a defrag action.
	// Used as a cooldown to prevent continuous churn.
	// +optional
	LastDefragTime *metav1.Time `json:"lastDefragTime,omitempty"`
}

// GroupsSummary provides aggregate status information for all StatefulSet groups
type GroupsSummary struct {
	// ReadyGroups represents the number of StatefulSet groups that are ready
	ReadyGroups int32 `json:"readyGroups,omitempty"`

	// TotalGroups represents the total number of StatefulSet groups
	TotalGroups int32 `json:"totalGroups,omitempty"`

	// Allocated groups are groups that the scheduler has allocated resources to.
	// This only gets filled if .spec.enableScheduling is true.
	AllocatedGroups int32 `json:"allocatedGroups,omitempty"`

	// UpdatedGroups represents the number of groups that are at the desired version
	UpdatedGroups int32 `json:"updatedGroups,omitempty"`

	// TotalReadyReplicas is the total number of ready replicas across all groups
	TotalReadyReplicas int32 `json:"totalReadyReplicas,omitempty"`

	// TotalDesiredReplicas is the total number of desired replicas across all groups
	TotalDesiredReplicas int32 `json:"totalDesiredReplicas,omitempty"`

	// MaxSchedulableGroups represents is the theoretical maximum number of groups that can be scheduled for this workload.
	// This includes potential preemptions of lower priority workloads.
	// This Field is only filled if there is an active demand for more groups. This is the case because otherwise
	// we would need to execute the scheduling work even when there is no demand for more groups.
	MaxSchedulableGroups int32 `json:"maxSchedulableGroups,omitempty"`

	// UncappedMaxSchedulable is the theoretical maximum schedulable groups ignoring queue GPU limits.
	// Unlike MaxSchedulableGroups (which is capped by the queue), this field shows the raw capacity
	// from the heap (free + preemptable nodes). It is populated both when there is active demand
	// and when there is no demand (via the background status update).
	// +optional
	UncappedMaxSchedulable int32 `json:"uncappedMaxSchedulable,omitempty"`

	// PreemptableGroups represents the number of additional groups that could be scheduled
	// if preempting lower priority workloads.
	PreemptableGroups int32 `json:"preemptableGroups,omitempty"`

	// QueuedGroups represents the number of groups that are currently waiting to get an allocation in this queue.
	QueuedGroups int32 `json:"queuedGroups,omitempty"`

	// PendingPreemptionGroups is the number of groups whose creation is blocked because
	// victim pods on the target nodes must finish being preempted first. When a group
	// requires preempting existing lower-priority pods, the scheduler issues delete
	// operations for the victims and waits for the nodes to free up before creating
	// replacement pods.
	PendingPreemptionGroups int32 `json:"pendingPreemptionGroups,omitempty"`

	// PendingTerminationGroups is the number of groups whose creation is blocked because
	// existing groups are currently terminating. This happens when an experiment PGD has
	// groups being deleted (e.g. failed groups) and the replacement would require
	// preemptions — the scheduler waits for the terminations to complete to reduce churn.
	PendingTerminationGroups int32 `json:"pendingTerminationGroups,omitempty"`

	// TerminalGroups is the number of groups that are terminal and don't require a restart.
	TerminalGroups int32 `json:"terminalGroups,omitempty"`

	// PendingMoves is the number of groups whose scheduling is blocked because
	// higher-priority movable PGDs must be relocated first. The scheduler has
	// initiated surge pods on target racks for those PGDs; once the surges are
	// ready and the source pods are deleted, the freed rack capacity allows
	// this PGD's groups to schedule normally.
	// +optional
	PendingMoves int32 `json:"pendingMoves,omitempty"`

	// SurgeGroups is the number of extra groups temporarily created by the defrag
	// logic to consolidate this movable PGD onto fewer racks. Computed fresh each
	// cycle from rack analysis. As each surge group becomes ready, a corresponding
	// group on a poorly-placed rack is deleted, keeping the effective count at
	// spec.groups while improving rack-level priority variance.
	// +optional
	SurgeGroups int32 `json:"surgeGroups,omitempty"`
}

// GroupStatus contains status information for a StatefulSet group
type GroupStatus struct {
	// Name of the StatefulSet
	Name string `json:"name,omitempty"`

	// GroupIndex represents the index of this group (0-based)
	GroupIndex int32 `json:"groupIndex,omitempty"`

	// ReadyReplicas is the number of pods created by this StatefulSet that have ready status
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas for this StatefulSet
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// Message provides additional information about why the group is not ready
	// +optional
	Message string `json:"message,omitempty"`
}

type BlacklistedNode struct {
	// NodeName is the name of the node that is blacklisted.
	NodeName string `json:"nodeName"`
	// Reason is the reason why the node is blacklisted. This will show in the unschedulable error message.
	Reason string `json:"reason,omitempty"`
	// Timestamp is when the node was blacklisted. Propagated from the Experiment CRD.
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pgd
// +kubebuilder:printcolumn:name="Groups",type=integer,JSONPath=`.spec.groups`,description="The number of individual groups"
// +kubebuilder:printcolumn:name="Group Size",type=integer,JSONPath=`.spec.groupSize`,description="The minimum size of an individual group"
// +kubebuilder:printcolumn:name="Ready Groups",type=integer,JSONPath=`.status.summary.readyGroups`,description="The number of Groups that are ready"
// +kubebuilder:printcolumn:name="Allocated Groups",type=integer,JSONPath=`.status.summary.allocatedGroups`,description="The number of Groups that have been allocated resources"
// +kubebuilder:printcolumn:name="Queued Groups",type=integer,JSONPath=`.status.summary.queuedGroups`,description="The number of groups that are currently waiting to get an allocation in this queue"
// +kubebuilder:printcolumn:name="Total Desired",type=integer,JSONPath=`.status.summary.totalDesiredReplicas`,description="The total number of desired replicas across all groups"
// +kubebuilder:printcolumn:name="Total Ready",type=integer,JSONPath=`.status.summary.totalReadyReplicas`,description="The total number of ready replicas across all groups"
// +kubebuilder:printcolumn:name="Queue",type=string,JSONPath=`.spec.queue`,description="The name of the queue"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].reason`,description="The status of the pod group deployment"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="The age of the PodGroupDeployment"
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.deployment.template.spec.containers[0].image`,description="The container image name",priority=1
// +kubebuilder:subresource:scale:specpath=.spec.groups,statuspath=.status.summary.allocatedGroups,selectorpath=.status.selector

// PodGroupDeployment is the Schema for the podgroupdeployments API.
// +kubebuilder:validation:XValidation:message="name must be at most 54 characters",rule="size(self.metadata.name) <= 54"
type PodGroupDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodGroupDeploymentSpec   `json:"spec,omitempty"`
	Status PodGroupDeploymentStatus `json:"status,omitempty"`
}

// IsNodeTopology returns true when the group is scoped to a single node,
// identified by the hostname topology key.
func (pgd *PodGroupDeployment) IsNodeTopology() bool {
	return pgd.Spec.RequiredTopologyKey == "kubernetes.io/hostname"
}

func (pgd *PodGroupDeployment) NodesPerGroup() int32 {
	if pgd.IsNodeTopology() {
		return 1
	}
	return pgd.Spec.GroupSize
}

// IsOnDeleteStrategy returns true when the strategy is OnDelete or unset (the default).
func (pgd *PodGroupDeployment) IsOnDeleteStrategy() bool {
	return pgd.Spec.Strategy.Type == OnDeleteDeploymentStrategyType || pgd.Spec.Strategy.Type == ""
}

func (pgd *PodGroupDeployment) IsRestartPolicyAlways() bool {
	return pgd.Spec.Deployment.Template.Spec.RestartPolicy == corev1.RestartPolicyAlways ||
		pgd.Spec.Deployment.Template.Spec.RestartPolicy == "" // default to Always
}

// ModifiedAt returns a timestamp used for priority sorting of PodGroupDeployments.
// The default is the creation timestamp.
// But if a client set the modifiedAt field, it will be used instead.
func (pgd *PodGroupDeployment) ModifiedAt() *metav1.Time {
	if pgd.Spec.ModifiedAt != nil {
		return pgd.Spec.ModifiedAt
	}

	return &pgd.CreationTimestamp
}

func (pgd *PodGroupDeployment) CouldBenefitFromMoves() bool {
	if pgd.Spec.GroupSize > 1 && !pgd.IsNodeTopology() {
		return true
	}
	return false
}

// +kubebuilder:object:root=true

// PodGroupDeploymentList contains a list of PodGroupDeployment.
type PodGroupDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodGroupDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodGroupDeployment{}, &PodGroupDeploymentList{})
}
