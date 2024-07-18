package scheduler

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	dagulogger "github.com/dagu-dev/dagu/internal/logger"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(New),
)

type Params struct {
	fx.In

	Config *config.Config
	Logger dagulogger.Logger
	Engine engine.Engine
}

func New(params Params) *Scheduler {
	return newScheduler(newSchedulerArgs{
		EntryReader: newEntryReader(newEntryReaderArgs{
			Engine:  params.Engine,
			DagsDir: params.Config.DAGs,
			JobCreator: &jobCreatorImpl{
				WorkDir:    params.Config.WorkDir,
				Engine:     params.Engine,
				Executable: params.Config.Executable,
			},
			Logger: params.Logger,
		}),
		Logger: params.Logger,
		LogDir: params.Config.LogDir,
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
