package auth

import (
	"context"
	"errors"
)

// ErrInvalidTokenSecret indicates the token secret is empty or otherwise unusable.
// Used by TokenSecretProvider implementations to signal that the next provider
// in a chain should be tried.
var ErrInvalidTokenSecret = errors.New("invalid token secret")

// TokenSecret is an opaque handle to JWT signing key material.
// The zero value is invalid, forcing callers through constructors.
type TokenSecret struct {
	key []byte // unexported: prevents direct access
}

// NewTokenSecret creates a TokenSecret from raw key bytes.
// Returns ErrInvalidTokenSecret if key is nil or empty.
// The input is defensively copied to prevent mutation after construction.
func NewTokenSecret(key []byte) (TokenSecret, error) {
	if len(key) == 0 {
		return TokenSecret{}, ErrInvalidTokenSecret
	}
	cp := make([]byte, len(key))
	copy(cp, key)
	return TokenSecret{key: cp}, nil
}

// NewTokenSecretFromString creates a TokenSecret from a string value.
// Returns ErrInvalidTokenSecret if the string is empty.
func NewTokenSecretFromString(s string) (TokenSecret, error) {
	if s == "" {
		return TokenSecret{}, ErrInvalidTokenSecret
	}
	return NewTokenSecret([]byte(s))
}

// SigningKey returns the raw key material for JWT signing.
// This is the ONLY way to access the key bytes.
func (ts TokenSecret) SigningKey() []byte {
	return ts.key
}

// IsValid reports whether the TokenSecret holds usable key material.
func (ts TokenSecret) IsValid() bool {
	return len(ts.key) > 0
}

// String implements fmt.Stringer. Always returns "[REDACTED]" to prevent
// accidental logging of secret material.
func (ts TokenSecret) String() string {
	return "[REDACTED]"
}

// GoString implements fmt.GoStringer. Returns a safe representation for %#v.
func (ts TokenSecret) GoString() string {
	return "auth.TokenSecret{[REDACTED]}"
}

// MarshalJSON prevents the secret from being serialized into JSON.
func (ts TokenSecret) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// MarshalText prevents the secret from being serialized as text.
func (ts TokenSecret) MarshalText() ([]byte, error) {
	return []byte("[REDACTED]"), nil
}

// TokenSecretProvider resolves the JWT signing secret from a configured source.
// Implementations may perform I/O (file reads, auto-generation) and should
// respect context cancellation.
type TokenSecretProvider interface {
	Resolve(ctx context.Context) (TokenSecret, error)
}
