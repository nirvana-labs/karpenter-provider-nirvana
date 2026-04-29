package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=karpenter,shortName=nnc
// +kubebuilder:subresource:status

type NirvanaNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NirvanaNodeClassSpec   `json:"spec,omitempty"`
	Status            NirvanaNodeClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NirvanaNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NirvanaNodeClass `json:"items"`
}

type NirvanaNodeClassSpec struct{}
