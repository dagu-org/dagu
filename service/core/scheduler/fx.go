package scheduler

import (
	"context"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/service/core/scheduler/entry_reader"
	"github.com/dagu-dev/dagu/service/core/scheduler/scheduler"
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
	Logger      logger.Logger
	EntryReader scheduler.EntryReader
}

func EntryReaderProvider(cfg *config.Config, jf entry_reader.JobFactory, logger logger.Logger) scheduler.EntryReader {
	return entry_reader.New(entry_reader.Params{
		DagsDir:    cfg.DAGs,
		JobFactory: jf,
		Logger:     logger,
	})
}

func JobFactoryProvider(cfg *config.Config) entry_reader.JobFactory {
	return &jobFactory{
		Command: cfg.Command,
		WorkDir: cfg.WorkDir,
	}
}

func New(params Params) *scheduler.Scheduler {
	return scheduler.New(scheduler.Params{
		EntryReader: params.EntryReader,
		Logger:      params.Logger,
		// TODO: check this is used
		LogDir: params.Config.LogDir,
	})
}

func LifetimeHooks(lc fx.Lifecycle, a *scheduler.Scheduler) {
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
