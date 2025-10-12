package fileproc

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ execution.ProcStore = (*Store)(nil)

// Store is a struct that implements the ProcStore interface.
type Store struct {
	baseDir    string
	staleTime  time.Duration
	procGroups sync.Map
}

// New creates a new instance of Store with the specified base directory.
func New(baseDir string) *Store {
	return &Store{
		baseDir:   baseDir,
		staleTime: time.Second * 45,
	}
}

// Lock locks process group
func (s *Store) TryLock(_ context.Context, groupName string) error {
	procGroup := s.newProcGroup(groupName)
	return procGroup.TryLock()
}

// Lock locks process group
func (s *Store) Unlock(ctx context.Context, groupName string) {
	procGroup := s.newProcGroup(groupName)
	if err := procGroup.Unlock(); err != nil {
		logger.Error(ctx, "Failed to unlock the proc group", "err", err)
	}
}

// CountAlive implements models.ProcStore.
func (s *Store) CountAlive(ctx context.Context, groupName string) (int, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.Count(ctx)
}

func (s *Store) CountAliveByDAGName(ctx context.Context, groupName, dagName string) (int, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.CountByDAGName(ctx, dagName)
}

// ListAlive implements models.ProcStore.
func (s *Store) ListAlive(ctx context.Context, groupName string) ([]core.DAGRunRef, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.ListAlive(ctx)
}

// Acquire implements models.ProcStore.
func (s *Store) Acquire(ctx context.Context, groupName string, dagRun core.DAGRunRef) (execution.ProcHandle, error) {
	procGroup := s.newProcGroup(groupName)
	proc, err := procGroup.Acquire(ctx, dagRun)
	if err != nil {
		return nil, err
	}
	if err := proc.startHeartbeat(ctx); err != nil {
		return nil, err
	}
	return proc, nil
}

// IsRunAlive implements models.ProcStore.
func (s *Store) IsRunAlive(ctx context.Context, groupName string, dagRun core.DAGRunRef) (bool, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.IsRunAlive(ctx, dagRun)
}

// ListAllAlive implements models.ProcStore.
// Returns all running DAG runs across all process groups.
func (s *Store) ListAllAlive(ctx context.Context) (map[string][]core.DAGRunRef, error) {
	result := make(map[string][]core.DAGRunRef)

	// Create base directory if it doesn't exist
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return result, nil // No processes if directory doesn't exist
	}

	// Read all directories in the base directory - each directory is a process group
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		groupName := entry.Name()
		procGroup := s.newProcGroup(groupName)

		// Get all alive processes for this group
		aliveRuns, err := procGroup.ListAlive(ctx)
		if err != nil {
			logger.Warn(ctx, "Failed to list alive processes for group", "group", groupName, "err", err)
			continue
		}

		if len(aliveRuns) > 0 {
			result[groupName] = aliveRuns
		}
	}

	return result, nil
}

func (s *Store) newProcGroup(groupName string) *ProcGroup {
	// Check if the ProcGroup already exists
	if pg, ok := s.procGroups.Load(groupName); ok {
		return pg.(*ProcGroup)
	}

	// Create a new ProcGroup only if it doesn't exist
	pgBaseDir := filepath.Join(s.baseDir, groupName)
	newPG := NewProcGroup(pgBaseDir, groupName, s.staleTime)
	pg, _ := s.procGroups.LoadOrStore(groupName, newPG)
	return pg.(*ProcGroup)
}
