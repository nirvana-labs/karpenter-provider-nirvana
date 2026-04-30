package v1alpha1

import (
	"github.com/awslabs/operatorpkg/status"
)

type NirvanaNodeClassStatus struct {
	Conditions []status.Condition `json:"conditions,omitempty"`
	Pools      []PoolStatus       `json:"pools,omitempty"`
}

type PoolStatus struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NodeCount    int    `json:"nodeCount"`
	InstanceType string `json:"instanceType"`
	StorageGi    int    `json:"storageGi"`
	Status       string `json:"status"`
}

func (in *NirvanaNodeClass) StatusConditions() status.ConditionSet {
	return status.NewReadyConditions().For(in)
}

func (in *NirvanaNodeClass) GetConditions() []status.Condition {
	return in.Status.Conditions
}

func (in *NirvanaNodeClass) SetConditions(conditions []status.Condition) {
	in.Status.Conditions = conditions
}
