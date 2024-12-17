// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
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
