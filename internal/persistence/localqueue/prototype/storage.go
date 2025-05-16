package prototype

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueueStorage = (*Storage)(nil)

type Storage struct {
	baseDir string
}

// Dequeue implements models.QueueStorage.
func (s *Storage) Dequeue(ctx context.Context, name string, id string) (models.QueuedItem, error) {
	panic("unimplemented")
}

// Enqueue implements models.QueueStorage.
func (s *Storage) Enqueue(ctx context.Context, name digraph.WorkflowRef, workflow digraph.WorkflowRef) error {
	panic("unimplemented")
}

func New(baseDir string) models.QueueStorage {
	return &Storage{
		baseDir: baseDir,
	}
}
