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

// Helper provides test utilities and configuration
type Helper struct {
	Context       context.Context
	Config        *config.Config
	LoggingOutput *SyncBuffer
	tmpDir        string
}

// DataStore creates a new DataStores instance
func (h Helper) DataStore() persistence.DataStores {
	return dsclient.NewDataStores(
		h.Config.Paths.DAGsDir,
		h.Config.Paths.DataDir,
		h.Config.Paths.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: h.Config.LatestStatusToday,
		},
	)
}

// Client creates a new Client instance
func (h Helper) Client() client.Client {
	return client.New(h.DataStore(), h.Config.Paths.Executable, h.Config.WorkDir)
}

// Cleanup removes temporary test directories
func (h Helper) Cleanup() {
	_ = os.RemoveAll(h.tmpDir)
}

// SyncBuffer provides thread-safe buffer operations
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

// TestHelperOption defines functional options for Helper
type TestHelperOption func(*Helper)

// WithCaptureLoggingOutput creates a logging capture option
func WithCaptureLoggingOutput() TestHelperOption {
	return func(h *Helper) {
		h.LoggingOutput = &SyncBuffer{buf: new(bytes.Buffer)}
		loggerInstance := logger.NewLogger(
			logger.WithDebug(),
			logger.WithFormat("text"),
			logger.WithWriter(h.LoggingOutput),
		)
		h.Context = logger.WithFixedLogger(h.Context, loggerInstance)
	}
}

var setupLock sync.Mutex

// Setup creates a new Helper instance for testing
func Setup(t *testing.T, opts ...TestHelperOption) Helper {
	setupLock.Lock()
	defer setupLock.Unlock()

	tmpDir := fileutil.MustTempDir("test")
	require.NoError(t, os.Setenv("DAGU_HOME", tmpDir))

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.Paths.Executable = filepath.Join(fileutil.MustGetwd(), "../../.local/bin/dagu")

	helper := Helper{
		Context: createDefaultContext(),
		Config:  cfg,
		tmpDir:  tmpDir,
	}

	for _, opt := range opts {
		opt(&helper)
	}

	t.Cleanup(helper.Cleanup)
	return helper
}

// SetupForDir creates a new Helper instance with a specific configuration directory
func SetupForDir(t *testing.T, dir string) Helper {
	setupLock.Lock()
	defer setupLock.Unlock()

	tmpDir := fileutil.MustTempDir("test")
	require.NoError(t, os.Setenv("HOME", tmpDir))

	configureViper(dir)

	cfg, err := config.Load()
	require.NoError(t, err)

	return Helper{
		Context: createDefaultContext(),
		Config:  cfg,
		tmpDir:  tmpDir,
	}
}

// createDefaultContext creates a context with default logger settings
func createDefaultContext() context.Context {
	ctx := context.Background()
	return logger.WithLogger(ctx, logger.NewLogger(
		logger.WithDebug(),
		logger.WithFormat("text"),
	))
}

// configureViper sets up Viper configuration
func configureViper(dir string) {
	viper.AddConfigPath(dir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")
}
