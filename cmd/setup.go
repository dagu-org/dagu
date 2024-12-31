// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func wrapRunE(f func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := f(cmd, args); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return nil
	}
}

type setup struct {
	cfg *config.Config
}

func newSetup(cfg *config.Config) *setup {
	return &setup{cfg: cfg}
}

func (s *setup) dataStores() persistence.DataStores {
	return dsclient.NewDataStores(
		s.cfg.Paths.DAGsDir,
		s.cfg.Paths.DataDir,
		s.cfg.Paths.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: s.cfg.LatestStatusToday,
		},
	)
}

type clientOption func(*clientOptions)

type clientOptions struct {
	dagStore     persistence.DAGStore
	historyStore persistence.HistoryStore
}

func withDAGStore(dagStore persistence.DAGStore) clientOption {
	return func(o *clientOptions) {
		o.dagStore = dagStore
	}
}

func withHistoryStore(historyStore persistence.HistoryStore) clientOption {
	return func(o *clientOptions) {
		o.historyStore = historyStore
	}
}

func (s *setup) client(opts ...clientOption) client.Client {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}
	dagStore := options.dagStore
	if dagStore == nil {
		dagStore = s.dagStore()
	}
	historyStore := options.historyStore
	if historyStore == nil {
		historyStore = s.historyStore()
	}

	return client.New(
		s.dataStores(),
		dagStore,
		historyStore,
		s.cfg.Paths.Executable,
		s.cfg.WorkDir,
	)
}

func (s *setup) server(ctx context.Context) *server.Server {
	dagCache := filecache.New[*digraph.DAG](0, time.Hour*12)
	dagCache.StartEviction(ctx)
	dagStore := s.dagStoreWithCache(dagCache)

	historyCache := filecache.New[*model.Status](0, time.Hour*12)
	historyCache.StartEviction(ctx)
	historyStore := s.historyStoreWithCache(historyCache)

	cli := s.client(withDAGStore(dagStore), withHistoryStore(historyStore))
	return frontend.New(s.cfg, cli)
}

func (s *setup) scheduler() *scheduler.Scheduler {
	return scheduler.New(s.cfg, s.client())
}

func (s *setup) dagStore() persistence.DAGStore {
	return local.NewDAGStore(s.cfg.Paths.DAGsDir)
}

func (s *setup) dagStoreWithCache(cache *filecache.Cache[*digraph.DAG]) persistence.DAGStore {
	return local.NewDAGStore(s.cfg.Paths.DAGsDir, local.WithFileCache(cache))
}

func (s *setup) historyStore() persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir, jsondb.WithLatestStatusToday(
		s.cfg.LatestStatusToday,
	))
}

func (s *setup) historyStoreWithCache(cache *filecache.Cache[*model.Status]) persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir,
		jsondb.WithLatestStatusToday(s.cfg.LatestStatusToday),
		jsondb.WithFileCache(cache),
	)
}

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

type signalListener interface {
	Signal(context.Context, os.Signal)
}

var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to the OS signals and passes them to the listener.
// It listens for the context cancellation as well.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			listener.Signal(ctx, os.Interrupt)
		case sig := <-signalChan:
			listener.Signal(ctx, sig)
		}
	}()
}
