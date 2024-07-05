package client

import (
	"os"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/local"
	"github.com/dagu-dev/dagu/internal/persistence/local/storage"
)

var _ persistence.DataStoreFactory = (*dataStoreFactoryImpl)(nil)

type dataStoreFactoryImpl struct {
	historyStore persistence.HistoryStore
	dagStore     persistence.DAGStore

	dags              string
	dataDir           string
	suspendFlagsDir   string
	latestStatusToday bool
	loader            *dag.Loader
}

type NewDataStoreFactoryArgs struct {
	DAGs              string
	DataDir           string
	SuspendFlagsDir   string
	LatestStatusToday bool
	Loader            *dag.Loader
}

func NewDataStoreFactory(args *NewDataStoreFactoryArgs) persistence.DataStoreFactory {
	dataStoreImpl := &dataStoreFactoryImpl{
		dags:              args.DAGs,
		dataDir:           args.DataDir,
		suspendFlagsDir:   args.SuspendFlagsDir,
		latestStatusToday: args.LatestStatusToday,
		loader:            args.Loader,
	}
	_ = dataStoreImpl.InitDagDir()
	return dataStoreImpl
}

func (f *dataStoreFactoryImpl) InitDagDir() error {
	_, err := os.Stat(f.dags)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(f.dags, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (f *dataStoreFactoryImpl) NewHistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	if f.historyStore == nil {
		f.historyStore = jsondb.New(
			f.dataDir, f.dags, f.latestStatusToday)
	}
	return f.historyStore
}

func (f *dataStoreFactoryImpl) NewDAGStore() persistence.DAGStore {
	if f.dagStore == nil {
		f.dagStore = local.NewDAGStore(&local.NewDAGStoreArgs{
			Dir:    f.dags,
			Loader: f.loader,
		})
	}
	return f.dagStore
}

func (f *dataStoreFactoryImpl) NewFlagStore() persistence.FlagStore {
	return local.NewFlagStore(storage.NewStorage(f.suspendFlagsDir))
}
