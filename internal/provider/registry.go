package provider

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// Registry maps remote URL host patterns to PipelineProvider implementations.
type Registry struct {
	entries []entry
}

type entry struct {
	host     string
	provider domain.PipelineProvider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register associates a host pattern (e.g., "github.com") with a provider.
func (r *Registry) Register(host string, p domain.PipelineProvider) {
	r.entries = append(r.entries, entry{host: host, provider: p})
}

// Detect returns the provider matching the host in the given remote URL.
// Returns an error if no matching provider is registered.
func (r *Registry) Detect(remoteURL string) (domain.PipelineProvider, error) {
	for _, e := range r.entries {
		if strings.Contains(remoteURL, e.host) {
			return e.provider, nil
		}
	}
	return nil, fmt.Errorf("no provider found for remote: %s", remoteURL)
}
