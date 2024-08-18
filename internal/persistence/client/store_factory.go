// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package client

import (
	"os"

	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/persistence"
	"github.com/daguflow/dagu/internal/persistence/jsondb"
	"github.com/daguflow/dagu/internal/persistence/local"
	"github.com/daguflow/dagu/internal/persistence/local/storage"
)

var _ persistence.DataStores = (*dataStores)(nil)

type dataStores struct {
	historyStore persistence.HistoryStore
	dagStore     persistence.DAGStore

	dags              string
	dataDir           string
	suspendFlagsDir   string
	latestStatusToday bool
	logger            logger.Logger
}

type DataStoreOptions struct {
	LatestStatusToday bool
}

func NewDataStores(
	dags string,
	dataDir string,
	suspendFlagsDir string,
	logger logger.Logger,
	opts DataStoreOptions,
) persistence.DataStores {
	dataStoreImpl := &dataStores{
		dags:              dags,
		dataDir:           dataDir,
		suspendFlagsDir:   suspendFlagsDir,
		latestStatusToday: opts.LatestStatusToday,
		logger:            logger,
	}
	_ = dataStoreImpl.InitDagDir()
	return dataStoreImpl
}

func (f *dataStores) InitDagDir() error {
	_, err := os.Stat(f.dags)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(f.dags, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (f *dataStores) HistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	if f.historyStore == nil {
		f.historyStore = jsondb.New(
			f.dataDir, f.logger, f.latestStatusToday)
	}
	return f.historyStore
}

func (f *dataStores) DAGStore() persistence.DAGStore {
	if f.dagStore == nil {
		f.dagStore = local.NewDAGStore(&local.NewDAGStoreArgs{Dir: f.dags})
	}
	return f.dagStore
}

func (f *dataStores) FlagStore() persistence.FlagStore {
	return local.NewFlagStore(storage.NewStorage(f.suspendFlagsDir))
}
