package v1alpha1

import (
	"github.com/awslabs/operatorpkg/status"
)

// NirvanaNodeClassStatus defines the observed state of NirvanaNodeClass.
type NirvanaNodeClassStatus struct {
	// Conditions contains condition information for the node class.
	// +optional
	Conditions []status.Condition `json:"conditions,omitempty"`

	// Pools is a snapshot of the discovered worker pools and their current state.
	// +optional
	Pools []PoolStatus `json:"pools,omitempty"`
}

// PoolStatus is a summary of a worker pool's current state.
type PoolStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NodeCount int    `json:"nodeCount"`
	VCPU      int    `json:"vcpu"`
	RAMGi     int    `json:"ramGi"`
	StorageGi int    `json:"storageGi"`
	Status    string `json:"status"`
}

// StatusConditions returns the condition set for the NirvanaNodeClass.
func (in *NirvanaNodeClass) StatusConditions() status.ConditionSet {
	return status.NewReadyConditions().For(in)
}

// GetConditions returns the conditions for the NirvanaNodeClass.
func (in *NirvanaNodeClass) GetConditions() []status.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions for the NirvanaNodeClass.
func (in *NirvanaNodeClass) SetConditions(conditions []status.Condition) {
	in.Status.Conditions = conditions
}
