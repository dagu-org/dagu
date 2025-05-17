package localproc

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.ProcStorage = (*Storage)(nil)

// Storage is a struct that implements the ProcStorage interface.
type Storage struct {
	baseDir   string
	staleTime time.Duration
}

// New creates a new instance of Storage with the specified base directory.
func New(baseDir string) *Storage {
	return &Storage{
		baseDir:   baseDir,
		staleTime: time.Second * 45,
	}
}

// Count implements models.ProcStorage.
func (s *Storage) Count(ctx context.Context, name string) (int, error) {
	panic("unimplemented")
}

// Get implements models.ProcStorage.
func (s *Storage) Get(ctx context.Context, workflow digraph.WorkflowRef) (models.Proc, error) {
	panic("unimplemented")
}
