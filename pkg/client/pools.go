package client

import (
	"context"
	"fmt"

	"github.com/nirvana-labs/nirvana-go/nks"
	"github.com/nirvana-labs/nirvana-go/operations"
	"github.com/nirvana-labs/nirvana-go/packages/param"
)

// ListPools returns all worker pools for the given NKS cluster.
func (c *Client) ListPools(ctx context.Context, clusterID string) ([]WorkerPool, error) {
	pager := c.sdk.NKS.Clusters.Pools.ListAutoPaging(ctx, clusterID, nks.ClusterPoolListParams{})

	var pools []WorkerPool
	for pager.Next() {
		pools = append(pools, convertPool(pager.Current()))
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	return pools, nil
}

// GetPool returns a single worker pool by ID.
func (c *Client) GetPool(ctx context.Context, clusterID, poolID string) (*WorkerPool, error) {
	sdkPool, err := c.sdk.NKS.Clusters.Pools.Get(ctx, clusterID, poolID)
	if err != nil {
		return nil, fmt.Errorf("getting pool %s: %w", poolID, err)
	}

	pool := convertPool(*sdkPool)
	return &pool, nil
}

// UpdatePool updates a worker pool's node count and returns the async operation.
func (c *Client) UpdatePool(ctx context.Context, clusterID, poolID string, nodeCount int) (*operations.Operation, error) {
	op, err := c.sdk.NKS.Clusters.Pools.Update(ctx, clusterID, poolID, nks.ClusterPoolUpdateParams{
		NodeCount: param.NewOpt(int64(nodeCount)),
	})
	if err != nil {
		return nil, fmt.Errorf("updating pool %s: %w", poolID, err)
	}

	return op, nil
}

// CheckPoolAvailability preflights a scale of pool to nodeCount without mutating state.
// Returns nil if the target size is provisionable; otherwise the API error.
func (c *Client) CheckPoolAvailability(ctx context.Context, clusterID, poolID string, nodeCount int) error {
	err := c.sdk.NKS.Clusters.Pools.Availability.Update(ctx, clusterID, poolID, nks.ClusterPoolAvailabilityUpdateParams{
		NodeCount: param.NewOpt(int64(nodeCount)),
	})
	if err != nil {
		return fmt.Errorf("pool %s availability check for nodeCount=%d: %w", poolID, nodeCount, err)
	}
	return nil
}

func convertPool(p nks.NKSNodePool) WorkerPool {
	return WorkerPool{
		ID:        p.ID,
		ClusterID: p.ClusterID,
		Name:      p.Name,
		NodeCount: int(p.NodeCount),
		Status:    string(p.Status),
		NodeConfig: NodeConfig{
			CPUConfig: CPUConfig{
				VCPU: int(p.NodeConfig.CPUConfig.Vcpu),
			},
			MemoryConfig: MemoryConfig{
				Size: int(p.NodeConfig.MemoryConfig.Size),
			},
			BootVolume: BootVolume{
				Size: int(p.NodeConfig.BootVolume.Size),
			},
		},
		Tags:      p.Tags,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
