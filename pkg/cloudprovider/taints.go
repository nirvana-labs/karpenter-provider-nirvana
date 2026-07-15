package cloudprovider

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// parsePoolTaint parses a Nirvana pool taint string into a corev1.Taint. The
// Nirvana API formats each taint as "key=value:Effect" where the value may be
// empty ("key=:Effect") or omitted entirely ("key:Effect"), and Effect is one
// of NoSchedule, PreferNoSchedule, or NoExecute.
func parsePoolTaint(s string) (corev1.Taint, error) {
	keyValue, effect, ok := strings.Cut(s, ":")
	if !ok {
		return corev1.Taint{}, fmt.Errorf("taint %q missing ':Effect' suffix", s)
	}

	switch corev1.TaintEffect(effect) {
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
	default:
		return corev1.Taint{}, fmt.Errorf("taint %q has invalid effect %q", s, effect)
	}

	key, value, _ := strings.Cut(keyValue, "=")
	if key == "" {
		return corev1.Taint{}, fmt.Errorf("taint %q has empty key", s)
	}

	return corev1.Taint{
		Key:    key,
		Value:  value,
		Effect: corev1.TaintEffect(effect),
	}, nil
}

// taintKey uniquely identifies a taint for set comparison. Kubernetes treats a
// taint's identity as (key, value, effect), so all three participate.
func taintKey(t corev1.Taint) [3]string {
	return [3]string{t.Key, t.Value, string(t.Effect)}
}

// poolTaintsMatch reports whether a pool whose nodes carry poolTaints is an
// exact match for a NodeClaim that expects the taint set `expected`.
//
// The match must be exact in both directions: a pool carrying a taint the
// NodeClaim doesn't expect would reject pods Karpenter believes fit (the churn
// bug), and a pool missing a taint the NodeClaim expects would produce a node
// that violates the NodePool contract. Unparseable pool taints fail the match
// so we never scale a pool we can't reason about.
func poolTaintsMatch(poolTaints []string, expected []corev1.Taint) bool {
	if len(poolTaints) != len(expected) {
		return false
	}

	want := make(map[[3]string]struct{}, len(expected))
	for _, t := range expected {
		want[taintKey(t)] = struct{}{}
	}

	for _, raw := range poolTaints {
		t, err := parsePoolTaint(raw)
		if err != nil {
			return false
		}
		if _, ok := want[taintKey(t)]; !ok {
			return false
		}
		delete(want, taintKey(t))
	}

	return len(want) == 0
}
