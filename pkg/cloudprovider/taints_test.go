package cloudprovider

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
)

func poolWithTaints(name, instanceType string, taints []string) client.WorkerPool {
	p := pool(name, instanceType)
	p.Status = "ready"
	p.NodeConfig.Taints = taints
	return p
}

func TestParsePoolTaint(t *testing.T) {
	cases := []struct {
		in      string
		want    corev1.Taint
		wantErr bool
	}{
		{in: "dedicated=gpu:NoSchedule", want: corev1.Taint{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}},
		{in: "key=:NoExecute", want: corev1.Taint{Key: "key", Value: "", Effect: corev1.TaintEffectNoExecute}},
		{in: "key:PreferNoSchedule", want: corev1.Taint{Key: "key", Value: "", Effect: corev1.TaintEffectPreferNoSchedule}},
		{in: "no-effect", wantErr: true},
		{in: "key=value:Bogus", wantErr: true},
		{in: "=value:NoSchedule", wantErr: true},
	}
	for _, c := range cases {
		got, err := parsePoolTaint(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePoolTaint(%q): expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePoolTaint(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parsePoolTaint(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestPoolTaintsMatch(t *testing.T) {
	gpu := corev1.Taint{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}

	// Untainted pool matches a NodeClaim that expects no taints.
	if !poolTaintsMatch(nil, nil) {
		t.Error("empty pool should match empty expectation")
	}
	// Exact match regardless of order.
	if !poolTaintsMatch([]string{"dedicated=gpu:NoSchedule"}, []corev1.Taint{gpu}) {
		t.Error("matching taint set should match")
	}
	// A tainted pool must NOT match a NodeClaim expecting no taints — this is the
	// churn bug: Karpenter would place an untolerating pod here.
	if poolTaintsMatch([]string{"dedicated=gpu:NoSchedule"}, nil) {
		t.Error("tainted pool must not match empty expectation")
	}
	// A NodeClaim expecting a taint must not match an untainted pool.
	if poolTaintsMatch(nil, []corev1.Taint{gpu}) {
		t.Error("untainted pool must not match taint expectation")
	}
	// Value/effect differences are not a match.
	if poolTaintsMatch([]string{"dedicated=gpu:NoExecute"}, []corev1.Taint{gpu}) {
		t.Error("differing effect must not match")
	}
	// Unparseable pool taints fail closed.
	if poolTaintsMatch([]string{"garbage"}, nil) {
		t.Error("unparseable taint must not match")
	}
	// PreferNoSchedule is soft — it never blocks scheduling — so a pool whose
	// only taint is PreferNoSchedule matches a NodeClaim that expects no taints.
	if !poolTaintsMatch([]string{"dedicated=gpu:PreferNoSchedule"}, nil) {
		t.Error("PreferNoSchedule-only pool should match empty expectation")
	}
	// A hard taint alongside a soft one still blocks: only the hard taint counts,
	// and it doesn't match an empty expectation.
	if poolTaintsMatch([]string{"soft=x:PreferNoSchedule", "dedicated=gpu:NoSchedule"}, nil) {
		t.Error("hard taint must still block even when a soft taint is present")
	}
	// A PreferNoSchedule taint is ignored, so it doesn't need to be mirrored on
	// the pool to match a NodeClaim that carries one.
	softExpect := corev1.Taint{Key: "soft", Value: "x", Effect: corev1.TaintEffectPreferNoSchedule}
	if !poolTaintsMatch([]string{"dedicated=gpu:NoSchedule"}, []corev1.Taint{softExpect, gpu}) {
		t.Error("PreferNoSchedule in expectation should be ignored when matching hard taints")
	}
}

func TestEligiblePoolsInCostOrderTaintAware(t *testing.T) {
	gpuTaint := corev1.Taint{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}
	requested := []string{"n1-highcpu-16"}

	tainted := poolWithTaints("tainted", "n1-highcpu-16", []string{"dedicated=gpu:NoSchedule"})
	untainted := poolWithTaints("untainted", "n1-highcpu-16", nil)
	pools := []client.WorkerPool{tainted, untainted}

	// A NodeClaim with no taints must land on the untainted pool, never the
	// tainted one — even though both match the instance type.
	ordered, _ := eligiblePoolsInCostOrder(pools, requested, nil)
	if len(ordered) != 1 || pools[ordered[0]].ID != "untainted" {
		t.Errorf("expected only untainted pool eligible, got %v", poolIDs(pools, ordered))
	}

	// A NodeClaim that expects the gpu taint must land on the tainted pool.
	ordered, _ = eligiblePoolsInCostOrder(pools, requested, []corev1.Taint{gpuTaint})
	if len(ordered) != 1 || pools[ordered[0]].ID != "tainted" {
		t.Errorf("expected only tainted pool eligible, got %v", poolIDs(pools, ordered))
	}

	// If the only instance-type match carries a taint the NodeClaim doesn't
	// expect, there is no eligible pool rather than a wrong-taint scale-up.
	if ordered, _ := eligiblePoolsInCostOrder([]client.WorkerPool{tainted}, requested, nil); len(ordered) != 0 {
		t.Errorf("expected no eligible pool when taints don't match, got %v", poolIDs([]client.WorkerPool{tainted}, ordered))
	}
}

// The cheapest allowed instance type must not hide an eligible pool of another
// allowed type: here the cheaper n1-highcpu-2 has no ready pool, but the
// allowed n1-highcpu-16 does, so selection must fall through to it instead of
// returning nothing. requestedTypes is cheapest-first, so the ready pricier
// pool is the only eligible one.
func TestEligiblePoolsInCostOrderFallsThroughToEligibleType(t *testing.T) {
	cheapNotReady := poolWithTaints("cheap-not-ready", "n1-highcpu-2", nil)
	cheapNotReady.Status = "provisioning"
	pricierReady := poolWithTaints("pricier-ready", "n1-highcpu-16", nil)
	pools := []client.WorkerPool{cheapNotReady, pricierReady}

	ordered, hasTemporarySkip := eligiblePoolsInCostOrder(pools, []string{"n1-highcpu-2", "n1-highcpu-16"}, nil)
	if len(ordered) != 1 || pools[ordered[0]].ID != "pricier-ready" {
		t.Errorf("expected fallthrough to pricier-ready pool, got %v", poolIDs(pools, ordered))
	}
	if !hasTemporarySkip {
		t.Error("expected hasTemporarySkip=true for the not-ready cheaper pool")
	}
}

func poolIDs(pools []client.WorkerPool, idx []int) []string {
	ids := make([]string, 0, len(idx))
	for _, i := range idx {
		ids = append(ids, pools[i].ID)
	}
	return ids
}
