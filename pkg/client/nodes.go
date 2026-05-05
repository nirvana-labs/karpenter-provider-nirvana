package client

import (
	"context"
	"fmt"

	"github.com/nirvana-labs/nirvana-go/nks"
)

func (c *Client) ListWorkerNodes(ctx context.Context, clusterID, poolID string) ([]WorkerNode, error) {
	pager := c.sdk.NKS.Clusters.Pools.Nodes.ListAutoPaging(ctx, clusterID, poolID, nks.ClusterPoolNodeListParams{})

	var nodes []WorkerNode
	for pager.Next() {
		nodes = append(nodes, convertNode(pager.Current()))
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing nodes for pool %s: %w", poolID, err)
	}

	return nodes, nil
}

func (c *Client) DeleteWorkerNode(ctx context.Context, clusterID, poolID, nodeID string) (string, error) {
	op, err := c.sdk.NKS.Clusters.Pools.Nodes.Delete(ctx, clusterID, poolID, nodeID)
	if err != nil {
		return "", fmt.Errorf("deleting node %s from pool %s: %w", nodeID, poolID, err)
	}
	return op.ID, nil
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
