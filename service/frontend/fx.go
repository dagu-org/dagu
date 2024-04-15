package frontend

import (
	"context"
	"embed"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/service/frontend/handlers"
	"github.com/dagu-dev/dagu/service/frontend/server"
	"go.uber.org/fx"
)

var (
	//go:embed templates/* assets/*
	assetsFS embed.FS
)

var Module = fx.Options(
	fx.Provide(
		fx.Annotate(handlers.NewDAG, fx.ResultTags(`group:"handlers"`))),
	fx.Provide(New),
)

type Params struct {
	fx.In

	Config   *config.Config
	Logger   logger.Logger
	Handlers []server.New `group:"handlers"`
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

func New(params Params) *server.Server {
	serverParams := server.Params{
		Host:     params.Config.Host,
		Port:     params.Config.Port,
		TLS:      params.Config.TLS,
		Logger:   params.Logger,
		Handlers: params.Handlers,
		AssetsFS: assetsFS,
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

	return server.NewServer(serverParams)
}
