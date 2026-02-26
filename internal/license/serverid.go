package license

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// GetOrCreateServerID reads the server ID from the license directory.
// If the file does not exist, it generates a new UUID v7 and persists it.
func GetOrCreateServerID(licenseDir string) (string, error) {
	if err := os.MkdirAll(licenseDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create license directory: %w", err)
	}

	idPath := filepath.Join(licenseDir, "server_id")

	// Attempt atomic creation to avoid TOCTOU race.
	f, err := os.OpenFile(idPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) //nolint:gosec // path is constructed from trusted config dir
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// File already exists — read it.
			data, readErr := os.ReadFile(idPath) //nolint:gosec // path is constructed from trusted config dir
			if readErr != nil {
				return "", fmt.Errorf("failed to read server ID: %w", readErr)
			}
			id := strings.TrimSpace(string(data))
			if id != "" {
				if _, parseErr := uuid.Parse(id); parseErr != nil {
					return regenerateServerID(idPath)
				}
				return id, nil
			}
			// File exists but is empty — regenerate.
			return regenerateServerID(idPath)
		}
		return "", fmt.Errorf("failed to create server ID file: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to generate server ID: %w", err)
	}

	if _, err := f.WriteString(id.String()); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to write server ID: %w", err)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close server ID file: %w", err)
	}

	return id.String(), nil
}

// regenerateServerID generates a new server ID and writes it to the given path,
// overwriting any existing content. Used when the file exists but is empty or
// contains invalid (non-UUID) data.
func regenerateServerID(idPath string) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate server ID: %w", err)
	}

	dir := filepath.Dir(idPath)
	tmp, err := os.CreateTemp(dir, ".server_id-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup on error

	if _, err := tmp.WriteString(id.String()); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to write server ID: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("failed to sync server ID file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("failed to close server ID file: %w", err)
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		return "", fmt.Errorf("failed to set permissions on server ID file: %w", err)
	}
	if err := os.Rename(tmpName, idPath); err != nil {
		return "", fmt.Errorf("failed to rename server ID file: %w", err)
	}

	return id.String(), nil
}
