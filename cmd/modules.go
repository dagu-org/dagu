package cmd

import (
	"github.com/daguflow/dagu/internal/client"
	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/persistence"
	dsclient "github.com/daguflow/dagu/internal/persistence/client"
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
