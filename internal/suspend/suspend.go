package suspend

import (
	"fmt"

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/utils"
)

type SuspendChecker struct {
	storage *storage.Storage
}

func NewSuspendChecker(s *storage.Storage) *SuspendChecker {
	return &SuspendChecker{
		storage: s,
	}
}

func (s *SuspendChecker) ToggleSuspend(d *dag.DAG, suspend bool) error {
	if suspend {
		return s.storage.Create(fileName(d))
	} else if s.IsSuspended(d) {
		return s.storage.Delete(fileName(d))
	}
	return nil
}

func (s *SuspendChecker) IsSuspended(d *dag.DAG) bool {
	return s.storage.Exists(fileName(d))
}

func fileName(d *dag.DAG) string {
	return fmt.Sprintf("%s.suspend", utils.ValidFilename(d.Name, "-"))
}
