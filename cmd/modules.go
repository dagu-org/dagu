package cmd

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"
)

func newEngine(cfg *config.Config, ds persistence.DataStores) engine.Engine {
	return engine.New(ds, cfg.Executable, cfg.WorkDir)
}

func newDataStores(cfg *config.Config) persistence.DataStores {
	return client.NewDataStores(
		cfg.DAGs,
		cfg.DataDir,
		cfg.SuspendFlagsDir,
		client.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)
}
