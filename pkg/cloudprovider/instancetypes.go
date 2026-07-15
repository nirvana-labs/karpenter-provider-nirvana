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

	// Nirvana does not expose per-instance-type pricing yet, so we synthesize a
	// relative hourly price from vCPU and memory. The absolute value is
	// meaningless; only the ordering matters — it lets Karpenter right-size by
	// preferring the smallest instance type that fits a pending pod instead of
	// treating every offering as free (Price: 0) and tie-breaking arbitrarily.
	pricePerVCPUHour   = 0.0400
	pricePerMemoryGBHr = 0.0050
)

// computePrice returns a synthetic relative price that increases monotonically
// with vCPU and memory.
func computePrice(vcpu, memoryGB int) float64 {
	return float64(vcpu)*pricePerVCPUHour + float64(memoryGB)*pricePerMemoryGBHr
}

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
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, "linux"),
			),
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
					Price:     computePrice(spec.VCPU, spec.MemoryGB),
					Available: true,
				},
			},
		}
		result = append(result, it)
	}

	return result
}
