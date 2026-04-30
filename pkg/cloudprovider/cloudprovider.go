package cloudprovider

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	"github.com/rs/zerolog/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

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
	pools, err := p.nirvanaClient.ListPools(ctx, p.clusterID)
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	pool := selectPool(pools)
	if pool == nil {
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("no eligible pools available"))
	}

	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Str("pool_id", pool.ID).
		Str("pool_name", pool.Name).
		Str("instance_type", pool.NodeConfig.InstanceType).
		Int("current_node_count", pool.NodeCount).
		Int("boot_volume_gb", pool.NodeConfig.BootVolume.Size).
		Msg("create called — would scale up this pool")

	return &karpv1.NodeClaim{
		Status: karpv1.NodeClaimStatus{
			ProviderID:  fmt.Sprintf("nirvana://%s/%s/%s", p.clusterID, pool.ID, nodeClaim.Name),
			Capacity:    nodeClaim.Spec.Resources.Requests,
			Allocatable: nodeClaim.Spec.Resources.Requests,
		},
	}, nil
}

func (p *CloudProvider) Delete(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Str("provider_id", nodeClaim.Status.ProviderID).
		Msg("delete called — would scale down pool")

	return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance terminated"))
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

func selectPool(pools []client.WorkerPool) *client.WorkerPool {
	for i, pool := range pools {
		if pool.Status == "ready" {
			return &pools[i]
		}
	}
	return nil
}
