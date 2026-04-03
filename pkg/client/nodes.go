package client

import (
	"context"
	"fmt"

	"github.com/nirvana-labs/nirvana-go/nks"
)

// ListWorkerNodes returns all worker nodes in a given pool.
func (c *Client) ListWorkerNodes(ctx context.Context, clusterID, poolID string) ([]WorkerNode, error) {
	pager := c.sdk.NKS.Clusters.Pools.Nodes.ListAutoPaging(ctx, clusterID, poolID, nks.ClusterPoolNodeListParams{})

	var nodes []WorkerNode
	for pager.Next() {
		nodes = append(nodes, convertNode(pager.Current()))
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing nodes in pool %s: %w", poolID, err)
	}

	return nodes, nil
}

// DeleteWorkerNode deletes a specific worker node from a pool.
// Fire-and-forget: returns as soon as the API accepts the request.
func (c *Client) DeleteWorkerNode(ctx context.Context, clusterID, poolID, nodeID string) error {
	_, err := c.sdk.NKS.Clusters.Pools.Nodes.Delete(ctx, clusterID, poolID, nodeID)
	if err != nil {
		return fmt.Errorf("deleting node %s from pool %s: %w", nodeID, poolID, err)
	}
	return nil
}

func convertNode(n nks.NKSNode) WorkerNode {
	var privateIP *string
	if n.PrivateIP != "" {
		privateIP = &n.PrivateIP
	}
	return WorkerNode{
		ID:        n.ID,
		Name:      n.Name,
		PrivateIP: privateIP,
		Status:    string(n.Status),
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}
