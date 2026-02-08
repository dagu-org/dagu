package scheduler

import (
	// nolint: gosec
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
)

// dagState is the per-DAG JSON schema (extensible for future fields).
type dagState struct {
	LastTick time.Time `json:"lastTick"`
}

// DAGStateStore manages per-DAG state files.
// Each DAG gets its own JSON file at {dataDir}/scheduler/dag-state/{safeName}.json.
type DAGStateStore struct {
	dir     string // {dataDir}/scheduler/dag-state/
	dagsDir string // base DAGs directory for relative path computation
}

// NewDAGStateStore creates a DAGStateStore that persists per-DAG state files.
func NewDAGStateStore(dataDir, dagsDir string) *DAGStateStore {
	return &DAGStateStore{
		dir:     filepath.Join(dataDir, "scheduler", "dag-state"),
		dagsDir: dagsDir,
	}
}

// Load reads the state for a single DAG from disk.
// Returns zero state if the file is missing or corrupt.
func (s *DAGStateStore) Load(dag *core.DAG) (dagState, error) {
	filePath := filepath.Join(s.dir, s.stateFileName(dag))

	data, err := os.ReadFile(filePath) //nolint:gosec // path derived from internal config
	if err != nil {
		if os.IsNotExist(err) {
			return dagState{}, nil
		}
		return dagState{}, fmt.Errorf("failed to read DAG state: %w", err)
	}

	var state dagState
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupt file — treat as missing
		return dagState{}, nil
	}

	return state, nil
}

// Save atomically writes the state for a single DAG to disk.
func (s *DAGStateStore) Save(dag *core.DAG, state dagState) error {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return fmt.Errorf("failed to create DAG state directory: %w", err)
	}

	filePath := filepath.Join(s.dir, s.stateFileName(dag))
	return fileutil.WriteJSONAtomic(filePath, state, 0o600)
}

// SaveAll saves the lastTick for all DAGs in bulk.
// Errors are logged but do not stop processing.
func (s *DAGStateStore) SaveAll(dags map[string]*core.DAG, tick time.Time) error {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return fmt.Errorf("failed to create DAG state directory: %w", err)
	}

	st := dagState{LastTick: tick}
	var firstErr error
	for _, dag := range dags {
		filePath := filepath.Join(s.dir, s.stateFileName(dag))
		if err := fileutil.WriteJSONAtomic(filePath, st, 0o600); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// LoadAll loads state for all provided DAGs.
func (s *DAGStateStore) LoadAll(dags map[string]*core.DAG) (map[*core.DAG]dagState, error) {
	result := make(map[*core.DAG]dagState, len(dags))
	for _, dag := range dags {
		state, err := s.Load(dag)
		if err != nil {
			return nil, fmt.Errorf("failed to load state for DAG %s: %w", dag.Name, err)
		}
		result[dag] = state
	}
	return result, nil
}

// Migrate performs a one-time migration from the old global state.json to per-DAG files.
// If the old file doesn't exist, this is a no-op.
func (s *DAGStateStore) Migrate(oldWatermarkPath string, dags map[string]*core.DAG) error {
	data, err := os.ReadFile(oldWatermarkPath) //nolint:gosec // path derived from internal config
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to migrate
		}
		return fmt.Errorf("failed to read old watermark: %w", err)
	}

	var old struct {
		LastTick time.Time `json:"lastTick"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		// Corrupt old file — remove it and skip migration
		_ = os.Remove(oldWatermarkPath)
		return nil
	}

	if old.LastTick.IsZero() {
		_ = os.Remove(oldWatermarkPath)
		return nil
	}

	// Seed each DAG with the global lastTick
	if err := s.SaveAll(dags, old.LastTick); err != nil {
		return fmt.Errorf("failed to migrate watermark to per-DAG state: %w", err)
	}

	// Remove the old global file
	if err := os.Remove(oldWatermarkPath); err != nil {
		return fmt.Errorf("failed to remove old watermark file: %w", err)
	}

	return nil
}

// stateFileName derives a safe filename from the DAG's location relative to dagsDir.
func (s *DAGStateStore) stateFileName(dag *core.DAG) string {
	relPath, err := filepath.Rel(s.dagsDir, dag.Location)
	if err != nil {
		// Fallback to using the DAG name
		relPath = dag.Name
	}

	// Strip YAML extension
	relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath))

	safe := fileutil.SafeName(relPath)
	if safe != relPath {
		// SafeName modified the string (e.g., / → _), add hash for collision safety
		h := sha256.Sum256([]byte(relPath))
		safe = safe + "-" + hex.EncodeToString(h[:])[:8]
	}

	return safe + ".json"
}
