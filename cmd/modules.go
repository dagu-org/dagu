package cmd

import (
	"github.com/dagu-dev/dagu/internal/client"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence"
	dsclient "github.com/dagu-dev/dagu/internal/persistence/client"
)

func newClient(cfg *config.Config, ds persistence.DataStores, lg logger.Logger) client.Client {
	return client.New(ds, cfg.Executable, cfg.WorkDir, lg)
}

func newDataStores(cfg *config.Config) persistence.DataStores {
	return dsclient.NewDataStores(
		cfg.DAGs,
		cfg.DataDir,
		cfg.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)
}
