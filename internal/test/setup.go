// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"bytes"
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

type Helper struct {
	Context       context.Context
	Config        *config.Config
	LoggingOutput *SyncBuffer

	tmpDir string
}

func (t Helper) Cleanup() {
	_ = os.RemoveAll(t.tmpDir)
}

func (t Helper) DataStore() persistence.DataStores {
	return dsclient.NewDataStores(
		t.Config.DAGs,
		t.Config.DataDir,
		t.Config.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: t.Config.LatestStatusToday,
		},
	)
}

func (t Helper) Client() client.Client {
	return client.New(t.DataStore(), t.Config.Executable, t.Config.WorkDir)
}

var lock sync.Mutex

type TestHelperOption func(th *Helper)

func WithCaptureLoggingOutput() TestHelperOption {
	return func(th *Helper) {
		th.LoggingOutput = &SyncBuffer{buf: new(bytes.Buffer)}

		loggerInstance := logger.NewLogger(
			logger.WithDebug(),
			logger.WithFormat("text"),
			logger.WithWriter(th.LoggingOutput),
		)
		th.Context = logger.WithFixedLogger(th.Context, loggerInstance)
	}
}

type SyncBuffer struct {
	buf  *bytes.Buffer
	lock sync.Mutex
}

func (b *SyncBuffer) Write(p []byte) (n int, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.Write(p)
}

func (b *SyncBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.String()
}

func Setup(t *testing.T, opts ...TestHelperOption) Helper {
	lock.Lock()
	defer lock.Unlock()

	tmpDir := fileutil.MustTempDir("test")

	_ = os.Setenv("DAGU_HOME", tmpDir)

	cfg, err := config.Load()
	require.NoError(t, err)

	// Set the executable path to the test binary.
	cfg.Executable = filepath.Join(fileutil.MustGetwd(), "../../bin/dagu")

	// TODO: Remove this if possible
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

	loggerOpts := []logger.Option{
		logger.WithDebug(), logger.WithFormat("text"),
	}
	ctx = logger.WithLogger(ctx, logger.NewLogger(loggerOpts...))

	helper := Helper{
		Context: ctx,
		Config:  cfg,

		tmpDir: tmpDir,
	}

	for _, opt := range opts {
		opt(&helper)
	}

	t.Cleanup(helper.Cleanup)

	return helper
}

func SetupForDir(t *testing.T, dir string) Helper {
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

	return Helper{
		Context: ctx,
		Config:  cfg,

		tmpDir: tmpDir,
	}
}
