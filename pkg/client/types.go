package client

import "time"

type WorkerPool struct {
	ID         string     `json:"id"`
	ClusterID  string     `json:"cluster_id"`
	Name       string     `json:"name"`
	NodeCount  int        `json:"node_count"`
	Status     string     `json:"status"`
	NodeConfig NodeConfig `json:"node_config"`
	Tags       []string   `json:"tags"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type NodeConfig struct {
	InstanceType string     `json:"instance_type"`
	BootVolume   BootVolume `json:"boot_volume"`
	// Taints are the Kubernetes taints the Nirvana pool stamps onto every node
	// it provisions, each formatted as "key=value:Effect" (value may be empty:
	// "key=:Effect"). Karpenter must be told about these so it only schedules
	// tolerating pods onto this pool's nodes.
	Taints []string `json:"taints"`
}

type BootVolume struct {
	Size int `json:"size"`
}

type InstanceTypeSpec struct {
	Name     string `json:"name"`
	VCPU     int    `json:"vcpu"`
	MemoryGB int    `json:"memory_gb"`
	Region   string `json:"region"`
}

type WorkerNode struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PrivateIP *string   `json:"private_ip"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
