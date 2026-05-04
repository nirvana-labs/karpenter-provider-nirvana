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
