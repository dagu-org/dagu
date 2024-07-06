package frontend

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/frontend/dag"
	"github.com/dagu-dev/dagu/internal/frontend/server"
	"github.com/dagu-dev/dagu/internal/logger"
	"go.uber.org/fx"
)

var Module = fx.Options(fx.Provide(NewServer))

type Params struct {
	fx.In

	Config *config.Config
	Logger logger.Logger
	Engine engine.Engine
}

func NewServer(params Params) *server.Server {

	var hs []server.Handler

	hs = append(hs, dag.NewHandler(
		&dag.NewHandlerArgs{
			Engine:             params.Engine,
			LogEncodingCharset: params.Config.LogEncodingCharset,
		},
	))

	serverParams := server.NewServerArgs{
		Host:        params.Config.Host,
		Port:        params.Config.Port,
		TLS:         params.Config.TLS,
		Logger:      params.Logger,
		Handlers:    hs,
		AssetsFS:    assetsFS,
		NavbarColor: params.Config.NavbarColor,
		NavbarTitle: params.Config.NavbarTitle,
		APIBaseURL:  params.Config.APIBaseURL,
	}

	if params.Config.IsAuthToken {
		serverParams.AuthToken = &server.AuthToken{
			Token: params.Config.AuthToken,
		}
	}

	if params.Config.IsBasicAuth {
		serverParams.BasicAuth = &server.BasicAuth{
			Username: params.Config.BasicAuthUsername,
			Password: params.Config.BasicAuthPassword,
		}
	}

	return server.New(serverParams)
}

func LifetimeHooks(lc fx.Lifecycle, srv *server.Server) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) (err error) {
				return srv.Serve(ctx)
			},
			OnStop: func(_ context.Context) error {
				srv.Shutdown()
				return nil
			},
		},
	)
}
