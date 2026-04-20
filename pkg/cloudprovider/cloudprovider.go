package cloudprovider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/awslabs/operatorpkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	nirvanaclient "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/providers/instance"
)

const providerName = "nirvana"

// CloudProvider implements the Karpenter CloudProvider interface for Nirvana NKS.
type CloudProvider struct {
	kubeClient       client.Client
	instanceProvider *instance.Provider
}

// New creates a new Nirvana CloudProvider.
func New(kubeClient client.Client, instanceProvider *instance.Provider) *CloudProvider {
	return &CloudProvider{
		kubeClient:       kubeClient,
		instanceProvider: instanceProvider,
	}
}

// Name returns the cloud provider name.
func (p *CloudProvider) Name() string {
	return providerName
}

// Create provisions a new node by scaling up the pool that most closely
// matches the NodeClaim's resource requirements.
func (p *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	nodeClass, err := p.resolveNodeClass(ctx, nodeClaim)
	if err != nil {
		return nil, fmt.Errorf("resolving node class: %w", err)
	}

	// Idempotency check: if a node already exists for this NodeClaim, return it.
	if existing, err := p.findExistingNodeForClaim(ctx, nodeClaim, nodeClass.Spec.ClusterID); err != nil {
		log.Error().Err(err).Msg("failed idempotency check, proceeding with creation")
	} else if existing != nil {
		log.Info().
			Str("provider_id", existing.Status.ProviderID).
			Msg("found existing node for NodeClaim, returning it (idempotent create)")
		return existing, nil
	}

	var selectorTags []string
	var selectorPoolIDs []string
	if nodeClass.Spec.PoolSelector != nil {
		selectorTags = nodeClass.Spec.PoolSelector.Tags
		selectorPoolIDs = nodeClass.Spec.PoolSelector.PoolIDs
	}

	requests := resourceRequestsFromNodeClaim(nodeClaim)

	pool, err := p.instanceProvider.SelectPool(ctx, selectorTags, selectorPoolIDs, requests)
	if err != nil {
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("selecting pool: %w", err))
	}

	log.Info().
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Int("current_count", pool.NodeCount).
		Msg("scaling up pool")

	newNode, err := p.instanceProvider.ScaleUp(ctx, pool)
	if err != nil {
		if nirvanaclient.IsAvailabilityRejection(err) {
			return nil, cloudprovider.NewInsufficientCapacityError(
				fmt.Errorf("pool %s rejected scale-up: %w", pool.ID, err),
			)
		}
		return nil, fmt.Errorf("scaling up pool %s: %w", pool.ID, err)
	}

	log.Info().
		Str("node_id", newNode.ID).
		Str("pool_id", pool.ID).
		Msg("new node provisioned")

	return hydrateNodeClaim(nodeClaim, nodeClass.Spec.ClusterID, pool, newNode), nil
}

// resourceRequestsFromNodeClaim extracts CPU, memory, and storage requirements
// from a NodeClaim and converts them to integer values for pool matching.
func resourceRequestsFromNodeClaim(nodeClaim *karpv1.NodeClaim) instance.ResourceRequests {
	var requests instance.ResourceRequests

	if nodeClaim.Spec.Resources.Requests != nil {
		if cpu, ok := nodeClaim.Spec.Resources.Requests[corev1.ResourceCPU]; ok {
			// Convert millicores to whole vCPUs, rounding up.
			requests.VCPU = int((cpu.MilliValue() + 999) / 1000)
		}
		if mem, ok := nodeClaim.Spec.Resources.Requests[corev1.ResourceMemory]; ok {
			// Convert bytes to GiB, rounding up.
			const giB = 1024 * 1024 * 1024
			requests.RAMGi = int((mem.Value() + giB - 1) / giB)
		}
		if storage, ok := nodeClaim.Spec.Resources.Requests[corev1.ResourceEphemeralStorage]; ok {
			const giB = 1024 * 1024 * 1024
			requests.StorageGi = int((storage.Value() + giB - 1) / giB)
		}
	}

	return requests
}

// Delete removes a specific worker node by its provider ID.
// If the node is already gone (404), it returns NodeClaimNotFoundError so
// Karpenter can clean up the NodeClaim without error.
func (p *CloudProvider) Delete(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	_, poolID, nodeID, err := parseProviderID(nodeClaim.Status.ProviderID)
	if err != nil {
		return fmt.Errorf("parsing provider ID: %w", err)
	}

	canScale, err := p.instanceProvider.CanScaleDown(ctx, poolID)
	if err != nil {
		return fmt.Errorf("checking scale-down eligibility for pool %s: %w", poolID, err)
	}
	if !canScale {
		return fmt.Errorf("cannot delete node from pool %s: already at minimum node count", poolID)
	}

	log.Info().
		Str("pool_id", poolID).
		Str("node_id", nodeID).
		Msg("deleting worker node")

	if err := p.instanceProvider.DeleteNodeByID(ctx, poolID, nodeID); err != nil {
		if nirvanaclient.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(
				fmt.Errorf("worker node %s not found (already deleted): %w", nodeID, err),
			)
		}
		return fmt.Errorf("deleting worker node %s from pool %s: %w", nodeID, poolID, err)
	}

	return nil
}

// Get retrieves a NodeClaim by its provider ID.
func (p *CloudProvider) Get(ctx context.Context, providerID string) (*karpv1.NodeClaim, error) {
	clusterID, poolID, workerNodeID, err := parseProviderID(providerID)
	if err != nil {
		return nil, fmt.Errorf("parsing provider ID: %w", err)
	}

	pool, err := p.instanceProvider.GetPool(ctx, poolID)
	if err != nil {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("pool %s not found: %w", poolID, err))
	}

	nodes, err := p.instanceProvider.ListNodes(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("listing nodes in pool %s: %w", poolID, err)
	}

	for _, node := range nodes {
		if node.ID == workerNodeID {
			nc := &karpv1.NodeClaim{}
			return hydrateNodeClaim(nc, clusterID, pool, &node), nil
		}
	}

	return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("worker node %s not found in pool %s", workerNodeID, poolID))
}

// List returns all NodeClaims across all pools in the cluster.
func (p *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	pools, err := p.instanceProvider.ListPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	var claims []*karpv1.NodeClaim
	for _, pool := range pools {
		nodes, err := p.instanceProvider.ListNodes(ctx, pool.ID)
		if err != nil {
			return nil, fmt.Errorf("listing nodes in pool %s: %w", pool.ID, err)
		}

		for _, node := range nodes {
			nc := &karpv1.NodeClaim{}
			claims = append(claims, hydrateNodeClaim(nc, pool.ClusterID, &pool, &node))
		}
	}

	return claims, nil
}

// GetInstanceTypes returns available instance types derived from pool configurations.
func (p *CloudProvider) GetInstanceTypes(ctx context.Context, _ *karpv1.NodePool) ([]*cloudprovider.InstanceType, error) {
	pools, err := p.instanceProvider.ListPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing pools for instance types: %w", err)
	}

	// TODO: Get the actual region from the cluster API. For now use a placeholder.
	region := "us-sea-1"
	_ = region

	return PoolsToInstanceTypes(pools, region), nil
}

// GetSupportedNodeClasses returns the NirvanaNodeClass type.
func (p *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1alpha1.NirvanaNodeClass{}}
}

// IsDrifted checks if a NodeClaim has drifted from its pool configuration.
func (p *CloudProvider) IsDrifted(ctx context.Context, nodeClaim *karpv1.NodeClaim) (cloudprovider.DriftReason, error) {
	_, poolID, _, err := parseProviderID(nodeClaim.Status.ProviderID)
	if err != nil {
		return "", fmt.Errorf("parsing provider ID: %w", err)
	}

	pool, err := p.instanceProvider.GetPool(ctx, poolID)
	if err != nil {
		return "", fmt.Errorf("getting pool %s: %w", poolID, err)
	}

	currentInstanceType := InstanceTypeName(pool.NodeConfig)
	claimInstanceType := nodeClaim.Labels[corev1.LabelInstanceTypeStable]

	if claimInstanceType != "" && claimInstanceType != currentInstanceType {
		return "PoolConfigChanged", nil
	}

	return "", nil
}

// RepairPolicies returns health check policies for Nirvana nodes.
// Tolerations are set to 30 minutes to accommodate NKS node boot times.
func (p *CloudProvider) RepairPolicies() []cloudprovider.RepairPolicy {
	return []cloudprovider.RepairPolicy{
		{
			ConditionType:      "Ready",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
		{
			ConditionType:      "Ready",
			ConditionStatus:    corev1.ConditionUnknown,
			TolerationDuration: 30 * time.Minute,
		},
	}
}

// findExistingNodeForClaim searches all pools for a worker node whose name
// matches the NodeClaim name. This enables idempotent creation: if Karpenter
// retries a Create after a transient failure, we return the already-created node.
func (p *CloudProvider) findExistingNodeForClaim(ctx context.Context, nodeClaim *karpv1.NodeClaim, clusterID string) (*karpv1.NodeClaim, error) {
	pools, err := p.instanceProvider.ListPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	for _, pool := range pools {
		nodes, err := p.instanceProvider.ListNodes(ctx, pool.ID)
		if err != nil {
			return nil, fmt.Errorf("listing nodes in pool %s: %w", pool.ID, err)
		}

		for _, node := range nodes {
			if node.Name == nodeClaim.Name {
				nc := &karpv1.NodeClaim{}
				return hydrateNodeClaim(nc, clusterID, &pool, &node), nil
			}
		}
	}

	return nil, nil
}

// resolveNodeClass reads the NirvanaNodeClass referenced by the NodeClaim.
func (p *CloudProvider) resolveNodeClass(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*v1alpha1.NirvanaNodeClass, error) {
	nodeClassRef := nodeClaim.Spec.NodeClassRef
	if nodeClassRef == nil {
		return nil, fmt.Errorf("nodeClaim %s has no nodeClassRef", nodeClaim.Name)
	}

	nodeClass := &v1alpha1.NirvanaNodeClass{}
	if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: nodeClassRef.Name}, nodeClass); err != nil {
		return nil, fmt.Errorf("getting NirvanaNodeClass %s: %w", nodeClassRef.Name, err)
	}

	return nodeClass, nil
}

// Provider ID format: nirvana://{clusterID}/{poolID}/{workerNodeID}

func buildProviderID(clusterID, poolID, workerNodeID string) string {
	return fmt.Sprintf("nirvana://%s/%s/%s", clusterID, poolID, workerNodeID)
}

func parseProviderID(providerID string) (clusterID, poolID, workerNodeID string, err error) {
	trimmed := strings.TrimPrefix(providerID, "nirvana://")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid provider ID format %q: expected nirvana://clusterID/poolID/workerNodeID", providerID)
	}
	return parts[0], parts[1], parts[2], nil
}

func hydrateNodeClaim(
	nc *karpv1.NodeClaim,
	clusterID string,
	pool *nirvanaclient.WorkerPool,
	node *nirvanaclient.WorkerNode,
) *karpv1.NodeClaim {
	nc.Status.ProviderID = buildProviderID(clusterID, pool.ID, node.ID)

	if nc.Labels == nil {
		nc.Labels = make(map[string]string)
	}
	nc.Labels[corev1.LabelInstanceTypeStable] = InstanceTypeName(pool.NodeConfig)
	nc.Labels[karpv1.CapacityTypeLabelKey] = karpv1.CapacityTypeOnDemand

	nc.Status.Capacity = corev1.ResourceList{
		corev1.ResourceCPU:              resource.MustParse(fmt.Sprintf("%d", pool.NodeConfig.CPUConfig.VCPU)),
		corev1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dGi", pool.NodeConfig.MemoryConfig.Size)),
		corev1.ResourceEphemeralStorage: resource.MustParse(fmt.Sprintf("%dGi", pool.NodeConfig.BootVolume.Size)),
		corev1.ResourcePods:             resource.MustParse("110"),
	}

	return nc
}
