package instance

import "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cooldown"

// Provider is responsible for creating, listing, and deleting instances on
// Nirvana. This is a scaffold — methods will be filled in once the API client
// is wired up.
type Provider struct {
	cooldown *cooldown.Manager
}

// New returns an instance Provider that respects pool cooldowns via cm.
func New(cm *cooldown.Manager) *Provider {
	return &Provider{cooldown: cm}
}
