package frontend

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/handlers"
	"github.com/dagu-org/dagu/internal/frontend/server"
)

func New(cfg *config.Config, cli client.Client) *server.Server {
	var apiHandlers []server.Handler

	dagAPIHandler := handlers.NewDAG(cli, cfg.UI.LogEncodingCharset, cfg.RemoteNodes, cfg.APIBasePath)
	apiHandlers = append(apiHandlers, dagAPIHandler)

	systemAPIHandler := handlers.NewSystem()
	apiHandlers = append(apiHandlers, systemAPIHandler)

	var remoteNodes []string
	for _, n := range cfg.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	serverParams := server.NewServerArgs{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		TLS:                   cfg.TLS,
		Handlers:              apiHandlers,
		AssetsFS:              assetsFS,
		NavbarColor:           cfg.UI.NavbarColor,
		NavbarTitle:           cfg.UI.NavbarTitle,
		MaxDashboardPageLimit: cfg.UI.MaxDashboardPageLimit,
		APIBaseURL:            cfg.APIBasePath,
		TimeZone:              cfg.TZ,
		RemoteNodes:           remoteNodes,
		Headless:              cfg.Headless,
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
