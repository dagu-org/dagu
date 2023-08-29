package frontend

import (
	"context"
	"github.com/yohamta/dagu/internal/config"
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
	return web.NewServer(params.Config)
}
