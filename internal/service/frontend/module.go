package frontend

import (
	"context"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/service/frontend/server"
	"go.uber.org/fx"
)

var Module = fx.Options(fx.Provide(NewServer))

type Params struct {
	fx.In

	Config *config.Config
	Logger logger.Logger
	Engine engine.Engine
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
