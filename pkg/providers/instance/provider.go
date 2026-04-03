package instance

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cooldown"
)

// ResourceRequests represents the minimum resources a pool must satisfy.
type ResourceRequests struct {
	VCPU      int // minimum vCPUs required
	RAMGi     int // minimum RAM in GiB required
	StorageGi int // minimum storage in GiB required
}

const (
	// TODO: Fetch these limits from the cluster object's autoscaling config.
	maxNodeCount = 5
	minNodeCount = 1
	newNodeRetries      = 3
	newNodeRetryBackoff = 5 * time.Second
	statusReady         = "ready"
)

// Provider orchestrates pool selection, scaling, and new node identification.
type Provider struct {
	client          *client.Client
	cooldownManager *cooldown.Manager
	clusterID       string

	// Per-pool mutexes to serialize concurrent scale operations on the same pool.
	poolMu   sync.Mutex
	poolLocks map[string]*sync.Mutex
}

// New creates a new instance provider.
func New(c *client.Client, cm *cooldown.Manager, clusterID string) *Provider {
	return &Provider{
		client:          c,
		cooldownManager: cm,
		clusterID:       clusterID,
		poolLocks:       make(map[string]*sync.Mutex),
	}
}

func (p *Provider) lockPool(poolID string) *sync.Mutex {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()

	mu, ok := p.poolLocks[poolID]
	if !ok {
		mu = &sync.Mutex{}
		p.poolLocks[poolID] = mu
	}
	return mu
}

// SelectPool picks the eligible pool whose config most closely matches the
// requested resources (smallest pool that can still satisfy the requirements).
// Pools are filtered by readiness, cooldown, max capacity, and optional selectors.
func (p *Provider) SelectPool(
	ctx context.Context,
	selectorTags []string,
	selectorPoolIDs []string,
	requests ResourceRequests,
) (*client.WorkerPool, error) {
	pools, err := p.client.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	eligible := filterEligiblePools(pools, p.cooldownManager, selectorTags, selectorPoolIDs, requests)
	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible pools available for scaling")
	}

	return &eligible[0], nil
}

// ScaleUp increments the pool's node count by 1, waits for the operation to complete,
// and identifies the new worker node. The cooldown duration is set dynamically based
// on how long the operation actually took (multiplier × duration).
func (p *Provider) ScaleUp(ctx context.Context, pool *client.WorkerPool) (*client.WorkerNode, error) {
	mu := p.lockPool(pool.ID)
	mu.Lock()
	defer mu.Unlock()

	// Snapshot current nodes before scaling.
	existingNodes, err := p.client.ListWorkerNodes(ctx, p.clusterID, pool.ID)
	if err != nil {
		return nil, fmt.Errorf("listing existing nodes: %w", err)
	}
	existingIDs := make(map[string]bool, len(existingNodes))
	for _, n := range existingNodes {
		existingIDs[n.ID] = true
	}

	// Scale up: node_count + 1.
	newCount := pool.NodeCount + 1
	p.cooldownManager.RecordScaleStart(pool.ID)

	op, err := p.client.UpdatePool(ctx, p.clusterID, pool.ID, newCount)
	if err != nil {
		p.cooldownManager.RecordScaleComplete(pool.ID)
		return nil, fmt.Errorf("updating pool %s node count to %d: %w", pool.ID, newCount, err)
	}

	// Block until the operation completes — same pattern as GCP's Create.
	_, err = p.client.WaitForOperation(ctx, op.ID)
	if err != nil {
		p.cooldownManager.RecordScaleComplete(pool.ID)
		return nil, fmt.Errorf("waiting for scale-up operation %s: %w", op.ID, err)
	}

	// Dynamic cooldown: set based on how long the operation actually took.
	p.cooldownManager.RecordScaleComplete(pool.ID)

	// Identify the new node by diffing current nodes against the snapshot.
	newNode, err := p.identifyNewNode(ctx, pool.ID, existingIDs)
	if err != nil {
		return nil, fmt.Errorf("identifying new node in pool %s: %w", pool.ID, err)
	}

	return newNode, nil
}

// DeleteNodeByID deletes a specific worker node from its pool by node ID.
// This is fire-and-forget: it returns as soon as the API accepts the request.
func (p *Provider) DeleteNodeByID(ctx context.Context, poolID, nodeID string) error {
	return p.client.DeleteWorkerNode(ctx, p.clusterID, poolID, nodeID)
}


// CanScaleDown returns true if the pool is above the minimum node count.
func (p *Provider) CanScaleDown(ctx context.Context, poolID string) (bool, error) {
	// TODO: Fetch min node count from the cluster object's autoscaling config.
	pool, err := p.client.GetPool(ctx, p.clusterID, poolID)
	if err != nil {
		return false, fmt.Errorf("getting pool %s: %w", poolID, err)
	}
	return pool.NodeCount > minNodeCount, nil
}

// ListNodes returns all worker nodes in the given pool.
func (p *Provider) ListNodes(ctx context.Context, poolID string) ([]client.WorkerNode, error) {
	return p.client.ListWorkerNodes(ctx, p.clusterID, poolID)
}

// GetPool returns a single worker pool by ID.
func (p *Provider) GetPool(ctx context.Context, poolID string) (*client.WorkerPool, error) {
	return p.client.GetPool(ctx, p.clusterID, poolID)
}

// ListPools returns all pools in the cluster.
func (p *Provider) ListPools(ctx context.Context) ([]client.WorkerPool, error) {
	return p.client.ListPools(ctx, p.clusterID)
}

func (p *Provider) identifyNewNode(ctx context.Context, poolID string, existingIDs map[string]bool) (*client.WorkerNode, error) {
	for attempt := range newNodeRetries {
		nodes, err := p.client.ListWorkerNodes(ctx, p.clusterID, poolID)
		if err != nil {
			return nil, fmt.Errorf("listing nodes (attempt %d): %w", attempt+1, err)
		}

		for _, node := range nodes {
			if !existingIDs[node.ID] {
				return &node, nil
			}
		}

		if attempt < newNodeRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(newNodeRetryBackoff):
			}
		}
	}

	return nil, fmt.Errorf("new node not found after %d retries", newNodeRetries)
}

// filterEligiblePools returns pools that can satisfy the requested resources,
// sorted by best fit (smallest excess resources first). Pools that don't meet
// the minimum CPU/RAM/storage requirements are excluded.
func filterEligiblePools(
	pools []client.WorkerPool,
	cm *cooldown.Manager,
	selectorTags []string,
	selectorPoolIDs []string,
	requests ResourceRequests,
) []client.WorkerPool {
	poolIDSet := make(map[string]bool, len(selectorPoolIDs))
	for _, id := range selectorPoolIDs {
		poolIDSet[id] = true
	}

	var eligible []client.WorkerPool
	for _, pool := range pools {
		if pool.Status != statusReady {
			continue
		}
		if cm.IsInCooldown(pool.ID) {
			continue
		}
		if pool.NodeCount >= maxNodeCount {
			continue
		}
		if len(selectorPoolIDs) > 0 && !poolIDSet[pool.ID] {
			continue
		}
		if !hasAllTags(pool.Tags, selectorTags) {
			continue
		}
		// Only include pools that can satisfy the resource requirements.
		if pool.NodeConfig.CPUConfig.VCPU < requests.VCPU ||
			pool.NodeConfig.MemoryConfig.Size < requests.RAMGi ||
			pool.NodeConfig.BootVolume.Size < requests.StorageGi {
			continue
		}
		eligible = append(eligible, pool)
	}

	// Sort by best fit: smallest total excess resources first.
	// This picks the pool closest to what's needed, minimizing waste.
	slices.SortFunc(eligible, func(a, b client.WorkerPool) int {
		excessA := poolExcess(a.NodeConfig, requests)
		excessB := poolExcess(b.NodeConfig, requests)
		if excessA != excessB {
			return int(excessA - excessB)
		}
		// Tie-break by smallest config.
		if a.NodeConfig.CPUConfig.VCPU != b.NodeConfig.CPUConfig.VCPU {
			return a.NodeConfig.CPUConfig.VCPU - b.NodeConfig.CPUConfig.VCPU
		}
		return a.NodeConfig.MemoryConfig.Size - b.NodeConfig.MemoryConfig.Size
	})

	return eligible
}

// poolExcess returns a score representing how much a pool's config exceeds
// the requested resources. Lower is better (closer fit).
func poolExcess(cfg client.NodeConfig, req ResourceRequests) int64 {
	cpuExcess := int64(cfg.CPUConfig.VCPU - req.VCPU)
	ramExcess := int64(cfg.MemoryConfig.Size - req.RAMGi)
	storageExcess := int64(cfg.BootVolume.Size - req.StorageGi)
	return cpuExcess*cpuExcess + ramExcess*ramExcess + storageExcess*storageExcess
}

func hasAllTags(poolTags, requiredTags []string) bool {
	if len(requiredTags) == 0 {
		return true
	}
	tagSet := make(map[string]bool, len(poolTags))
	for _, t := range poolTags {
		tagSet[t] = true
	}
	for _, required := range requiredTags {
		if !tagSet[required] {
			return false
		}
	}
	return true
}
