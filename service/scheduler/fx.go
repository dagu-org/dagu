package scheduler

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	dagulogger "github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/service/scheduler/entry_reader"
	"github.com/dagu-dev/dagu/service/scheduler/scheduler"
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
	EntryReader scheduler.EntryReader
}

func EntryReaderProvider(
	cfg *config.Config,
	engine engine.Engine,
	jf entry_reader.JobFactory,
	logger dagulogger.Logger,
) scheduler.EntryReader {
	loader := dag.NewLoader(cfg)
	return entry_reader.New(entry_reader.Params{
		Engine:     engine,
		DagsDir:    cfg.DAGs,
		JobFactory: jf,
		Logger:     logger,
	}, loader)
}

func JobFactoryProvider(cfg *config.Config, eng engine.Engine) entry_reader.JobFactory {
	return &jobFactory{
		WorkDir:    cfg.WorkDir,
		Engine:     eng,
		Executable: cfg.Executable,
	}
}

func New(params Params) *scheduler.Scheduler {
	return scheduler.New(scheduler.Params{
		EntryReader: params.EntryReader,
		Logger:      params.Logger,
		LogDir:      params.Config.LogDir,
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
