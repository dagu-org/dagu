package scheduler

import (
	"context"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"go.uber.org/fx"
)

type Params struct {
	fx.In

	Config *config.Config
	Logger logger.Logger

	EntryReader EntryReader
}

var Module = fx.Options(
	fx.Provide(EntryReaderProvider),
	fx.Provide(New),
)

func EntryReaderProvider(cfg *config.Config) EntryReader {
	// TODO: fix this
	return NewEntryReader(cfg.DAGs, cfg)
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
