package client

import (
	"context"
	"fmt"

	nirvana "github.com/nirvana-labs/nirvana-go"
	"github.com/nirvana-labs/nirvana-go/nks"
	"github.com/rs/zerolog/log"
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

func (c *Client) CheckPoolUpdateAvailability(ctx context.Context, clusterID, poolID string, newNodeCount int) error {
	err := c.sdk.NKS.Clusters.Pools.Availability.Update(ctx, clusterID, poolID, nks.ClusterPoolAvailabilityUpdateParams{
		NodeCount: nirvana.Int(int64(newNodeCount)),
	})
	if err != nil {
		return fmt.Errorf("pool %s availability check failed for node count %d: %w", poolID, newNodeCount, err)
	}
	return nil
}

func (c *Client) UpdatePool(ctx context.Context, clusterID, poolID string, newNodeCount int) (string, error) {
	log.Info().
		Str("cluster_id", clusterID).
		Str("pool_id", poolID).
		Int("new_count", newNodeCount).
		Msg("updating pool node count")

	op, err := c.sdk.NKS.Clusters.Pools.Update(ctx, clusterID, poolID, nks.ClusterPoolUpdateParams{
		NodeCount: nirvana.Int(int64(newNodeCount)),
	})
	if err != nil {
		return "", fmt.Errorf("updating pool %s node count to %d: %w", poolID, newNodeCount, err)
	}
	return op.ID, nil
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
