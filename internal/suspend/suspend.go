package suspend

import (
	"fmt"

	"github.com/yohamta/dagu/internal/config"
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

func (s *SuspendChecker) ToggleSuspend(cfg *config.DAG, suspend bool) error {
	if suspend {
		return s.storage.Create(fileName(cfg))
	} else if s.IsSuspended(cfg) {
		return s.storage.Delete(fileName(cfg))
	}
	return nil
}

func (s *SuspendChecker) IsSuspended(cfg *config.DAG) bool {
	return s.storage.Exists(fileName(cfg))
}

func fileName(cfg *config.DAG) string {
	return fmt.Sprintf("%s.suspend", utils.ValidFilename(cfg.Name, "-"))
}
