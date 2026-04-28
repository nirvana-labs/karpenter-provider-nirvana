package cooldown

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Manager tracks pool scaling durations and enforces cooldown periods.
// After a pool finishes scaling, a cooldown of multiplier × provisioning
// duration is applied before that pool can be scaled again.
type Manager struct {
	mu          sync.RWMutex
	scaleStarts map[string]time.Time
	cooldowns   map[string]time.Time
	multiplier  float64
}

// NewManager creates a cooldown manager with the given multiplier.
// A non-positive multiplier defaults to 2.0.
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

// RecordScaleStart marks the beginning of a scale operation for poolID.
func (m *Manager) RecordScaleStart(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scaleStarts[poolID] = time.Now()
}

// RecordScaleComplete marks the scale operation done and starts the cooldown
// timer at multiplier × elapsed.
func (m *Manager) RecordScaleComplete(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start, ok := m.scaleStarts[poolID]
	if !ok {
		return
	}

	elapsed := time.Since(start)
	m.cooldowns[poolID] = time.Now().Add(time.Duration(float64(elapsed) * m.multiplier))
	delete(m.scaleStarts, poolID)
}

// IsInCooldown reports whether poolID has an active scale operation or an
// unexpired cooldown.
func (m *Manager) IsInCooldown(poolID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, active := m.scaleStarts[poolID]; active {
		return true
	}
	expiry, ok := m.cooldowns[poolID]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// GetCooldownRemaining returns the remaining cooldown for poolID, or 0.
func (m *Manager) GetCooldownRemaining(poolID string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	expiry, ok := m.cooldowns[poolID]
	if !ok {
		return 0
	}
	if remaining := time.Until(expiry); remaining > 0 {
		return remaining
	}
	return 0
}

// ClearCooldown drops any tracking for poolID.
func (m *Manager) ClearCooldown(poolID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cooldowns, poolID)
	delete(m.scaleStarts, poolID)
}

// StartCleanup runs a background goroutine that evicts expired cooldown
// entries every interval until ctx is done.
func (m *Manager) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("cooldown cleanup goroutine stopped")
				return
			case <-ticker.C:
				if removed := m.cleanup(); removed > 0 {
					log.Debug().Int("removed", removed).Msg("cooldown cleanup evicted expired entries")
				}
			}
		}
	}()
}

func (m *Manager) cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0
	for poolID, expiry := range m.cooldowns {
		if now.After(expiry) {
			delete(m.cooldowns, poolID)
			removed++
		}
	}
	return removed
}
