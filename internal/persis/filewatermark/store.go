package filewatermark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/scheduler"
)

const stateFileName = "state.json"

var _ scheduler.WatermarkStore = (*Store)(nil)

// Store implements scheduler.WatermarkStore using a JSON file in the scheduler data directory.
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
func (s *Store) Load(ctx context.Context) (*scheduler.SchedulerState, error) {
	path := s.statePath()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn(ctx, "Failed to read watermark state file, starting fresh",
				tag.Error(err), tag.File(path))
		}
		return newEmptyState(), nil
	}

	var state scheduler.SchedulerState
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Warn(ctx, "Corrupt watermark state file, starting fresh",
			tag.Error(err), tag.File(path))
		return newEmptyState(), nil
	}

	const expectedVersion = 1
	if state.Version != 0 && state.Version != expectedVersion {
		logger.Warn(ctx, "Watermark state version mismatch, starting fresh",
			tag.File(path),
			slog.Int("version", state.Version),
		)
		return newEmptyState(), nil
	}
	if state.Version == 0 {
		state.Version = expectedVersion
	}

	if state.DAGs == nil {
		state.DAGs = make(map[string]scheduler.DAGWatermark)
	}

	return &state, nil
}

// Save writes the scheduler state atomically (temp file + rename).
func (s *Store) Save(ctx context.Context, state *scheduler.SchedulerState) error {
	if err := os.MkdirAll(s.baseDir, 0o750); err != nil {
		return fmt.Errorf("failed to create watermark directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scheduler state: %w", err)
	}

	path := s.statePath()

	// Write to temp file with fsync for durability, then rename for atomicity
	tmpFile := path + ".tmp"
	if err := writeFileSync(tmpFile, data, 0o600); err != nil {
		_ = os.Remove(tmpFile)
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

// writeFileSync writes data to a file and calls fsync before closing.
func writeFileSync(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filepath.Clean(name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) //nolint:gosec // name is constructed internally from baseDir + ".tmp"
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func newEmptyState() *scheduler.SchedulerState {
	return &scheduler.SchedulerState{
		Version: 1,
		DAGs:    make(map[string]scheduler.DAGWatermark),
	}
}
