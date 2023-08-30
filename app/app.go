package app

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/service/frontend"
	"go.uber.org/fx"
	"os"
)

var (
	CommonModule = fx.Options(
		fx.Provide(ConfigProvider),
		fx.Provide(logger.NewSlogLogger),
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
	// TODO: fixme
	cfgInstance = config.Get()
	return cfgInstance
}

func NewFrontendService() *fx.App {
	return fx.New(
		CommonModule,
		frontend.Module,
		fx.Invoke(frontend.LifetimeHooks),
	)
}
