package fileserviceregistry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// instanceInfo represents the information stored for each service instance
type instanceInfo struct {
	ID        string                  `json:"id"`
	Host      string                  `json:"host"`
	Port      int                     `json:"port"`
	PID       int                     `json:"pid"`
	Status    exec.ServiceStatus `json:"status"`
	StartedAt time.Time               `json:"startedAt"`
}

// instanceFilePath returns the file path for an instance
func instanceFilePath(baseDir, serviceName, instanceID string) string {
	return filepath.Join(baseDir, serviceName, fmt.Sprintf("%s.json", fileutil.SafeName(instanceID)))
}

// writeInstanceFile writes instance information to a file atomically
func writeInstanceFile(filename string, info *instanceInfo) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal instance info: %w", err)
	}

	// Write atomically by writing to temp file first then renaming
	tmpFile := filename + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filename); err != nil {
		_ = os.Remove(tmpFile) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// readInstanceFile reads instance information from a file
func readInstanceFile(path string) (*instanceInfo, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read instance file: %w", err)
	}

	var info instanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instance info: %w", err)
	}

	return &info, nil
}

// removeInstanceFile removes an instance file
func removeInstanceFile(filename string) error {
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove instance file: %w", err)
	}
	return nil
}
