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
)

const providerName = "nirvana"

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	nirvanaClient *client.Client
	clusterID     string
	region        string
}

func New(nirvanaClient *client.Client, clusterID, region string) *CloudProvider {
	return &CloudProvider{
		nirvanaClient: nirvanaClient,
		clusterID:     clusterID,
		region:        region,
	}
}

func (p *CloudProvider) Name() string {
	return providerName
}

func (p *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	candidates := instanceTypeValues(nodeClaim)

	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Strs("candidate_instance_types", candidates).
		Msg("create: received nodeclaim")

	pools, err := p.nirvanaClient.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	specs, err := p.nirvanaClient.ListInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instance types: %w", err)
	}

	// Honor Karpenter's contract of launching the cheapest compatible offering,
	// ordered cheapest-first so an unavailable cheaper type falls back to a
	// costlier one that has a ready pool.
	requestedTypes := rankInstanceTypesByCost(candidates, specs)

	log.Info().
		Int("pool_count", len(pools)).
		Strs("requested_instance_types", requestedTypes).
		Msg("create: fetched pools")

	pool, err := p.selectPoolForCreate(ctx, pools, requestedTypes, nodeClaim.Spec.Taints)
	if err != nil {
		log.Warn().Err(err).
			Str("nodeclaim", nodeClaim.Name).
			Strs("requested_instance_types", requestedTypes).
			Msg("create: no eligible pool found")
		return nil, err
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Str("instance_type", pool.NodeConfig.InstanceType).
		Int("current_count", pool.NodeCount).
		Msg("create: selected pool")

	capacity, err := capacityFromSpec(pool.NodeConfig.InstanceType, specs, pool.NodeConfig.BootVolume.Size)
	if err != nil {
		return nil, fmt.Errorf("resolving capacity for pool %s: %w", pool.ID, err)
	}

	// Availability was already confirmed by selectPoolForCreate.
	targetCount := pool.NodeCount + 1

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Int("current_count", pool.NodeCount).
		Int("target_count", targetCount).
		Msg("now creating vm")

	operationID, err := p.nirvanaClient.UpdatePool(ctx, p.clusterID, pool.ID, targetCount)
	if err != nil {
		if client.IsConflict(err) {
			log.Warn().Str("pool_id", pool.ID).Msg("create: pool has active operation, retrying")
			return nil, errPoolsTemporarilyUnavailable
		}
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
				corev1.LabelArchStable:         "amd64",
				corev1.LabelOSStable:           "linux",
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

	log.Info().
		Str("pool_id", poolID).
		Str("node_id", nodeID).
		Msg("now deleting vm")

	operationID, err := p.nirvanaClient.DeleteWorkerNode(ctx, p.clusterID, poolID, nodeID)
	if err != nil {
		if client.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("node %s not found: %w", nodeID, err))
		}
		if client.IsConflict(err) {
			return fmt.Errorf("pool %s has active operation, retrying: %w", poolID, err)
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
	_, poolID, nodeID, err := parseProviderID(providerID)
	if err != nil {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("invalid provider id: %w", err))
	}

	nodes, err := p.nirvanaClient.ListWorkerNodes(ctx, p.clusterID, poolID)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("pool %s not found: %w", poolID, err))
		}
		return nil, fmt.Errorf("listing nodes for pool %s: %w", poolID, err)
	}

	var found *client.WorkerNode
	for i, n := range nodes {
		if n.ID == nodeID {
			found = &nodes[i]
			break
		}
	}
	if found == nil || found.Status == "deleting" || found.Status == "error" {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("node %s not found in pool %s", nodeID, poolID))
	}

	return &karpv1.NodeClaim{
		Status: karpv1.NodeClaimStatus{
			ProviderID: providerID,
		},
	}, nil
}

func (p *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	pools, err := p.nirvanaClient.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	var claims []*karpv1.NodeClaim
	for _, pool := range pools {
		nodes, err := p.nirvanaClient.ListWorkerNodes(ctx, p.clusterID, pool.ID)
		if err != nil {
			return nil, fmt.Errorf("listing nodes for pool %s: %w", pool.ID, err)
		}

		for _, node := range nodes {
			if node.Status == "deleting" || node.Status == "error" {
				continue
			}
			claims = append(claims, &karpv1.NodeClaim{
				Status: karpv1.NodeClaimStatus{
					ProviderID: fmt.Sprintf("nirvana://%s/%s/%s", p.clusterID, pool.ID, node.ID),
				},
			})
		}
	}

	return claims, nil
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

// selectPoolForCreate returns a pool that is ready, taint-matched, and
// confirmed to have scale-up availability, for the cheapest eligible instance
// type. requestedTypes is ordered cheapest-first; a pool that is not ready,
// whose taints don't match, or whose availability check is rejected advances to
// the next pool or instance type, so an unavailable cheaper option never blocks
// a costlier one that can be launched. An empty requestedTypes means
// unconstrained — any ready pool is eligible.
func (p *CloudProvider) selectPoolForCreate(ctx context.Context, pools []client.WorkerPool, requestedTypes []string, expectedTaints []corev1.Taint) (*client.WorkerPool, error) {
	candidates, hasTemporarySkip := eligiblePoolsInCostOrder(pools, requestedTypes, expectedTaints)

	var lastAvailErr error
	for _, i := range candidates {
		pool := &pools[i]
		targetCount := pool.NodeCount + 1
		if err := p.nirvanaClient.CheckPoolUpdateAvailability(ctx, p.clusterID, pool.ID, targetCount); err != nil {
			log.Warn().Err(err).Str("pool_id", pool.ID).Int("target_count", targetCount).Msg("create: no availability, trying next pool")
			lastAvailErr = err
			continue
		}
		return pool, nil
	}

	if lastAvailErr != nil {
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no pool with available capacity for instance types %v: %w", requestedTypes, lastAvailErr))
	}
	if hasTemporarySkip {
		return nil, errPoolsTemporarilyUnavailable
	}
	return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no eligible pools available for instance types %v", requestedTypes))
}

// eligiblePoolsInCostOrder returns the indices of pools that match an allowed
// instance type, carry exactly the taints the NodeClaim expects, and are ready
// — grouped by requestedTypes (which the caller orders cheapest-first) and,
// within a type, ordered by fewest nodes. An empty requestedTypes means
// unconstrained: any ready, taint-matched pool is eligible. hasTemporarySkip
// reports whether a pool was skipped only because it was not yet ready, so the
// caller can distinguish "retry later" from "no eligible pool".
func eligiblePoolsInCostOrder(pools []client.WorkerPool, requestedTypes []string, expectedTaints []corev1.Taint) (ordered []int, hasTemporarySkip bool) {
	order := requestedTypes
	if len(order) == 0 {
		order = []string{""} // unconstrained: single match-any pass
	}

	for _, requestedType := range order {
		candidates := make([]int, 0, len(pools))
		for i, pool := range pools {
			if requestedType != "" && pool.NodeConfig.InstanceType != requestedType {
				log.Debug().Str("pool_id", pool.ID).Str("pool_type", pool.NodeConfig.InstanceType).Str("requested", requestedType).Msg("create: skipping pool, instance type mismatch")
				continue
			}
			// Only scale a pool whose taints exactly match what the NodeClaim
			// expects. Karpenter's scheduler already decided this pod tolerates
			// the NodeClaim's taints; scaling a pool with a different taint set
			// would produce a node the pod can't schedule onto, which Karpenter
			// then tears down and retries forever.
			if !poolTaintsMatch(pool.NodeConfig.Taints, expectedTaints) {
				log.Debug().Str("pool_id", pool.ID).Strs("pool_taints", pool.NodeConfig.Taints).Msg("create: skipping pool, taint mismatch")
				continue
			}
			if pool.Status != "ready" {
				log.Debug().Str("pool_id", pool.ID).Str("status", pool.Status).Msg("create: skipping pool, not ready")
				hasTemporarySkip = true
				continue
			}
			candidates = append(candidates, i)
		}

		sort.Slice(candidates, func(a, b int) bool {
			return pools[candidates[a]].NodeCount < pools[candidates[b]].NodeCount
		})
		ordered = append(ordered, candidates...)
	}

	return ordered, hasTemporarySkip
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

// instanceTypeValues returns the instance types the NodeClaim is constrained to,
// or nil when it is unconstrained.
func instanceTypeValues(nodeClaim *karpv1.NodeClaim) []string {
	for _, req := range nodeClaim.Spec.Requirements {
		if req.Key == corev1.LabelInstanceTypeStable && req.Operator == corev1.NodeSelectorOpIn {
			return req.Values
		}
	}
	return nil
}

// rankInstanceTypesByCost orders candidates cheapest-first. Candidates with a
// known spec sort by price ahead of those without; unknown-spec candidates keep
// their original relative order and trail the priced ones, so a constrained
// NodeClaim never widens to matching any pool. Returns nil when candidates is
// empty (unconstrained — any pool is eligible).
func rankInstanceTypesByCost(candidates []string, specs []client.InstanceTypeSpec) []string {
	if len(candidates) == 0 {
		return nil
	}

	priceByName := make(map[string]float64, len(specs))
	for _, s := range specs {
		priceByName[s.Name] = computePrice(s.VCPU, s.MemoryGB)
	}

	ranked := append([]string(nil), candidates...)
	sort.SliceStable(ranked, func(a, b int) bool {
		priceA, okA := priceByName[ranked[a]]
		priceB, okB := priceByName[ranked[b]]
		if okA != okB {
			return okA // priced candidates sort ahead of unknown ones
		}
		if !okA {
			return false // both unknown: preserve original order
		}
		return priceA < priceB
	})
	return ranked
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
