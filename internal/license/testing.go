package license

import (
	"crypto/ed25519"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// NewTestManager creates a Manager pre-loaded with the given features for use in tests.
// It generates an ephemeral ed25519 key pair, signs a JWT with the requested features,
// and updates the manager's internal state so Checker() returns a licensed state.
func NewTestManager(features ...string) *Manager {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic("ed25519.GenerateKey: " + err.Error())
	}

	claims := &LicenseClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "dagu-test",
			Subject:   "test-license",
		},
		ClaimsVersion: 1,
		Plan:          "pro",
		Features:      features,
		ActivationID:  "act-test",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(priv)
	if err != nil {
		panic("jwt sign: " + err.Error())
	}

	m := NewManager(ManagerConfig{}, pub, nil, slog.Default())
	m.state.Update(claims, signed)
	return m
}
