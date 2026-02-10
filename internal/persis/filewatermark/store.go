// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package filewatermark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const stateFileName = "state.json"

var _ exec.WatermarkStore = (*Store)(nil)

// Store implements exec.WatermarkStore using a JSON file in the scheduler data directory.
type Store struct {
	baseDir string
}

// New creates a new file-based watermark store.
// baseDir is typically filepath.Join(cfg.Paths.DataDir, "scheduler").
func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// Load reads the scheduler state from the state file.
// If the file is missing or corrupt, it returns a fresh empty state.
func (s *Store) Load(_ context.Context) (*exec.SchedulerState, error) {
	path := s.statePath()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return newEmptyState(), nil
		}
		return newEmptyState(), nil
	}

	var state exec.SchedulerState
	if err := json.Unmarshal(data, &state); err != nil {
		return newEmptyState(), nil
	}

	if state.DAGs == nil {
		state.DAGs = make(map[string]exec.DAGWatermark)
	}

	return &state, nil
}

// Save writes the scheduler state atomically (temp file + rename).
func (s *Store) Save(ctx context.Context, state *exec.SchedulerState) error {
	if err := os.MkdirAll(s.baseDir, 0o750); err != nil {
		return fmt.Errorf("failed to create watermark directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scheduler state: %w", err)
	}

	path := s.statePath()

	// Write to temp file first for atomicity
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		// Clean up temp file on rename failure
		_ = os.Remove(tmpFile)
		logger.Error(ctx, "Failed to rename watermark state file",
			tag.Error(err),
			tag.File(path),
		)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

func (s *Store) statePath() string {
	return filepath.Join(s.baseDir, stateFileName)
}

func newEmptyState() *exec.SchedulerState {
	return &exec.SchedulerState{
		Version: 1,
		DAGs:    make(map[string]exec.DAGWatermark),
	}
}
