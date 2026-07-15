package cloudprovider

import (
	"reflect"
	"testing"

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
