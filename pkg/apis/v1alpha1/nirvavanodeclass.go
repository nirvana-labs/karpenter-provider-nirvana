package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=karpenter,shortName=nnc
// +kubebuilder:subresource:status

// NirvanaNodeClass is the CRD that configures Nirvana-specific provisioning parameters.
type NirvanaNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NirvanaNodeClassSpec   `json:"spec,omitempty"`
	Status            NirvanaNodeClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NirvanaNodeClassList contains a list of NirvanaNodeClass.
type NirvanaNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NirvanaNodeClass `json:"items"`
}

// NirvanaNodeClassSpec defines the desired state of NirvanaNodeClass.
type NirvanaNodeClassSpec struct {
	// ClusterID is the NKS cluster ID to scale pools within.
	ClusterID string `json:"clusterID"`

	// APIKeySecretRef references the Kubernetes Secret containing the Nirvana API key.
	APIKeySecretRef SecretRef `json:"apiKeySecretRef"`

	// APIEndpoint overrides the default Nirvana API endpoint.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// PoolSelector restricts which worker pools Karpenter may scale.
	// If empty, all pools in the cluster are eligible.
	// +optional
	PoolSelector *PoolSelector `json:"poolSelector,omitempty"`

	// CooldownMultiplier is the multiplier applied to provisioning time for cooldown.
	// Defaults to 2.0.
	// +optional
	CooldownMultiplier *float64 `json:"cooldownMultiplier,omitempty"`

	// MaxPollDuration is the maximum time to poll an operation before timing out.
	// Defaults to 30m.
	// +optional
	MaxPollDuration *metav1.Duration `json:"maxPollDuration,omitempty"`
}

// SecretRef is a reference to a Kubernetes Secret key.
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// PoolSelector filters which worker pools are eligible for scaling.
type PoolSelector struct {
	// Tags filters pools by their tags. A pool must have ALL specified tags.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// PoolIDs explicitly lists eligible pool IDs.
	// +optional
	PoolIDs []string `json:"poolIDs,omitempty"`
}
