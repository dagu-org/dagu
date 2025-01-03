package frontend

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/dag"
	"github.com/dagu-org/dagu/internal/frontend/server"
)

func New(cfg *config.Config, cli client.Client) *server.Server {
	var hs []server.Handler

	hs = append(hs, dag.NewHandler(
		&dag.NewHandlerArgs{
			Client:             cli,
			LogEncodingCharset: cfg.UI.LogEncodingCharset,
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
		Handlers:              hs,
		AssetsFS:              assetsFS,
		NavbarColor:           cfg.UI.NavbarColor,
		NavbarTitle:           cfg.UI.NavbarTitle,
		MaxDashboardPageLimit: cfg.UI.MaxDashboardPageLimit,
		APIBaseURL:            cfg.APIBaseURL,
		TimeZone:              cfg.TZ,
		RemoteNodes:           remoteNodes,
	}

	if cfg.Auth.Token.Enabled {
		serverParams.AuthToken = &server.AuthToken{
			Token: cfg.Auth.Token.Value,
		}
	}

	if cfg.Auth.Basic.Enabled {
		serverParams.BasicAuth = &server.BasicAuth{
			Username: cfg.Auth.Basic.Username,
			Password: cfg.Auth.Basic.Password,
		}
	}

	return server.New(serverParams)
}
