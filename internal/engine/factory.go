package engine

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
)

type Factory interface {
	Create() Engine
}

type factoryImpl struct {
	dataStoreFactory persistence.DataStoreFactory
	executable       string
	workDir          string
}

func NewFactory(ds persistence.DataStoreFactory, cfg *config.Config) Factory {
	impl := &factoryImpl{
		dataStoreFactory: ds,
		executable:       cfg.Executable,
	}
	return impl
}

func (f *factoryImpl) Create() Engine {
	return &engineImpl{
		dataStoreFactory: f.dataStoreFactory,
		executable:       f.executable,
		workDir:          f.workDir,
	}
}
