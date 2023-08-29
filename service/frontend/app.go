package frontend

import (
	"context"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/service/frontend/web"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(New),
	fx.Invoke(LifetimeHooks),
)

type Params struct {
	fx.In

	Config *config.Config
	Logger logger.Logger
}

func LifetimeHooks(lc fx.Lifecycle, srv *web.Server) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) (err error) {
				return srv.Start()
			},
			OnStop: func(ctx context.Context) error {
				return srv.Shutdown(ctx)
			},
		},
	)
}

func New(params Params) *web.Server {
	serverParams := web.ServerParams{
		Host:   params.Config.Host,
		Port:   params.Config.Port,
		TLS:    params.Config.TLS,
		Logger: params.Logger,
	}

	if params.Config.IsBasicAuth {
		serverParams.BasicAuth = &web.BasicAuth{
			Username: params.Config.BasicAuthUsername,
			Password: params.Config.BasicAuthUsername,
		}
	}

	return web.NewServer(serverParams)
}
