// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dagrunstore selects the configured DAG-run persistence backend.
package dagrunstore

import (
	"context"
	"fmt"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/persis/dagrunstore/postgres"
	"github.com/dagucloud/dagu/internal/persis/filedagrun"
)

// Options contains runtime options that are not part of persistent config.
type Options struct {
	FileCache         *fileutil.Cache[*exec.DAGRunStatus]
	LatestStatusToday bool
	Location          *time.Location
}

// Option configures DAG-run store construction.
type Option func(*Options)

// WithHistoryFileCache sets the optional file-store status cache.
func WithHistoryFileCache(cache *fileutil.Cache[*exec.DAGRunStatus]) Option {
	return func(o *Options) {
		o.FileCache = cache
	}
}

// WithLatestStatusToday configures whether latest lookups are restricted to today.
func WithLatestStatusToday(latestStatusToday bool) Option {
	return func(o *Options) {
		o.LatestStatusToday = latestStatusToday
	}
}

// WithLocation sets the timezone used for "today" calculations.
func WithLocation(location *time.Location) Option {
	return func(o *Options) {
		o.Location = location
	}
}

// New creates the configured DAG-run store.
func New(ctx context.Context, cfg *config.Config, opts ...Option) (exec.DAGRunStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	options := Options{
		Location: cfg.Core.Location,
	}
	for _, opt := range opts {
		opt(&options)
	}

	switch cfg.DAGRunStore.Backend {
	case "", config.DAGRunStoreBackendFile:
		return newFileStore(cfg, options), nil
	case config.DAGRunStoreBackendPostgres:
		return postgres.New(ctx, postgres.Config{
			DSN:               cfg.DAGRunStore.Postgres.DSN,
			LocalWorkDirBase:  cfg.Paths.DAGRunsDir,
			AutoMigrate:       cfg.DAGRunStore.Postgres.AutoMigrate,
			LatestStatusToday: options.LatestStatusToday,
			Location:          options.Location,
			Pool: postgres.PoolConfig{
				MaxOpenConns:    cfg.DAGRunStore.Postgres.Pool.MaxOpenConns,
				MaxIdleConns:    cfg.DAGRunStore.Postgres.Pool.MaxIdleConns,
				ConnMaxLifetime: cfg.DAGRunStore.Postgres.Pool.ConnMaxLifetime,
				ConnMaxIdleTime: cfg.DAGRunStore.Postgres.Pool.ConnMaxIdleTime,
			},
		})
	default:
		return nil, fmt.Errorf("unsupported dag-run store backend %q", cfg.DAGRunStore.Backend)
	}
}

func newFileStore(cfg *config.Config, options Options) exec.DAGRunStore {
	fileOpts := []filedagrun.DAGRunStoreOption{
		filedagrun.WithArtifactDir(cfg.Paths.ArtifactDir),
		filedagrun.WithLatestStatusToday(options.LatestStatusToday),
		filedagrun.WithLocation(options.Location),
	}
	if options.FileCache != nil {
		fileOpts = append(fileOpts, filedagrun.WithHistoryFileCache(options.FileCache))
	}
	return filedagrun.New(cfg.Paths.DAGRunsDir, fileOpts...)
}
