package client

import (
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/local"
	"github.com/dagu-dev/dagu/internal/persistence/local/storage"
)

var _ persistence.DataStoreFactory = (*dataStoreFactoryImpl)(nil)

type dataStoreFactoryImpl struct {
	cfg          *config.Config
	historyStore persistence.HistoryStore
	dagStore     persistence.DAGStore
}

func NewDataStoreFactory(cfg *config.Config) persistence.DataStoreFactory {
	dataStoreImpl := &dataStoreFactoryImpl{cfg: cfg}
	_ = dataStoreImpl.InitDagDir()
	return dataStoreImpl
}

func (f *dataStoreFactoryImpl) InitDagDir() error {
	_, err := os.Stat(f.cfg.DAGs)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(f.cfg.DAGs, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (f *dataStoreFactoryImpl) NewHistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	if f.historyStore == nil {
		f.historyStore = jsondb.New(
			f.cfg.DataDir, f.cfg.DAGs, f.cfg.LatestStatusToday)
	}
	return f.historyStore
}

func (f *dataStoreFactoryImpl) NewDAGStore() persistence.DAGStore {
	if f.dagStore == nil {
		f.dagStore = local.NewDAGStore(f.cfg, f.cfg.DAGs)
	}
	return f.dagStore
}

func (f *dataStoreFactoryImpl) NewFlagStore() persistence.FlagStore {
	return local.NewFlagStore(storage.NewStorage(f.cfg.SuspendFlagsDir))
}
