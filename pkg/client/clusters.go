package client

import (
	"context"
	"fmt"
)

type Cluster struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

func (c *Client) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	cluster, err := c.sdk.NKS.Clusters.Get(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("getting cluster %s: %w", clusterID, err)
	}
	return &Cluster{
		ID:     cluster.ID,
		Name:   cluster.Name,
		Region: string(cluster.Region),
	}, nil
}
