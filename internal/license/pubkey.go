//go:build !dev

package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

const prodPubKeyB64 = "yILP+havpFnWixwOYuXODueHTQ5CDD7DfRxVhc/A9/Q="

// PublicKey returns the production Ed25519 public key for license verification.
func PublicKey() (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(prodPubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded license public key: %w", err)
	}
	return ed25519.PublicKey(raw), nil
}
