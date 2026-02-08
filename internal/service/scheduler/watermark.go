package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WatermarkStore persists the scheduler's last-tick timestamp to disk.
// This allows the catch-up engine to determine how long the scheduler was offline.
type WatermarkStore struct {
	filePath string
}

type watermarkState struct {
	LastTick time.Time `json:"lastTick"`
}

// NewWatermarkStore creates a WatermarkStore that persists state at the given path.
func NewWatermarkStore(dataDir string) *WatermarkStore {
	return &WatermarkStore{
		filePath: filepath.Join(dataDir, "scheduler", "state.json"),
	}
}

// Load reads the last-tick timestamp from disk.
// Returns zero time if the file is missing or corrupt.
func (w *WatermarkStore) Load() (time.Time, error) {
	data, err := os.ReadFile(w.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to read watermark: %w", err)
	}

	var state watermarkState
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupt file — treat as missing
		return time.Time{}, nil
	}

	return state.LastTick, nil
}

// Save atomically writes the last-tick timestamp to disk.
func (w *WatermarkStore) Save(t time.Time) error {
	dir := filepath.Dir(w.filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create watermark directory: %w", err)
	}

	state := watermarkState{LastTick: t}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal watermark: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := w.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write watermark temp file: %w", err)
	}
	if err := os.Rename(tmpPath, w.filePath); err != nil {
		return fmt.Errorf("failed to rename watermark file: %w", err)
	}

	return nil
}
