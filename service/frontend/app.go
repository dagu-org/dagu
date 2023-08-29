package frontend

import (
	"context"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/service/frontend/http"
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

func LifetimeHooks(lc fx.Lifecycle, srv *http.Server) {
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

func New(params Params) *http.Server {
	serverParams := http.ServerParams{
		Host:   params.Config.Host,
		Port:   params.Config.Port,
		TLS:    params.Config.TLS,
		Logger: params.Logger,
	}

	if params.Config.IsBasicAuth {
		serverParams.BasicAuth = &http.BasicAuth{
			Username: params.Config.BasicAuthUsername,
			Password: params.Config.BasicAuthUsername,
		}
	}

	return http.NewServer(serverParams)
}
