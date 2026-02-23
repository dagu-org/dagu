package license

import (
	"crypto/ed25519"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// VerifyToken parses and verifies a JWT license token using the given Ed25519 public key.
// It rejects tokens that use algorithms other than EdDSA.
func VerifyToken(pubKey ed25519.PublicKey, tokenString string) (*LicenseClaims, error) {
	claims := &LicenseClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return claims, nil
}

// VerifyTokenLenient parses a JWT license token without validating claims (e.g., expiry).
// This is used to extract claims from expired tokens for grace period evaluation.
// It still enforces EdDSA signature verification.
func VerifyTokenLenient(pubKey ed25519.PublicKey, tokenString string) (*LicenseClaims, error) {
	claims := &LicenseClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return claims, nil
}
