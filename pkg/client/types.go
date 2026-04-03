package client

import "time"

// WorkerPool represents an NKS worker pool from the Nirvana API.
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

// NodeConfig represents the resource configuration for nodes in a pool.
type NodeConfig struct {
	// CPU configuration.
	CPUConfig CPUConfig `json:"cpu_config"`
	// Memory configuration.
	MemoryConfig MemoryConfig `json:"memory_config"`
	// Boot volume configuration.
	BootVolume BootVolume `json:"boot_volume"`
}

// CPUConfig represents the CPU configuration for a node.
type CPUConfig struct {
	// Number of virtual CPUs.
	VCPU int `json:"vcpu"`
}

// MemoryConfig represents the memory configuration for a node.
type MemoryConfig struct {
	// Size of the memory in GB.
	Size int `json:"size"`
}

// BootVolume represents the boot volume configuration for a node.
type BootVolume struct {
	// Size of the boot volume in GB.
	Size int `json:"size"`
}

// WorkerNode represents a worker node within a pool.
type WorkerNode struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PrivateIP *string   `json:"private_ip"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
