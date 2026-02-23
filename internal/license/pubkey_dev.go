//go:build dev

package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
)

const devFallbackPubKeyB64 = "yILP+havpFnWixwOYuXODueHTQ5CDD7DfRxVhc/A9/Q="

// PublicKey returns the Ed25519 public key for license verification.
// In dev builds, it reads DAGU_LICENSE_PUBKEY_B64 env var with fallback to the production key.
func PublicKey() ed25519.PublicKey {
	keyB64 := os.Getenv("DAGU_LICENSE_PUBKEY_B64")
	if keyB64 == "" {
		keyB64 = devFallbackPubKeyB64
	}
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		panic("invalid license public key: " + err.Error())
	}
	return ed25519.PublicKey(raw)
}
