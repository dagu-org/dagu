package scheduler

import (
	"github.com/dagu-dev/dagu/service/scheduler"
	"go.uber.org/fx"
)

func New(topLevelModule fx.Option) *fx.App {
	return fx.New(
		topLevelModule,
		scheduler.Module,
		fx.Invoke(scheduler.LifetimeHooks),
		fx.NopLogger,
	)
}
