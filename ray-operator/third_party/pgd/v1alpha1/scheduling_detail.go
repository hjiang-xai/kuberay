package v1alpha1

import (
	"encoding/json"
	"reflect"
)

// This file defines structured error messages for why a PodGroupDeployment can't be scheduled on a given set of nodes.
// The error is ultimately seriealized into a JSON string to workaround Go's lack of tagged unions.
//
// Example:
// '{"type":"ResourceRequestsDetail","insufficientResources":[{"resourceName":"cpu","reason":"Insufficient cpu","requested":14000,"used":14000,"capacity":16000}]}'

// MarshalDetail marshals a scheduling detail struct to JSON with the "type" field
// automatically injected for tagged union parsing by UIs.
// The type name is derived from the Go struct name (e.g., NodeSelectorDetail).
func MarshalDetail(d any) ([]byte, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	typeName := reflect.TypeOf(d).Name()
	typePart := []byte(`{"type":"` + typeName + `",`)
	return append(typePart, data[1:]...), nil
}

// NodeSelectorDetail provides structured information about node selector mismatches.
type NodeSelectorDetail struct {
	Selector string `json:"selector"`
}

// NodeAffinityDetail provides structured information about node affinity mismatches.
// Each entry in Details is a pre-formatted, human-readable reason produced by the
// scheduler (e.g. "workload affinity requires label infra.x.ai/nodepoolname to be in [gpu-pool]").
type NodeAffinityDetail struct {
	Details []string `json:"details,omitempty"`
}

// TolerationDetail provides structured information about untolerated taints.
type TolerationDetail struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// InsufficientResource describes a single resource that is insufficient on a node.
type InsufficientResource struct {
	ResourceName string `json:"resourceName"`
	Reason       string `json:"reason"`
	Requested    int64  `json:"requested"`
	Used         int64  `json:"used"`
	Capacity     int64  `json:"capacity"`
}

// ResourceRequestsDetail provides structured information about insufficient resources.
type ResourceRequestsDetail struct {
	InsufficientResources []InsufficientResource `json:"insufficientResources"`
}

// AggregatedInsufficientResource describes aggregated stats for a single resource across multiple nodes.
// Whether the resource exceeds total capacity or just free resources is conveyed by the
// parent summary key ("will never fit" vs "currently doesn't fit"), so only the relevant
// average field is populated.
type AggregatedInsufficientResource struct {
	ResourceName string `json:"resourceName"`
	// AvgOverCapacity is the average amount by which the request exceeds total node capacity.
	AvgOverCapacity int64 `json:"avgOverCapacity,omitempty"`
	// AvgOverFree is the average amount by which the request exceeds free resources (capacity - used).
	AvgOverFree int64 `json:"avgOverFree,omitempty"`
}

// ResourceRequestsAggregatedDetail provides aggregated resource insufficiency information across multiple nodes.
// This replaces per-node ResourceRequestsDetail for grouping purposes so that nodes with
// slightly different usage numbers but the same class of problem are grouped together.
type ResourceRequestsAggregatedDetail struct {
	InsufficientResources []AggregatedInsufficientResource `json:"insufficientResources"`
}

// WorkloadAntiAffinityDetail provides structured information about workload anti-affinity violations.
type WorkloadAntiAffinityDetail struct {
	Deployment string `json:"deployment"`
}

// EvaluationErrorDetail provides structured information when a scheduling condition
// evaluation fails with an error.
type EvaluationErrorDetail struct {
	Error string `json:"error"`
}
