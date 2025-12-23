package agents

import (
	"github.com/andrew/ai-cli-server/internal/config"
)

// Registry manages provider instances
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a provider registry from config
func NewRegistry(cfg *config.Config, providerFactories map[string]func(*config.Config) Provider) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}

	for name, factory := range providerFactories {
		provider := factory(cfg)
		if provider.IsAvailable() {
			r.providers[name] = provider
		}
	}

	return r
}

// Get returns a provider by name
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// GetAll returns all registered providers
func (r *Registry) GetAll() map[string]Provider {
	return r.providers
}

// Available returns names of available providers
func (r *Registry) Available() []string {
	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// IsAvailable checks if a provider is registered and available
func (r *Registry) IsAvailable(name string) bool {
	_, ok := r.providers[name]
	return ok
}
