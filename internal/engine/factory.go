package engine

import "github.com/dagu-dev/dagu/internal/persistence"

type Factory interface {
	Create() Engine
}

type factoryImpl struct {
	dataStoreFactory persistence.DataStoreFactory
}

func NewFactory(ds persistence.DataStoreFactory) Factory {
	return &factoryImpl{
		dataStoreFactory: ds,
	}
}

func (f *factoryImpl) Create() Engine {
	return New(f.dataStoreFactory)
}
