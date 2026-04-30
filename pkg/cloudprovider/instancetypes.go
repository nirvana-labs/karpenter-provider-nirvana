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
	overheadCPUMillis = 100
	overheadMemoryMiB = 256
)

func PoolsToInstanceTypes(pools []client.WorkerPool, instanceTypeSpecs map[string]client.InstanceTypeSpec, region string) []*cloudprovider.InstanceType {
	seen := make(map[string]bool)
	var result []*cloudprovider.InstanceType

	for _, pool := range pools {
		name := pool.NodeConfig.InstanceType
		if seen[name] {
			continue
		}
		seen[name] = true

		spec, ok := instanceTypeSpecs[name]
		if !ok {
			continue
		}

		it := &cloudprovider.InstanceType{
			Name: name,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(fmt.Sprintf("%d", spec.VCPU)),
				corev1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dGi", spec.MemoryGB)),
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
					Price:     0,
					Available: true,
				},
			},
		}
		result = append(result, it)
	}

	return result
}
