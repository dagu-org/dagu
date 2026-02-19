package tokensecret

import (
	"context"

	"github.com/dagu-org/dagu/internal/auth"
)

// StaticProvider resolves a token secret from a pre-configured string value.
// It performs no I/O and has no side effects.
type StaticProvider struct {
	secret auth.TokenSecret
}

// NewStatic creates a StaticProvider from a raw string.
// Returns an error if the string is empty (cannot construct a valid TokenSecret).
func NewStatic(raw string) (*StaticProvider, error) {
	ts, err := auth.NewTokenSecretFromString(raw)
	if err != nil {
		return nil, err
	}
	return &StaticProvider{secret: ts}, nil
}

// Resolve returns the pre-configured token secret.
func (p *StaticProvider) Resolve(_ context.Context) (auth.TokenSecret, error) {
	return p.secret, nil
}
