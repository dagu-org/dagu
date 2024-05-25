package cmd

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/service/frontend"
	"go.uber.org/fx"
)

var (
	topLevelModule = fx.Options(
		fx.Provide(config.Get),
		fx.Provide(engine.NewFactory),
		fx.Provide(logger.NewSlogLogger),
		fx.Provide(client.NewDataStoreFactory),
	)
)

func newFrontend() *fx.App {
	return fx.New(
		topLevelModule,
		frontend.Module,
		fx.Invoke(frontend.LifetimeHooks),
		fx.NopLogger,
	)
}
