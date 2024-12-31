// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"os"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"
)

var _ persistence.DataStores = (*dataStores)(nil)

type dataStores struct {
	historyStore persistence.HistoryStore
	dagStore     persistence.DAGStore

	dagsDir           string
	dataDir           string
	suspendFlagsDir   string
	latestStatusToday bool
}

type DataStoreOptions struct {
	LatestStatusToday bool
}

func NewDataStores(
	dagsDir string,
	dataDir string,
	suspendFlagsDir string,
	opts DataStoreOptions,
) persistence.DataStores {
	dataStoreImpl := &dataStores{
		dagsDir:           dagsDir,
		dataDir:           dataDir,
		suspendFlagsDir:   suspendFlagsDir,
		latestStatusToday: opts.LatestStatusToday,
	}
	_ = dataStoreImpl.InitDagDir()
	return dataStoreImpl
}

func (f *dataStores) InitDagDir() error {
	_, err := os.Stat(f.dagsDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(f.dagsDir, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (f *dataStores) HistoryStore() persistence.HistoryStore {
	// TODO: Add support for other data stores (e.g. sqlite, postgres, etc.)
	if f.historyStore == nil {
		cfg := jsondb.DefaultConfig()
		cfg.LatestStatusToday = f.latestStatusToday
		f.historyStore = jsondb.New(f.dataDir, cfg)
	}
	return f.historyStore
}

func (f *dataStores) DAGStore() persistence.DAGStore {
	if f.dagStore == nil {
		f.dagStore = local.NewDAGStore(f.dagsDir)
	}
	return f.dagStore
}

func (f *dataStores) FlagStore() persistence.FlagStore {
	return local.NewFlagStore(storage.NewStorage(f.suspendFlagsDir))
}
