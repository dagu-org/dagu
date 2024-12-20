// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type Setup struct {
	Context context.Context
	Config  *config.Config

	tmpDir string
}

func (t Setup) Cleanup() {
	_ = os.RemoveAll(t.tmpDir)
}

func (t Setup) DataStore() persistence.DataStores {
	return dsclient.NewDataStores(
		t.Config.DAGs,
		t.Config.DataDir,
		t.Config.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: t.Config.LatestStatusToday,
		},
	)
}

func (t Setup) Client() client.Client {
	return client.New(t.DataStore(), t.Config.Executable, t.Config.WorkDir)
}

var (
	lock sync.Mutex
)

func SetupTest(t *testing.T) Setup {
	lock.Lock()
	defer lock.Unlock()

	tmpDir := fileutil.MustTempDir("test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	configDir := filepath.Join(tmpDir, "config")
	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.DAGs = filepath.Join(tmpDir, "dags")
	cfg.WorkDir = tmpDir
	cfg.BaseConfig = filepath.Join(tmpDir, "config", "base.yaml")
	cfg.DataDir = filepath.Join(tmpDir, "data")
	cfg.LogDir = filepath.Join(tmpDir, "log")
	cfg.AdminLogsDir = filepath.Join(tmpDir, "log", "admin")

	// Set the executable path to the test binary.
	cfg.Executable = filepath.Join(fileutil.MustGetwd(), "../../bin/dagu")

	// Set environment variables.
	// This is required for some tests that run the executable
	_ = os.Setenv("DAGU_DAGS_DIR", cfg.DAGs)
	_ = os.Setenv("DAGU_WORK_DIR", cfg.WorkDir)
	_ = os.Setenv("DAGU_BASE_CONFIG", cfg.BaseConfig)
	_ = os.Setenv("DAGU_LOG_DIR", cfg.LogDir)
	_ = os.Setenv("DAGU_DATA_DIR", cfg.DataDir)
	_ = os.Setenv("DAGU_SUSPEND_FLAGS_DIR", cfg.SuspendFlagsDir)
	_ = os.Setenv("DAGU_ADMIN_LOG_DIR", cfg.AdminLogsDir)

	ctx := context.Background()
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.WithDebug(), logger.WithFormat("text")))

	setup := Setup{
		Context: ctx,
		Config:  cfg,

		tmpDir: tmpDir,
	}

	t.Cleanup(setup.Cleanup)

	return setup
}

func SetupForDir(t *testing.T, dir string) Setup {
	lock.Lock()
	defer lock.Unlock()

	tmpDir := fileutil.MustTempDir("test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	viper.AddConfigPath(dir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	cfg, err := config.Load()
	require.NoError(t, err)

	ctx := context.Background()
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.WithDebug(), logger.WithFormat("text")))

	return Setup{
		Context: ctx,
		Config:  cfg,

		tmpDir: tmpDir,
	}
}
