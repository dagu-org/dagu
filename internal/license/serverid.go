package license

import (
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

	data, err := os.ReadFile(idPath) //nolint:gosec // path is constructed from trusted config dir
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate server ID: %w", err)
	}

	if err := os.WriteFile(idPath, []byte(id.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write server ID: %w", err)
	}

	return id.String(), nil
}
