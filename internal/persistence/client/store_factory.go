package client

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
)

type dataStoreFactoryImpl struct {
	dataDir string
	dagsDir string
}

var _ persistence.DataStoreFactory = (*dataStoreFactoryImpl)(nil)

func NewDataStoreFactory(cfg *config.Config) persistence.DataStoreFactory {
	return &dataStoreFactoryImpl{
		dataDir: cfg.DataDir,
		dagsDir: cfg.DAGs,
	}
}

func (f dataStoreFactoryImpl) NewHistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	return jsondb.New(f.dataDir)
}

func (f dataStoreFactoryImpl) NewDAGStore() persistence.DAGStore {
	panic("implement me")
}
