package app

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/service/frontend"
	"go.uber.org/fx"
	"os"
)

var (
	TopLevelModule = fx.Options(
		fx.Provide(ConfigProvider),
		fx.Provide(engine.NewFactory),
		fx.Provide(logger.NewSlogLogger),
		fx.Provide(client.NewDataStoreFactory),
	)
)

var (
	cfgInstance *config.Config = nil
)

func ConfigProvider() *config.Config {
	if cfgInstance != nil {
		return cfgInstance
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	if err := config.LoadConfig(home); err != nil {
		panic(err)
	}
	cfgInstance = config.Get()
	return cfgInstance
}

func NewFrontendService() *fx.App {
	return fx.New(
		TopLevelModule,
		frontend.Module,
		fx.Invoke(frontend.LifetimeHooks),
	)
}
