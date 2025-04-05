package frontend

import (
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/api/v1"
	"github.com/dagu-org/dagu/internal/frontend/handlers"
	"github.com/dagu-org/dagu/internal/frontend/server"
)

func New(cfg *config.Config, cli client.Client) *Server {
	var apiHandlers []server.Handler

	dagAPIHandler := handlers.NewDAG(cli, cfg.UI.LogEncodingCharset, cfg.Server.RemoteNodes, cfg.Server.APIBasePath)
	apiHandlers = append(apiHandlers, dagAPIHandler)

	systemAPIHandler := handlers.NewSystem()
	apiHandlers = append(apiHandlers, systemAPIHandler)

	var remoteNodes []string
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	serverParams := server.NewServerArgs{
		Host:                  cfg.Server.Host,
		Port:                  cfg.Server.Port,
		TLS:                   cfg.Server.TLS,
		BasePath:              cfg.Server.BasePath,
		APIBaseURL:            cfg.Server.APIBasePath,
		Headless:              cfg.Server.Headless,
		Handlers:              apiHandlers,
		AssetsFS:              assetsFS,
		NavbarColor:           cfg.UI.NavbarColor,
		NavbarTitle:           cfg.UI.NavbarTitle,
		MaxDashboardPageLimit: cfg.UI.MaxDashboardPageLimit,
		TimeZone:              cfg.Global.TZ,
		RemoteNodes:           remoteNodes,
	}

	if cfg.Server.Auth.Token.Enabled {
		serverParams.AuthToken = &server.AuthToken{
			Token: cfg.Server.Auth.Token.Value,
		}
	}

	if cfg.Server.Auth.Basic.Enabled {
		serverParams.BasicAuth = &server.BasicAuth{
			Username: cfg.Server.Auth.Basic.Username,
			Password: cfg.Server.Auth.Basic.Password,
		}
	}
	a := api.New(cli, cfg)
	return NewServer(a, cfg)
}
