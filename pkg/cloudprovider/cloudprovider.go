package cloudprovider

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"

	"github.com/rs/zerolog/log"
)

const providerName = "nirvana"

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	instanceTypes []*cloudprovider.InstanceType
}

func New() *CloudProvider {
	return &CloudProvider{instanceTypes: defaultInstanceTypes()}
}

func (p *CloudProvider) Name() string {
	return providerName
}

func (p *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	log.Info().Str("nodeclaim", nodeClaim.Name).Msg("create called")

	return &karpv1.NodeClaim{
		Status: karpv1.NodeClaimStatus{
			ProviderID:  fmt.Sprintf("nirvana://%s", nodeClaim.Name),
			Capacity:    nodeClaim.Spec.Resources.Requests,
			Allocatable: nodeClaim.Spec.Resources.Requests,
		},
	}, nil
}

func (p *CloudProvider) Delete(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	log.Info().
		Str("nodeclaim", nodeClaim.Name).
		Str("provider_id", nodeClaim.Status.ProviderID).
		Msg("delete called")

	return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance terminated"))
}

func (p *CloudProvider) Get(ctx context.Context, providerID string) (*karpv1.NodeClaim, error) {
	return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
}

func (p *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	return nil, nil
}

func (p *CloudProvider) GetInstanceTypes(ctx context.Context, _ *karpv1.NodePool) ([]*cloudprovider.InstanceType, error) {
	return p.instanceTypes, nil
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

func defaultInstanceTypes() []*cloudprovider.InstanceType {
	return []*cloudprovider.InstanceType{
		{
			Name: "nirvana-4vcpu-16gi-100gi",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, "nirvana-4vcpu-16gi-100gi"),
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, "linux"),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "us-sea-1"),
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
			),
			Offerings: cloudprovider.Offerings{
				&cloudprovider.Offering{
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "us-sea-1"),
						scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
					),
					Price:     0,
					Available: true,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("4"),
				corev1.ResourceMemory:           resource.MustParse("16Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				corev1.ResourcePods:             resource.MustParse("110"),
			},
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
	}
}
