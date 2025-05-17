package localproc

import (
	"context"
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

// Count implements models.ProcStore.
func (s *Store) Count(ctx context.Context, name string) (int, error) {
	if _, ok := s.procGroups.Load(name); !ok {
		s.procGroups.Store(name, NewProcGroup(s.baseDir, name, s.staleTime))
	}
	pg, _ := s.procGroups.Load(name)
	return pg.(*ProcGroup).Count(ctx, name)
}

// Get implements models.ProcStore.
func (s *Store) Get(ctx context.Context, workflow digraph.WorkflowRef) (models.Proc, error) {
	if _, ok := s.procGroups.Load(workflow.Name); !ok {
		s.procGroups.Store(workflow.Name, NewProcGroup(s.baseDir, workflow.Name, s.staleTime))
	}
	pg, _ := s.procGroups.Load(workflow.Name)
	return pg.(*ProcGroup).GetProc(ctx, workflow)
}
