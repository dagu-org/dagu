package scheduler

import (
	"context"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/service/scheduler/entry"
	"go.uber.org/fx"
	"time"
)

var Module = fx.Options(
	fx.Provide(EntryReaderProvider),
	fx.Provide(JobFactoryProvider),
	fx.Provide(New),
)

type Params struct {
	fx.In

	Config *config.Config
	Logger logger.Logger

	EntryReader EntryReader
}

type EntryReader interface {
	Read(now time.Time) ([]*entry.Entry, error)
}

func EntryReaderProvider(cfg *config.Config, jf entry.JobFactory) EntryReader {
	// TODO: fix this
	return entry.NewEntryReader(cfg.DAGs, cfg, jf)
}

func JobFactoryProvider(cfg *config.Config) entry.JobFactory {
	return &jobFactory{
		cfg: cfg,
	}
}

func New(params Params) *Scheduler {
	return &Scheduler{
		entryReader: params.EntryReader,
		cfg:         params.Config,
		stop:        make(chan struct{}),
		running:     false,
	}
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
