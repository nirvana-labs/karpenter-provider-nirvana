package cooldown

import (
	"context"
	"sync"
	"time"
)

// Manager tracks pool scaling durations and enforces cooldown periods.
// After a pool finishes scaling, a cooldown of multiplier × provisioning duration
// is applied before that pool can be scaled again.
type Manager struct {
	mu          sync.RWMutex
	scaleStarts map[string]time.Time // poolID → when scale operation started
	cooldowns   map[string]time.Time // poolID → when cooldown expires
	multiplier  float64
}

// NewManager creates a new cooldown manager with the given multiplier.
// A multiplier of 2.0 means cooldown = 2× the time the scaling operation took.
func NewManager(multiplier float64) *Manager {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	return &Manager{
		scaleStarts: make(map[string]time.Time),
		cooldowns:   make(map[string]time.Time),
		multiplier:  multiplier,
	}
}

// RecordScaleStart records that a scale operation has begun for the given pool.
func (m *Manager) RecordScaleStart(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scaleStarts[poolID] = time.Now()
}

// RecordScaleComplete records that a scale operation has completed and activates cooldown.
// The cooldown duration is multiplier × (now - scaleStart).
func (m *Manager) RecordScaleComplete(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start, ok := m.scaleStarts[poolID]
	if !ok {
		return
	}

	duration := time.Since(start)
	cooldownDuration := time.Duration(float64(duration) * m.multiplier)
	m.cooldowns[poolID] = time.Now().Add(cooldownDuration)
	delete(m.scaleStarts, poolID)
}

// IsInCooldown returns true if the pool is currently in a cooldown period
// or has an in-progress scaling operation.
func (m *Manager) IsInCooldown(poolID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// A pool with an active scaling operation is treated as in cooldown.
	if _, active := m.scaleStarts[poolID]; active {
		return true
	}

	expiry, ok := m.cooldowns[poolID]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// GetCooldownRemaining returns the remaining cooldown duration for a pool.
// Returns 0 if the pool is not in cooldown.
func (m *Manager) GetCooldownRemaining(poolID string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	expiry, ok := m.cooldowns[poolID]
	if !ok {
		return 0
	}

	remaining := time.Until(expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ClearCooldown manually removes a cooldown for a pool (useful for testing or emergency).
func (m *Manager) ClearCooldown(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cooldowns, poolID)
	delete(m.scaleStarts, poolID)
}

// StartCleanup launches a background goroutine that periodically removes expired cooldown entries.
func (m *Manager) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanup()
			}
		}
	}()
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for poolID, expiry := range m.cooldowns {
		if now.After(expiry) {
			delete(m.cooldowns, poolID)
		}
	}
}
