package frontend

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/frontend/dag"
	"github.com/dagu-dev/dagu/internal/frontend/server"
	"github.com/dagu-dev/dagu/internal/logger"
)

func New(cfg *config.Config, lg logger.Logger, eng engine.Engine) *server.Server {
	var hs []server.Handler

	hs = append(hs, dag.NewHandler(
		&dag.NewHandlerArgs{
			Engine:             eng,
			LogEncodingCharset: cfg.LogEncodingCharset,
		},
	))

	serverParams := server.NewServerArgs{
		Host:        cfg.Host,
		Port:        cfg.Port,
		TLS:         cfg.TLS,
		Logger:      lg,
		Handlers:    hs,
		AssetsFS:    assetsFS,
		NavbarColor: cfg.NavbarColor,
		NavbarTitle: cfg.NavbarTitle,
		APIBaseURL:  cfg.APIBaseURL,
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
