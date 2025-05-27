package localproc

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.ProcStore = (*Store)(nil)

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

// CountAlive implements models.ProcStore.
func (s *Store) CountAlive(ctx context.Context, name string) (int, error) {
	if _, ok := s.procGroups.Load(name); !ok {
		pgBaseDir := filepath.Join(s.baseDir, name)
		s.procGroups.Store(name, NewProcGroup(pgBaseDir, name, s.staleTime))
	}
	pg, _ := s.procGroups.Load(name)
	return pg.(*ProcGroup).Count(ctx, name)
}

// Acquire implements models.ProcStore.
func (s *Store) Acquire(ctx context.Context, dagRun digraph.DAGRunRef) (models.ProcHandle, error) {
	if _, ok := s.procGroups.Load(dagRun.Name); !ok {
		pgBaseDir := filepath.Join(s.baseDir, dagRun.Name)
		s.procGroups.Store(dagRun.Name, NewProcGroup(pgBaseDir, dagRun.Name, s.staleTime))
	}
	pg, _ := s.procGroups.Load(dagRun.Name)
	proc, err := pg.(*ProcGroup).Acquire(ctx, dagRun)
	if err != nil {
		return nil, err
	}
	if err := proc.startHeartbeat(ctx); err != nil {
		return nil, err
	}
	return proc, nil
}
