package cmd

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"
)

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
