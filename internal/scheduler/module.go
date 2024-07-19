package scheduler

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	dagulogger "github.com/dagu-dev/dagu/internal/logger"
	"go.uber.org/fx"
)

func New(
	config *config.Config,
	logger dagulogger.Logger,
	engine engine.Engine,
) *Scheduler {
	return newScheduler(newSchedulerArgs{
		EntryReader: newEntryReader(newEntryReaderArgs{
			Engine:  engine,
			DagsDir: config.DAGs,
			JobCreator: &jobCreatorImpl{
				WorkDir:    config.WorkDir,
				Engine:     engine,
				Executable: config.Executable,
			},
			Logger: logger,
		}),
		Logger: logger,
		LogDir: config.LogDir,
	})
}

func LifetimeHooks(lc fx.Lifecycle, a *Scheduler) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) (err error) {
				return a.Start(ctx)
			},
			OnStop: func(_ context.Context) error {
				a.Stop()
				return nil
			},
		},
	)
}
