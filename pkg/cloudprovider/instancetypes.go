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
			// Karpenter's InstanceType contract requires instance type, zone, capacity
			// type, and architecture to be declared here. The instance type entry is
			// load-bearing: core filters instance types with Requirements.Intersects,
			// which only compares keys present on both sides, so omitting the key makes
			// every instance type compatible with every NodePool. Core then overwrites
			// the NodeClaim's instance-type requirement with the full surviving list in
			// NodeClaimTemplate.ToNodeClaim, discarding the constraint the NodePool (and
			// any pod nodeSelector) asked for, and Create() scales whichever pool is
			// cheapest instead of the one the pod needs.
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, name),
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, "linux"),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, region),
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
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
