package cloudprovider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/awslabs/operatorpkg/status"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/nirvana-labs/nirvana-go/operations"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cooldown"
)

const (
	providerName    = "nirvana"
	maxNodesPerPool = 4
	minNodesPerPool = 2
)

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	nirvanaClient *client.Client
	clusterID     string
	region        string
	cooldowns     *cooldown.Manager
}

func New(nirvanaClient *client.Client, clusterID, region string, cooldowns *cooldown.Manager) *CloudProvider {
	return &CloudProvider{
		nirvanaClient: nirvanaClient,
		clusterID:     clusterID,
		region:        region,
		cooldowns:     cooldowns,
	}
}

func (p *CloudProvider) Name() string {
	return providerName
}

func (p *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	requestedType := instanceTypeFromRequirements(nodeClaim)

	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Str("requested_instance_type", requestedType).
		Msg("create: received nodeclaim")

	pools, err := p.nirvanaClient.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	log.Info().
		Int("pool_count", len(pools)).
		Str("requested_instance_type", requestedType).
		Msg("create: fetched pools")

	pool, err := p.selectPoolForCreate(pools, requestedType)
	if err != nil {
		log.Warn().Err(err).
			Str("nodeclaim", nodeClaim.Name).
			Str("requested_instance_type", requestedType).
			Msg("create: no eligible pool found")
		return nil, err
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Str("instance_type", pool.NodeConfig.InstanceType).
		Int("current_count", pool.NodeCount).
		Msg("create: selected pool")

	specs, err := p.nirvanaClient.ListInstanceTypes(ctx)
	if err != nil {
		p.cooldowns.ClearCooldown(pool.ID)
		return nil, fmt.Errorf("listing instance types: %w", err)
	}
	capacity, err := capacityFromSpec(pool.NodeConfig.InstanceType, specs, pool.NodeConfig.BootVolume.Size)
	if err != nil {
		p.cooldowns.ClearCooldown(pool.ID)
		return nil, fmt.Errorf("resolving capacity for pool %s: %w", pool.ID, err)
	}

	targetCount := pool.NodeCount + 1

	if err := p.nirvanaClient.CheckPoolUpdateAvailability(ctx, p.clusterID, pool.ID, targetCount); err != nil {
		p.cooldowns.ClearCooldown(pool.ID)
		log.Warn().Err(err).Str("pool_id", pool.ID).Int("target_count", targetCount).Msg("create: no availability for scale-up")
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no availability for pool %s: %w", pool.ID, err))
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Int("current_count", pool.NodeCount).
		Int("target_count", targetCount).
		Msg("now creating vm")

	operationID, err := p.nirvanaClient.UpdatePool(ctx, p.clusterID, pool.ID, targetCount)
	if err != nil {
		p.cooldowns.ClearCooldown(pool.ID)
		return nil, fmt.Errorf("scaling pool %s: %w", pool.ID, err)
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("operation_id", operationID).
		Msg("create: scale-up operation submitted, waiting for completion")

	if _, err := p.waitForOperation(ctx, pool.ID, operationID, "create"); err != nil {
		return nil, fmt.Errorf("waiting for scale-up of pool %s: %w", pool.ID, err)
	}

	newNode, err := p.latestNode(ctx, pool.ID, targetCount)
	if err != nil {
		return nil, fmt.Errorf("finding new node in pool %s: %w", pool.ID, err)
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("node_id", newNode.ID).
		Str("node_name", newNode.Name).
		Msg("create: new node identified")

	return &karpv1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				corev1.LabelInstanceTypeStable: pool.NodeConfig.InstanceType,
				corev1.LabelTopologyZone:       p.region,
				karpv1.CapacityTypeLabelKey:    karpv1.CapacityTypeOnDemand,
			},
		},
		Status: karpv1.NodeClaimStatus{
			ProviderID:  fmt.Sprintf("nirvana://%s/%s/%s", p.clusterID, pool.ID, newNode.ID),
			Capacity:    capacity,
			Allocatable: capacity,
		},
	}, nil
}

func (p *CloudProvider) Delete(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Str("provider_id", nodeClaim.Status.ProviderID).
		Msg("delete: received nodeclaim")

	_, poolID, nodeID, err := parseProviderID(nodeClaim.Status.ProviderID)
	if err != nil {
		return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("invalid provider id: %w", err))
	}

	if !p.cooldowns.TryReserve(poolID) {
		remaining := p.cooldowns.GetCooldownRemaining(poolID)
		log.Warn().
			Str("pool_id", poolID).
			Dur("remaining", remaining).
			Msg("delete: pool in cooldown")
		return fmt.Errorf("pool %s in cooldown for %s", poolID, remaining)
	}

	log.Info().
		Str("pool_id", poolID).
		Str("node_id", nodeID).
		Msg("now deleting vm")

	operationID, err := p.nirvanaClient.DeleteWorkerNode(ctx, p.clusterID, poolID, nodeID)
	if err != nil {
		p.cooldowns.ClearCooldown(poolID)
		if client.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("node %s not found: %w", nodeID, err))
		}
		return err
	}

	log.Info().
		Str("pool_id", poolID).
		Str("node_id", nodeID).
		Str("operation_id", operationID).
		Msg("delete: node deletion submitted, waiting for completion")

	if _, err := p.waitForOperation(ctx, poolID, operationID, "delete"); err != nil {
		return fmt.Errorf("waiting for deletion of node %s: %w", nodeID, err)
	}

	return nil
}

func (p *CloudProvider) Get(ctx context.Context, providerID string) (*karpv1.NodeClaim, error) {
	return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
}

func (p *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	return nil, nil
}

func (p *CloudProvider) GetInstanceTypes(ctx context.Context, _ *karpv1.NodePool) ([]*cloudprovider.InstanceType, error) {
	pools, err := p.nirvanaClient.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	specs, err := p.nirvanaClient.ListInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instance types: %w", err)
	}

	specMap := make(map[string]client.InstanceTypeSpec, len(specs))
	for _, s := range specs {
		specMap[s.Name] = s
	}

	return PoolsToInstanceTypes(pools, specMap, p.region), nil
}

func (p *CloudProvider) IsDrifted(ctx context.Context, nodeClaim *karpv1.NodeClaim) (cloudprovider.DriftReason, error) {
	return "", nil
}

func (p *CloudProvider) RepairPolicies() []cloudprovider.RepairPolicy {
	return nil
}

func (p *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1alpha1.NirvanaNodeClass{}}
}

var errPoolsTemporarilyUnavailable = fmt.Errorf("all candidate pools are temporarily unavailable")

func (p *CloudProvider) selectPoolForCreate(pools []client.WorkerPool, requestedType string) (*client.WorkerPool, error) {
	candidates := make([]int, 0, len(pools))
	hasTemporarySkip := false

	for i, pool := range pools {
		if requestedType != "" && pool.NodeConfig.InstanceType != requestedType {
			log.Debug().Str("pool_id", pool.ID).Str("pool_type", pool.NodeConfig.InstanceType).Str("requested", requestedType).Msg("create: skipping pool, instance type mismatch")
			continue
		}
		if pool.Status != "ready" {
			log.Debug().Str("pool_id", pool.ID).Str("status", pool.Status).Msg("create: skipping pool, not ready")
			hasTemporarySkip = true
			continue
		}
		if pool.NodeCount >= maxNodesPerPool {
			log.Warn().Str("pool_id", pool.ID).Str("pool_name", pool.Name).Int("current_count", pool.NodeCount).Int("max", maxNodesPerPool).Msg("create: pool at max capacity, skipping")
			continue
		}
		candidates = append(candidates, i)
	}

	if len(candidates) == 0 {
		if hasTemporarySkip {
			return nil, errPoolsTemporarilyUnavailable
		}
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no eligible pools available for instance type %s", requestedType))
	}

	sort.Slice(candidates, func(a, b int) bool {
		return pools[candidates[a]].NodeCount < pools[candidates[b]].NodeCount
	})

	for _, idx := range candidates {
		pool := &pools[idx]
		if p.cooldowns.TryReserve(pool.ID) {
			return pool, nil
		}
		remaining := p.cooldowns.GetCooldownRemaining(pool.ID)
		log.Warn().Str("pool_id", pool.ID).Dur("remaining", remaining).Msg("create: pool in cooldown, skipping")
	}

	return nil, errPoolsTemporarilyUnavailable
}

func (p *CloudProvider) latestNode(ctx context.Context, poolID string, expectedCount int) (*client.WorkerNode, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		nodes, err := p.nirvanaClient.ListWorkerNodes(ctx, p.clusterID, poolID)
		if err != nil {
			lastErr = err
			log.Warn().Err(err).Int("attempt", attempt+1).Str("pool_id", poolID).Msg("create: retrying node list")
			continue
		}
		if len(nodes) < expectedCount {
			lastErr = fmt.Errorf("expected %d nodes, got %d", expectedCount, len(nodes))
			log.Warn().Int("attempt", attempt+1).Int("expected", expectedCount).Int("got", len(nodes)).Str("pool_id", poolID).Msg("create: new node not yet visible, retrying")
			continue
		}

		latest := 0
		for i, n := range nodes {
			if n.CreatedAt.After(nodes[latest].CreatedAt) {
				latest = i
			}
		}
		return &nodes[latest], nil
	}
	return nil, fmt.Errorf("listing nodes after 3 attempts: %w", lastErr)
}

func (p *CloudProvider) waitForOperation(ctx context.Context, poolID, operationID, action string) (*operations.Operation, error) {
	start := time.Now()
	op, err := p.nirvanaClient.WaitForOperation(ctx, operationID)
	duration := time.Since(start)

	p.cooldowns.RecordScaleComplete(poolID)

	if err != nil {
		log.Error().
			Err(err).
			Str("pool_id", poolID).
			Str("operation_id", operationID).
			Str("action", action).
			Dur("duration", duration).
			Msgf("%s: vm operation failed", action)
		return nil, err
	}

	log.Info().
		Str("pool_id", poolID).
		Str("operation_id", operationID).
		Str("action", action).
		Dur("duration", duration).
		Msgf("%s: vm operation complete", action)
	return op, nil
}

func instanceTypeFromRequirements(nodeClaim *karpv1.NodeClaim) string {
	for _, req := range nodeClaim.Spec.Requirements {
		if req.Key == corev1.LabelInstanceTypeStable && req.Operator == corev1.NodeSelectorOpIn && len(req.Values) > 0 {
			return req.Values[0]
		}
	}
	return ""
}

func parseProviderID(providerID string) (clusterID, poolID, nodeClaimName string, err error) {
	trimmed := strings.TrimPrefix(providerID, "nirvana://")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("expected nirvana://clusterID/poolID/name, got %s", providerID)
	}
	return parts[0], parts[1], parts[2], nil
}

func capacityFromSpec(instanceType string, specs []client.InstanceTypeSpec, bootVolumeGB int) (corev1.ResourceList, error) {
	for _, s := range specs {
		if s.Name == instanceType {
			return corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(fmt.Sprintf("%d", s.VCPU)),
				corev1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dGi", s.MemoryGB)),
				corev1.ResourceEphemeralStorage: resource.MustParse(fmt.Sprintf("%dGi", bootVolumeGB)),
				corev1.ResourcePods:             resource.MustParse("110"),
			}, nil
		}
	}
	return nil, fmt.Errorf("instance type %s not found in specs", instanceType)
}
