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

// formatTaints renders taints as "key=value:Effect" (or "key:Effect" when the
// value is empty) strings, matching the Nirvana pool taint format, so log
// output lines up with the pool taints it is compared against.
func formatTaints(taints []corev1.Taint) []string {
	out := make([]string, len(taints))
	for i, t := range taints {
		if t.Value == "" {
			out[i] = fmt.Sprintf("%s:%s", t.Key, t.Effect)
		} else {
			out[i] = fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
		}
	}
	return out
}

// isHardTaint reports whether a taint actually blocks scheduling. Only
// NoSchedule and NoExecute keep an untolerating pod off a node; PreferNoSchedule
// is a soft preference the scheduler is free to override, so a pod can still be
// placed on a node carrying only PreferNoSchedule taints. We therefore ignore
// PreferNoSchedule when deciding whether a pool is safe to scale.
func isHardTaint(effect corev1.TaintEffect) bool {
	return effect == corev1.TaintEffectNoSchedule || effect == corev1.TaintEffectNoExecute
}

// poolTaintsMatch reports whether a pool whose nodes carry poolTaints is a safe
// match for a NodeClaim that expects the taint set `expected`.
//
// Only hard taints (NoSchedule / NoExecute) participate: those are the ones that
// can reject a pod, so the pool's hard taints must exactly equal the NodeClaim's
// expected hard taints. A pool carrying an unexpected hard taint would reject
// pods Karpenter believes fit (the churn bug), and a pool missing an expected
// hard taint would produce a node that violates the NodePool contract.
// PreferNoSchedule taints never block scheduling, so they are ignored on both
// sides — a pool whose only taints are PreferNoSchedule matches an untainted
// NodeClaim. Unparseable pool taints fail the match so we never scale a pool we
// can't reason about.
func poolTaintsMatch(poolTaints []string, expected []corev1.Taint) bool {
	have := make(map[[3]string]struct{}, len(poolTaints))
	for _, raw := range poolTaints {
		t, err := parsePoolTaint(raw)
		if err != nil {
			return false
		}
		if !isHardTaint(t.Effect) {
			continue
		}
		have[taintKey(t)] = struct{}{}
	}

	want := make(map[[3]string]struct{}, len(expected))
	for _, t := range expected {
		if !isHardTaint(t.Effect) {
			continue
		}
		want[taintKey(t)] = struct{}{}
	}

	if len(have) != len(want) {
		return false
	}
	for k := range have {
		if _, ok := want[k]; !ok {
			return false
		}
	}
	return true
}
