// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package frontend

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/dags"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/logger"
)

func New(cfg *config.Config, lg logger.Logger, cli client.Client) *server.Server {
	var hs []server.Handler

	hs = append(hs, dags.NewHandler(&dags.NewHandlerArgs{
		Client:             cli,
		LogEncodingCharset: cfg.LogEncodingCharset,
	}))

	serverParams := server.NewServerArgs{
		Host:        cfg.Host,
		Port:        cfg.Port,
		TLS:         cfg.TLS,
		Logger:      lg,
		Handlers:    hs,
		AssetsFS:    assetsFS,
		NavbarColor: cfg.NavbarColor,
		NavbarTitle: cfg.NavbarTitle,
		BasePath:    cfg.BasePath,
		APIBaseURL:  cfg.APIBaseURL,
		TimeZone:    cfg.TZ,
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
