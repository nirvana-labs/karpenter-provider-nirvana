package cloudprovider

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
)

func specMap() map[string]client.InstanceTypeSpec {
	return map[string]client.InstanceTypeSpec{
		"n1-highcpu-2":  {Name: "n1-highcpu-2", VCPU: 2, MemoryGB: 4},
		"n1-highcpu-16": {Name: "n1-highcpu-16", VCPU: 16, MemoryGB: 32},
	}
}

func pool(name, instanceType string) client.WorkerPool {
	return client.WorkerPool{
		ID:   name,
		Name: name,
		NodeConfig: client.NodeConfig{
			InstanceType: instanceType,
			BootVolume:   client.BootVolume{Size: 50},
		},
	}
}

// A smaller instance type must be cheaper than a larger one so Karpenter
// right-sizes toward the smallest fitting node instead of tie-breaking on the
// string ordering ("16" < "2") that previously picked the larger pool.
func TestComputePriceOrdersBySize(t *testing.T) {
	small := computePrice(2, 4)
	large := computePrice(16, 32)
	if !(small < large) {
		t.Fatalf("expected smaller instance to be cheaper: small=%v large=%v", small, large)
	}
	if small <= 0 {
		t.Fatalf("expected a positive price, got %v", small)
	}
}

func TestPoolsToInstanceTypesSetsNonZeroPrice(t *testing.T) {
	pools := []client.WorkerPool{pool("p1", "n1-highcpu-2")}
	its := PoolsToInstanceTypes(pools, specMap(), "us-sva-1")
	if len(its) != 1 {
		t.Fatalf("expected 1 instance type, got %d", len(its))
	}
	if got := its[0].Offerings[0].Price; got <= 0 {
		t.Fatalf("expected non-zero price, got %v", got)
	}
}

// Karpenter's InstanceType contract requires instance type, zone, capacity type,
// and architecture to be declared in Requirements.
func TestPoolsToInstanceTypesDeclaresWellKnownRequirements(t *testing.T) {
	its := PoolsToInstanceTypes([]client.WorkerPool{pool("p1", "n1-highcpu-2")}, specMap(), "us-sva-2")
	if len(its) != 1 {
		t.Fatalf("expected 1 instance type, got %d", len(its))
	}

	for key, want := range map[string]string{
		corev1.LabelInstanceTypeStable: "n1-highcpu-2",
		corev1.LabelArchStable:         "amd64",
		corev1.LabelOSStable:           "linux",
		corev1.LabelTopologyZone:       "us-sva-2",
		karpv1.CapacityTypeLabelKey:    karpv1.CapacityTypeOnDemand,
	} {
		requirement := its[0].Requirements.Get(key)
		if requirement.Len() != 1 || !requirement.Has(want) {
			t.Errorf("requirement %q: expected exactly [%s], got %v", key, want, requirement.Values())
		}
	}
}

// An instance type must be rejected by a NodeClaim constrained to a different
// instance type. This mirrors karpenter core's compatible() check, which is
// Requirements.Intersects and therefore silently passes any key the instance type
// does not declare. When n1-highcpu-2 survived filtering for an n1-standard-2
// NodePool, core replaced the NodeClaim's instance-type requirement with the whole
// surviving list and Create() scaled the cheapest pool — so a pod pinned to
// n1-standard-2 got an n1-highcpu-2 node and never scheduled.
func TestPoolsToInstanceTypesRejectsMismatchedInstanceType(t *testing.T) {
	specs := map[string]client.InstanceTypeSpec{
		"n1-highcpu-2":  {Name: "n1-highcpu-2", VCPU: 2, MemoryGB: 4},
		"n1-standard-2": {Name: "n1-standard-2", VCPU: 2, MemoryGB: 8},
	}
	its := PoolsToInstanceTypes([]client.WorkerPool{
		pool("cheap", "n1-highcpu-2"),
		pool("wanted", "n1-standard-2"),
	}, specs, "us-sva-2")
	if len(its) != 2 {
		t.Fatalf("expected 2 instance types, got %d", len(its))
	}

	// What a NodeClaim for the n1-standard-2 NodePool carries, after a pod with
	// nodeSelector node.kubernetes.io/instance-type=n1-standard-2 is added.
	nodeClaim := scheduling.NewRequirements(
		scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, "n1-standard-2"),
		scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
		scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, "linux"),
		scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "us-sva-2"),
		scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
	)

	for _, it := range its {
		compatible := it.Requirements.Intersects(nodeClaim) == nil
		want := it.Name == "n1-standard-2"
		if compatible != want {
			t.Errorf("instance type %s: compatible=%v, want %v", it.Name, compatible, want)
		}
	}
}

// A pod constrained to a zone the provider does not serve must not match, so
// Karpenter reports an unsatisfiable pod instead of launching in the wrong zone.
func TestPoolsToInstanceTypesRejectsMismatchedZone(t *testing.T) {
	its := PoolsToInstanceTypes([]client.WorkerPool{pool("p1", "n1-highcpu-2")}, specMap(), "us-sva-2")
	nodeClaim := scheduling.NewRequirements(
		scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "us-sva-1"),
	)
	if its[0].Requirements.Intersects(nodeClaim) == nil {
		t.Error("expected instance type in us-sva-2 to be incompatible with a us-sva-1 NodeClaim")
	}
}

func TestRankInstanceTypesByCost(t *testing.T) {
	specs := []client.InstanceTypeSpec{
		{Name: "n1-highcpu-2", VCPU: 2, MemoryGB: 4},
		{Name: "n1-highcpu-16", VCPU: 16, MemoryGB: 32},
	}

	// Cheapest-first ordering: "16" is listed before "2" but must rank after it.
	if got := rankInstanceTypesByCost([]string{"n1-highcpu-16", "n1-highcpu-2"}, specs); !reflect.DeepEqual(got, []string{"n1-highcpu-2", "n1-highcpu-16"}) {
		t.Errorf("expected cheapest-first ordering, got %v", got)
	}
	if got := rankInstanceTypesByCost(nil, specs); got != nil {
		t.Errorf("expected nil for unconstrained, got %v", got)
	}
	// Unknown-spec candidates trail priced ones rather than widening to match
	// any pool.
	if got := rankInstanceTypesByCost([]string{"unknown", "n1-highcpu-2"}, specs); !reflect.DeepEqual(got, []string{"n1-highcpu-2", "unknown"}) {
		t.Errorf("expected priced candidate ahead of unknown, got %v", got)
	}
}
