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
	Role              Role
}

// Option configures DAG-run store construction.
type Option func(*Options)

// Role identifies the Dagu process role that owns a DAG-run store connection.
type Role string

const (
	// RoleServer is used by the frontend/API process.
	RoleServer Role = "server"
	// RoleScheduler is used by the scheduler process.
	RoleScheduler Role = "scheduler"
	// RoleAgent is used by DAG execution processes.
	RoleAgent Role = "agent"
)

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

// WithRole selects the process-specific PostgreSQL DAG-run store settings.
func WithRole(role Role) Option {
	return func(o *Options) {
		o.Role = role
	}
}

// New creates the configured DAG-run store.
func New(ctx context.Context, cfg *config.Config, opts ...Option) (exec.DAGRunStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	options := Options{
		Location: cfg.Core.Location,
		Role:     RoleServer,
	}
	for _, opt := range opts {
		opt(&options)
	}

	switch cfg.DAGRunStore.Backend {
	case "", config.DAGRunStoreBackendFile:
		return newFileStore(cfg, options), nil
	case config.DAGRunStoreBackendPostgres:
		pgCfg, err := postgresRoleConfig(cfg.DAGRunStore.Postgres, options.Role)
		if err != nil {
			return nil, err
		}
		if pgCfg.DSN == "" {
			return nil, fmt.Errorf("dag_run_store.postgres.%s.dsn is required when dag_run_store.backend is postgres", options.Role)
		}
		return postgres.New(ctx, postgres.Config{
			DSN:               pgCfg.DSN,
			LocalWorkDirBase:  cfg.Paths.DAGRunsDir,
			AutoMigrate:       pgCfg.AutoMigrate,
			LatestStatusToday: options.LatestStatusToday,
			Location:          options.Location,
			Pool: postgres.PoolConfig{
				MaxOpenConns:    pgCfg.Pool.MaxOpenConns,
				MaxIdleConns:    pgCfg.Pool.MaxIdleConns,
				ConnMaxLifetime: pgCfg.Pool.ConnMaxLifetime,
				ConnMaxIdleTime: pgCfg.Pool.ConnMaxIdleTime,
			},
		})
	default:
		return nil, fmt.Errorf("unsupported dag-run store backend %q", cfg.DAGRunStore.Backend)
	}
}

func postgresRoleConfig(cfg config.DAGRunStorePostgresConfig, role Role) (config.DAGRunStorePostgresRoleConfig, error) {
	switch role {
	case "", RoleServer:
		return cfg.Server, nil
	case RoleScheduler:
		return cfg.Scheduler, nil
	case RoleAgent:
		return cfg.Agent, nil
	default:
		return config.DAGRunStorePostgresRoleConfig{}, fmt.Errorf("unsupported dag-run store role %q", role)
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
