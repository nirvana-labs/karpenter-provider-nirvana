package cloudprovider

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
)

const (
	// System overhead reserved for kubelet and OS.
	overheadCPUMillis = 100
	overheadMemoryMiB = 256
)

// InstanceTypeName returns the canonical instance type name for a pool's node config.
func InstanceTypeName(cfg client.NodeConfig) string {
	return fmt.Sprintf("nirvana-%dvcpu-%dgi-%dgi", cfg.CPUConfig.VCPU, cfg.MemoryConfig.Size, cfg.BootVolume.Size)
}

// PoolsToInstanceTypes converts a set of pools into deduplicated Karpenter InstanceTypes.
func PoolsToInstanceTypes(pools []client.WorkerPool, region string) []*cloudprovider.InstanceType {
	seen := make(map[string]bool)
	var result []*cloudprovider.InstanceType

	for _, pool := range pools {
		name := InstanceTypeName(pool.NodeConfig)
		if seen[name] {
			continue
		}
		seen[name] = true

		it := &cloudprovider.InstanceType{
			Name: name,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(fmt.Sprintf("%d", pool.NodeConfig.CPUConfig.VCPU)),
				corev1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dGi", pool.NodeConfig.MemoryConfig.Size)),
				corev1.ResourceEphemeralStorage: resource.MustParse(fmt.Sprintf("%dGi", pool.NodeConfig.BootVolume.Size)),
				corev1.ResourcePods:             resource.MustParse("110"),
			},
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", overheadCPUMillis)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", overheadMemoryMiB)),
				},
			},
			Offerings: cloudprovider.Offerings{
				&cloudprovider.Offering{
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
						scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, region),
					),
					Price:     0, // Nirvana does not expose pricing through the API.
					Available: true,
				},
			},
		}
		result = append(result, it)
	}

	return result
}
