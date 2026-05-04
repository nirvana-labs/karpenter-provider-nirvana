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
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

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

	pool := p.selectPoolForCreate(pools, requestedType)
	if pool == nil {
		log.Warn().
			Str("nodeclaim", nodeClaim.Name).
			Str("requested_instance_type", requestedType).
			Msg("create: no eligible pool found")
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no eligible pools available for instance type %s", requestedType))
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

	if err := p.waitForOperation(ctx, pool.ID, operationID, "create"); err != nil {
		return nil, fmt.Errorf("waiting for scale-up of pool %s: %w", pool.ID, err)
	}

	return &karpv1.NodeClaim{
		Status: karpv1.NodeClaimStatus{
			ProviderID:  fmt.Sprintf("nirvana://%s/%s/%s", p.clusterID, pool.ID, nodeClaim.Name),
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

	_, poolID, _, err := parseProviderID(nodeClaim.Status.ProviderID)
	if err != nil {
		return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("invalid provider id: %w", err))
	}

	log.Info().
		Str("pool_id", poolID).
		Msg("delete: parsed provider id")

	pool, err := p.nirvanaClient.GetPool(ctx, p.clusterID, poolID)
	if err != nil {
		if client.IsNotFound(err) {
			log.Warn().Str("pool_id", poolID).Msg("delete: pool not found")
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("pool %s not found", poolID))
		}
		return fmt.Errorf("getting pool %s: %w", poolID, err)
	}

	if pool.NodeCount <= minNodesPerPool {
		log.Warn().
			Str("pool_id", poolID).
			Str("pool_name", pool.Name).
			Int("current_count", pool.NodeCount).
			Int("min", minNodesPerPool).
			Msg("delete: pool at min capacity")
		return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("pool %s at minimum capacity", poolID))
	}

	if !p.cooldowns.TryReserve(poolID) {
		remaining := p.cooldowns.GetCooldownRemaining(poolID)
		log.Warn().
			Str("pool_id", poolID).
			Dur("remaining", remaining).
			Msg("delete: pool in cooldown")
		return fmt.Errorf("pool %s in cooldown for %s", poolID, remaining)
	}

	targetCount := pool.NodeCount - 1

	if err := p.nirvanaClient.CheckPoolUpdateAvailability(ctx, p.clusterID, poolID, targetCount); err != nil {
		p.cooldowns.ClearCooldown(poolID)
		log.Warn().Err(err).Str("pool_id", poolID).Int("target_count", targetCount).Msg("delete: no availability for scale-down")
		return fmt.Errorf("no availability for pool %s: %w", poolID, err)
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Int("current_count", pool.NodeCount).
		Int("target_count", targetCount).
		Msg("now deleting vm")

	operationID, err := p.nirvanaClient.UpdatePool(ctx, p.clusterID, poolID, targetCount)
	if err != nil {
		p.cooldowns.ClearCooldown(poolID)
		return fmt.Errorf("scaling down pool %s: %w", poolID, err)
	}

	log.Info().
		Str("pool_id", poolID).
		Str("operation_id", operationID).
		Msg("delete: scale-down operation submitted, waiting for completion")

	if err := p.waitForOperation(ctx, poolID, operationID, "delete"); err != nil {
		return fmt.Errorf("waiting for scale-down of pool %s: %w", poolID, err)
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

func (p *CloudProvider) selectPoolForCreate(pools []client.WorkerPool, requestedType string) *client.WorkerPool {
	candidates := make([]int, 0, len(pools))

	for i, pool := range pools {
		if pool.Status != "ready" {
			log.Debug().Str("pool_id", pool.ID).Str("status", pool.Status).Msg("create: skipping pool, not ready")
			continue
		}
		if requestedType != "" && pool.NodeConfig.InstanceType != requestedType {
			log.Debug().Str("pool_id", pool.ID).Str("pool_type", pool.NodeConfig.InstanceType).Str("requested", requestedType).Msg("create: skipping pool, instance type mismatch")
			continue
		}
		if pool.NodeCount >= maxNodesPerPool {
			log.Warn().Str("pool_id", pool.ID).Str("pool_name", pool.Name).Int("current_count", pool.NodeCount).Int("max", maxNodesPerPool).Msg("create: pool at max capacity, skipping")
			continue
		}
		candidates = append(candidates, i)
	}

	sort.Slice(candidates, func(a, b int) bool {
		return pools[candidates[a]].NodeCount < pools[candidates[b]].NodeCount
	})

	for _, idx := range candidates {
		pool := &pools[idx]
		if p.cooldowns.TryReserve(pool.ID) {
			return pool
		}
		remaining := p.cooldowns.GetCooldownRemaining(pool.ID)
		log.Warn().Str("pool_id", pool.ID).Dur("remaining", remaining).Msg("create: pool in cooldown, skipping")
	}

	return nil
}

func (p *CloudProvider) waitForOperation(ctx context.Context, poolID, operationID, action string) error {
	start := time.Now()
	err := p.nirvanaClient.WaitForOperation(ctx, operationID)
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
		return err
	}

	log.Info().
		Str("pool_id", poolID).
		Str("operation_id", operationID).
		Str("action", action).
		Dur("duration", duration).
		Msgf("%s: vm operation complete", action)
	return nil
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
