package client

import (
	"context"
	"fmt"

	"github.com/nirvana-labs/nirvana-go/nks"
)

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

func (c *Client) GetPool(ctx context.Context, clusterID, poolID string) (*WorkerPool, error) {
	sdkPool, err := c.sdk.NKS.Clusters.Pools.Get(ctx, clusterID, poolID)
	if err != nil {
		return nil, fmt.Errorf("getting pool %s: %w", poolID, err)
	}

	pool := convertPool(*sdkPool)
	return &pool, nil
}

func convertPool(p nks.NKSNodePool) WorkerPool {
	return WorkerPool{
		ID:        p.ID,
		ClusterID: p.ClusterID,
		Name:      p.Name,
		NodeCount: int(p.NodeCount),
		Status:    string(p.Status),
		NodeConfig: NodeConfig{
			InstanceType: p.NodeConfig.InstanceType,
			BootVolume: BootVolume{
				Size: int(p.NodeConfig.BootVolume.Size),
			},
		},
		Tags:      p.Tags,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
