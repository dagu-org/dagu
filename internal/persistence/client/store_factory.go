// Copyright (C) 2024 The Dagu Authors
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

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/queue"
	"github.com/dagu-org/dagu/internal/persistence/stats"

	"github.com/dagu-org/dagu/internal/persistence/local/storage"
)

var _ persistence.DataStores = (*dataStores)(nil)

type dataStores struct {
	historyStore persistence.HistoryStore
	dagStore     persistence.DAGStore
	queueStore   persistence.QueueStore
	statsStore   persistence.StatsStore

	dags              string
	dataDir           string
	queueDir          string
	statsDir          string
	suspendFlagsDir   string
	latestStatusToday bool
}

type DataStoreOptions struct {
	LatestStatusToday bool
}

func NewDataStores(
	dags string,
	dataDir string,
	queueDir string,
	statsDir string,
	suspendFlagsDir string,
	opts DataStoreOptions,
) persistence.DataStores {
	dataStoreImpl := &dataStores{
		dags:              dags,
		dataDir:           dataDir,
		queueDir:          queueDir,
		statsDir:          statsDir,
		suspendFlagsDir:   suspendFlagsDir,
		latestStatusToday: opts.LatestStatusToday,
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
			f.dataDir, f.latestStatusToday)
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

func (f *dataStores) QueueStore() persistence.QueueStore {
	if f.queueStore == nil {
		f.queueStore = queue.NewQueueStore(f.queueDir)
	}
	return f.queueStore
}

func (f *dataStores) StatsStore() persistence.StatsStore {
	if f.statsStore == nil {
		f.statsStore = stats.NewStatsStore(f.statsDir)
	}
	return f.statsStore
}
