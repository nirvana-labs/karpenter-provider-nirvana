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
}

func TestSelectPoolForCreateTaintAware(t *testing.T) {
	p := &CloudProvider{}
	gpuTaint := corev1.Taint{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}

	tainted := poolWithTaints("tainted", "n1-highcpu-16", []string{"dedicated=gpu:NoSchedule"})
	untainted := poolWithTaints("untainted", "n1-highcpu-16", nil)
	pools := []client.WorkerPool{tainted, untainted}

	// A NodeClaim with no taints must land on the untainted pool, never the
	// tainted one — even though both match the instance type.
	got, err := p.selectPoolForCreate(pools, "n1-highcpu-16", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "untainted" {
		t.Errorf("expected untainted pool, got %q", got.ID)
	}

	// A NodeClaim that expects the gpu taint must land on the tainted pool.
	got, err = p.selectPoolForCreate(pools, "n1-highcpu-16", []corev1.Taint{gpuTaint})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "tainted" {
		t.Errorf("expected tainted pool, got %q", got.ID)
	}

	// If the only instance-type match carries a taint the NodeClaim doesn't
	// expect, there is no eligible pool rather than a wrong-taint scale-up.
	if _, err := p.selectPoolForCreate([]client.WorkerPool{tainted}, "n1-highcpu-16", nil); err == nil {
		t.Error("expected error when no pool matches the expected taints")
	}
}
