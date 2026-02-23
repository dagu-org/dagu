//go:build !dev

package license

import (
	"crypto/ed25519"
	"encoding/base64"
)

const prodPubKeyB64 = "yILP+havpFnWixwOYuXODueHTQ5CDD7DfRxVhc/A9/Q="

// PublicKey returns the production Ed25519 public key for license verification.
func PublicKey() ed25519.PublicKey {
	raw, err := base64.StdEncoding.DecodeString(prodPubKeyB64)
	if err != nil {
		panic("invalid embedded license public key: " + err.Error())
	}
	return ed25519.PublicKey(raw)
}
