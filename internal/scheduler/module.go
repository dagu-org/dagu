package scheduler

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	dagulogger "github.com/dagu-dev/dagu/internal/logger"
)

func New(cfg *config.Config, lg dagulogger.Logger, eng engine.Engine) *Scheduler {
	return newScheduler(newSchedulerArgs{
		EntryReader: newEntryReader(newEntryReaderArgs{
			Engine:  eng,
			DagsDir: cfg.DAGs,
			JobCreator: &jobCreatorImpl{
				WorkDir:    cfg.WorkDir,
				Engine:     eng,
				Executable: cfg.Executable,
			},
			Logger: lg,
		}),
		Logger: lg,
		LogDir: cfg.LogDir,
	})
}
