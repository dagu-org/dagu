package cmd

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/frontend"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"go.uber.org/fx"
)

// frontendModule is a module for the frontend server.
var frontendModule = fx.Options(
	baseModule,
	frontend.Module,
	fx.NopLogger,
)

// baseModule is a common module for all commands.
var baseModule = fx.Options(
	fx.Provide(newLogger),
	fx.Provide(newEngine),
	fx.Provide(client.NewDataStores),
)

func newLogger(cfg *config.Config) logger.Logger {
	return logger.NewLogger(logger.NewLoggerArgs{
		Config: cfg,
	})
}

func newEngine(cfg *config.Config) engine.Engine {
	return engine.New(&engine.NewEngineArgs{
		DataStore:  newDataStores(cfg),
		Executable: cfg.Executable,
		WorkDir:    cfg.WorkDir,
	})
}

func newDataStores(cfg *config.Config) persistence.DataStores {
	return client.NewDataStores(&client.NewDataStoresArgs{
		DAGs:              cfg.DAGs,
		DataDir:           cfg.DataDir,
		SuspendFlagsDir:   cfg.SuspendFlagsDir,
		LatestStatusToday: cfg.LatestStatusToday,
	})
}
