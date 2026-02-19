package tokensecret

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
)

const (
	// secretFileName is the name of the file that stores the JWT signing secret.
	secretFileName = "token_secret"
	// secretByteLength is the number of random bytes to generate (32 bytes = 256 bits).
	secretByteLength = 32
	// dirPerm is the permission for the auth directory.
	dirPerm = 0700
	// filePerm is the permission for the secret file.
	filePerm = 0600
)

// FileProvider resolves a token secret from a file, auto-generating one if missing.
// The secret file is stored at {dir}/token_secret.
type FileProvider struct {
	dir string
}

// NewFile creates a FileProvider that reads or generates a secret in the given directory.
func NewFile(dir string) *FileProvider {
	return &FileProvider{dir: dir}
}

// Resolve reads the token secret from file, or generates and persists a new one.
//
// Error semantics:
//   - File missing or empty/whitespace-only → auto-generate, persist, and return
//   - Permission errors (read or write) → return fatal wrapped error (not ErrInvalidTokenSecret)
//   - Successfully read or generated → return valid TokenSecret
func (p *FileProvider) Resolve(_ context.Context) (auth.TokenSecret, error) {
	path := filepath.Join(p.dir, secretFileName)

	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from trusted config dir + constant filename
	if err == nil {
		// File exists — check if it has usable content.
		content := strings.TrimSpace(string(data))
		if content != "" {
			return auth.NewTokenSecretFromString(content)
		}
		// Empty/whitespace-only file — treat as missing, fall through to generation.
	} else if !os.IsNotExist(err) {
		// Permission error or other I/O failure — fatal, do not skip.
		return auth.TokenSecret{}, fmt.Errorf("failed to read token secret file %s: %w", path, err)
	}

	// Generate a new secret.
	secret, err := generateSecret()
	if err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to generate token secret: %w", err)
	}

	// Ensure directory exists.
	if err := os.MkdirAll(p.dir, dirPerm); err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to create auth directory %s: %w", p.dir, err)
	}

	// Persist atomically.
	if err := fileutil.WriteFileAtomic(path, []byte(secret), filePerm); err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to write token secret file %s: %w", path, err)
	}

	return auth.NewTokenSecretFromString(secret)
}

// generateSecret produces a cryptographically random base64url-encoded string.
// 32 bytes → 43 characters (base64 raw URL encoding, no padding).
func generateSecret() (string, error) {
	buf := make([]byte, secretByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
