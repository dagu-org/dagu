package prototype

import (
	"context"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueueStorage = (*Storage)(nil)

type Storage struct {
	baseDir string
	// queueFiles is a map of queue files, where the key is the workflow name
	// and the value is a slice of queue files indexed by priority
	queueFiles map[string][]*QueueFile
	mu         sync.Mutex
}

// Dequeue implements models.QueueStorage.
func (s *Storage) Dequeue(ctx context.Context, name string, id string) (models.QueuedItem, error) {
	panic("unimplemented")
}

// Enqueue implements models.QueueStorage.
func (s *Storage) Enqueue(ctx context.Context, p models.QueuePriority, name string, workflow digraph.WorkflowRef) error {
	panic("unimplemented")
}

func New(baseDir string) models.QueueStorage {
	return &Storage{
		baseDir:    baseDir,
		queueFiles: make(map[string][]*QueueFile),
	}
}
