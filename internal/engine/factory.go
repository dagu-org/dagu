package engine

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
)

type Factory interface {
	Create() Engine
}

type factoryImpl struct {
	dataStoreFactory persistence.DataStoreFactory
	executable       string
	workDir          string
	suspendChecker   *suspend.SuspendChecker
}

func NewFactory(ds persistence.DataStoreFactory, cfg *config.Config) Factory {
	impl := &factoryImpl{
		dataStoreFactory: ds,
	}
	if cfg == nil {
		cfg = config.Get()
	}
	impl.executable = cfg.Command
	impl.suspendChecker = suspend.NewSuspendChecker(
		storage.NewStorage(cfg.SuspendFlagsDir),
	)
	return impl
}

func (f *factoryImpl) Create() Engine {
	return &engineImpl{
		dataStoreFactory: f.dataStoreFactory,
		executable:       f.executable,
		workDir:          f.workDir,
		suspendChecker:   f.suspendChecker,
	}
}
