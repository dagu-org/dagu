package scheduler

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	dagulogger "github.com/dagu-dev/dagu/internal/logger"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(EntryReaderProvider),
	fx.Provide(JobFactoryProvider),
	fx.Provide(New),
)

type Params struct {
	fx.In

	Config      *config.Config
	Logger      dagulogger.Logger
	EntryReader EntryReader
}

func EntryReaderProvider(
	cfg *config.Config,
	eng engine.Engine,
	jf JobFactory,
	logger dagulogger.Logger,
) EntryReader {
	return newEntryReader(newEntryReaderArgs{
		Engine:     eng,
		DagsDir:    cfg.DAGs,
		JobFactory: jf,
		Logger:     logger,
	})
}

func JobFactoryProvider(
	cfg *config.Config, eng engine.Engine,
) JobFactory {
	return &jobFactory{
		WorkDir:    cfg.WorkDir,
		Engine:     eng,
		Executable: cfg.Executable,
	}
}

func New(params Params) *Scheduler {
	return NewScheduler(NewSchedulerArgs{
		EntryReader: params.EntryReader,
		Logger:      params.Logger,
		LogDir:      params.Config.LogDir,
	})
}

func LifetimeHooks(lc fx.Lifecycle, a *Scheduler) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) (err error) {
				return a.Start()
			},
			OnStop: func(_ context.Context) error {
				a.Stop()
				return nil
			},
		},
	)
}
