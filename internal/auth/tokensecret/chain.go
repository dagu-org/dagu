package tokensecret

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/auth"
)

// ChainProvider tries multiple TokenSecretProviders in priority order.
//
// Error semantics:
//   - ErrInvalidTokenSecret from a provider → skip to next provider
//   - Any other error → return immediately (fatal, e.g., permission denied)
//   - All providers exhausted → return ErrInvalidTokenSecret
type ChainProvider struct {
	providers []auth.TokenSecretProvider
}

// NewChain creates a ChainProvider that tries providers in the given order.
func NewChain(providers ...auth.TokenSecretProvider) *ChainProvider {
	return &ChainProvider{providers: providers}
}

// Resolve tries each provider in order and returns the first valid secret.
func (c *ChainProvider) Resolve(ctx context.Context) (auth.TokenSecret, error) {
	for _, p := range c.providers {
		ts, err := p.Resolve(ctx)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidTokenSecret) {
				continue // Skip to next provider.
			}
			return auth.TokenSecret{}, err // Fatal error — stop.
		}
		return ts, nil
	}
	return auth.TokenSecret{}, auth.ErrInvalidTokenSecret
}
