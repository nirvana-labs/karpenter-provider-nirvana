package cooldown

import (
	"testing"
	"time"
)

func TestCooldownBasicFlow(t *testing.T) {
	m := NewManager(2.0)

	m.RecordScaleStart("pool-1")
	// Simulate a 100ms scaling operation.
	time.Sleep(100 * time.Millisecond)
	m.RecordScaleComplete("pool-1")

	if !m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 to be in cooldown after scale complete")
	}

	remaining := m.GetCooldownRemaining("pool-1")
	if remaining <= 0 {
		t.Fatalf("expected positive remaining cooldown, got %s", remaining)
	}

	// With 2× multiplier on ~100ms operation, cooldown should be ~200ms.
	// Wait for it to expire.
	time.Sleep(300 * time.Millisecond)

	if m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 cooldown to have expired")
	}
}

func TestCooldownDuringActiveOperation(t *testing.T) {
	m := NewManager(2.0)

	// Pool should not be in cooldown before any operation.
	if m.IsInCooldown("pool-1") {
		t.Fatal("expected no cooldown before any operation")
	}

	// Start an operation — pool should now be in cooldown.
	m.RecordScaleStart("pool-1")

	if !m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 to be in cooldown during active operation")
	}

	// Complete the operation — should transition to time-based cooldown.
	time.Sleep(50 * time.Millisecond)
	m.RecordScaleComplete("pool-1")

	if !m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 to be in time-based cooldown after completion")
	}
}

func TestCooldownNotStarted(t *testing.T) {
	m := NewManager(2.0)

	if m.IsInCooldown("pool-unknown") {
		t.Fatal("expected no cooldown for unknown pool")
	}

	if remaining := m.GetCooldownRemaining("pool-unknown"); remaining != 0 {
		t.Fatalf("expected 0 remaining for unknown pool, got %s", remaining)
	}
}

func TestCooldownClear(t *testing.T) {
	m := NewManager(2.0)

	m.RecordScaleStart("pool-1")
	time.Sleep(50 * time.Millisecond)
	m.RecordScaleComplete("pool-1")

	if !m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 to be in cooldown")
	}

	m.ClearCooldown("pool-1")

	if m.IsInCooldown("pool-1") {
		t.Fatal("expected pool-1 cooldown to be cleared")
	}
}

func TestCooldownCompleteWithoutStart(t *testing.T) {
	m := NewManager(2.0)

	// Should be a no-op, not panic.
	m.RecordScaleComplete("pool-1")

	if m.IsInCooldown("pool-1") {
		t.Fatal("expected no cooldown when complete called without start")
	}
}

func TestCooldownDefaultMultiplier(t *testing.T) {
	m := NewManager(0) // Invalid multiplier, should default to 2.0
	if m.multiplier != 2.0 {
		t.Fatalf("expected default multiplier 2.0, got %f", m.multiplier)
	}

	m = NewManager(-1) // Negative, should default to 2.0
	if m.multiplier != 2.0 {
		t.Fatalf("expected default multiplier 2.0, got %f", m.multiplier)
	}
}
