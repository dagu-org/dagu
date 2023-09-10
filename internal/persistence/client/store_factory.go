package client

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/local"
)

type dataStoreFactoryImpl struct {
	cfg *config.Config
}

var _ persistence.DataStoreFactory = (*dataStoreFactoryImpl)(nil)

func NewDataStoreFactory(cfg *config.Config) persistence.DataStoreFactory {
	return &dataStoreFactoryImpl{
		cfg: cfg,
	}
}

func (f dataStoreFactoryImpl) NewHistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	return jsondb.New(f.cfg.DataDir, f.cfg.DAGs)
}

func (f dataStoreFactoryImpl) NewDAGStore() persistence.DAGStore {
	return local.NewDAGStore(f.cfg.DAGs)
}
