package tokensecret

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/auth"
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

var _ auth.TokenSecretProvider = (*FileProvider)(nil)

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

	fileExists := false
	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from trusted config dir + constant filename
	if err == nil {
		fileExists = true
		// File exists — check if it has usable content.
		content := strings.TrimSpace(string(data))
		if content != "" {
			return auth.NewTokenSecretFromString(content)
		}
		// Empty/whitespace-only file — treat as missing, fall through to generation.
	} else if !errors.Is(err, os.ErrNotExist) {
		// Permission error or other I/O failure — fatal, do not skip.
		return auth.TokenSecret{}, fmt.Errorf("failed to read token secret file %s: %w", path, err)
	}

	// Generate a new secret.
	secret, err := generateSecret()
	if err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to generate token secret: %w", err)
	}

	// Ensure directory exists with correct permissions.
	if err := os.MkdirAll(p.dir, dirPerm); err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to create auth directory %s: %w", p.dir, err)
	}
	if err := os.Chmod(p.dir, dirPerm); err != nil {
		return auth.TokenSecret{}, fmt.Errorf("failed to set auth directory permissions %s: %w", p.dir, err)
	}

	if fileExists {
		// File exists but is empty — safe to overwrite directly.
		if err := os.WriteFile(path, []byte(secret), filePerm); err != nil { //nolint:gosec // path is constructed from trusted config dir + constant filename
			return auth.TokenSecret{}, fmt.Errorf("failed to write token secret file %s: %w", path, err)
		}
	} else {
		// File doesn't exist — use exclusive create to prevent race conditions.
		// If another process created the file first, read their secret instead.
		if err := writeExclusive(path, []byte(secret), filePerm); err != nil {
			if errors.Is(err, os.ErrExist) {
				data, readErr := os.ReadFile(path) //nolint:gosec // path is constructed from trusted config dir + constant filename
				if readErr != nil {
					return auth.TokenSecret{}, fmt.Errorf("failed to read token secret after race: %w", readErr)
				}
				return auth.NewTokenSecretFromString(strings.TrimSpace(string(data)))
			}
			return auth.TokenSecret{}, fmt.Errorf("failed to write token secret file %s: %w", path, err)
		}
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

// writeExclusive atomically creates a file with content, failing if it already exists.
// Writes to a temp file first, then hard-links to the target path. This ensures
// that if the target file exists, it always contains complete content (no partial reads).
// Returns os.ErrExist if the file already exists (another process won the race).
func writeExclusive(path string, data []byte, perm os.FileMode) error {
	// Write full content to a unique temp file in the same directory.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".token_secret.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // Clean up temp file regardless.

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Hard-link is atomic and fails if target exists, preventing race conditions.
	if err := os.Link(tmpPath, path); err != nil {
		if os.IsExist(err) {
			return os.ErrExist
		}
		return err
	}
	return nil
}
