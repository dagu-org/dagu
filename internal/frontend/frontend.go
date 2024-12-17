// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/dag"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/logger"
)

func New(cfg *config.Config, lg logger.Logger, cli client.Client) *server.Server {
	var hs []server.Handler

	hs = append(hs, dag.NewHandler(
		&dag.NewHandlerArgs{
			Client:             cli,
			LogEncodingCharset: cfg.LogEncodingCharset,
			RemoteNodes:        cfg.RemoteNodes,
			ApiBasePath:        cfg.APIBaseURL,
		},
	))

	var remoteNodes []string
	for _, n := range cfg.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	serverParams := server.NewServerArgs{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		TLS:                   cfg.TLS,
		Logger:                lg,
		Handlers:              hs,
		AssetsFS:              assetsFS,
		NavbarColor:           cfg.NavbarColor,
		NavbarTitle:           cfg.NavbarTitle,
		APIBaseURL:            cfg.APIBaseURL,
		MaxDashboardPageLimit: cfg.MaxDashboardPageLimit,
		TimeZone:              cfg.TZ,
		RemoteNodes:           remoteNodes,
	}

	if cfg.IsAuthToken {
		serverParams.AuthToken = &server.AuthToken{
			Token: cfg.AuthToken,
		}
	}

	if cfg.IsBasicAuth {
		serverParams.BasicAuth = &server.BasicAuth{
			Username: cfg.BasicAuthUsername,
			Password: cfg.BasicAuthPassword,
		}
	}

	return server.New(serverParams)
}
