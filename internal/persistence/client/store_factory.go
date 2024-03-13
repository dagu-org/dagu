package client

import (
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/local"
	"github.com/dagu-dev/dagu/internal/persistence/local/storage"
)

type dataStoreFactoryImpl struct {
	cfg *config.Config
}

var _ persistence.DataStoreFactory = (*dataStoreFactoryImpl)(nil)

func NewDataStoreFactory(cfg *config.Config) persistence.DataStoreFactory {
	ds := &dataStoreFactoryImpl{
		cfg: cfg,
	}
	_ = ds.InitDagDir()
	return ds
}

func (f dataStoreFactoryImpl) InitDagDir() error {
	_, err := os.Stat(f.cfg.DAGs)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(f.cfg.DAGs, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (f dataStoreFactoryImpl) NewHistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	return jsondb.New(f.cfg.DataDir, f.cfg.DAGs)
}

func (f dataStoreFactoryImpl) NewDAGStore() persistence.DAGStore {
	return local.NewDAGStore(f.cfg.DAGs)
}

func (f dataStoreFactoryImpl) NewFlagStore() persistence.FlagStore {
	s := storage.NewStorage(f.cfg.SuspendFlagsDir)
	return local.NewFlagStore(s)
}
