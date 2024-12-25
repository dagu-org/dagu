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
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var setupLock sync.Mutex
var executablePath string

func init() {
	executablePath = filepath.Join(fileutil.MustGetwd(), "../../.local/bin/dagu")
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

// Setup creates a new Helper instance for testing
func Setup(t *testing.T, opts ...TestHelperOption) Helper {
	setupLock.Lock()
	defer setupLock.Unlock()

	tmpDir := fileutil.MustTempDir("test")
	require.NoError(t, os.Setenv("DAGU_HOME", tmpDir))

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.Paths.Executable = executablePath

	dataStores := dsclient.NewDataStores(
		cfg.Paths.DAGsDir,
		cfg.Paths.DataDir,
		cfg.Paths.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)

	helper := Helper{
		Context:    createDefaultContext(),
		Config:     cfg,
		Client:     client.New(dataStores, cfg.Paths.Executable, cfg.WorkDir),
		DataStores: dataStores,

		tmpDir: tmpDir,
	}

	for _, opt := range opts {
		opt(&helper)
	}

	t.Cleanup(helper.Cleanup)
	return helper
}

// Helper provides test utilities and configuration
type Helper struct {
	Context       context.Context
	Config        *config.Config
	LoggingOutput *SyncBuffer
	Client        client.Client
	DataStores    persistence.DataStores

	tmpDir string
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

// Cleanup removes temporary test directories
func (h Helper) Cleanup() {
	_ = os.RemoveAll(h.tmpDir)
}

func (h Helper) LoadDAGFile(t *testing.T, filename string) DAG {
	t.Helper()

	filePath := filepath.Join(fileutil.MustGetwd(), "testdata", filename)
	dag, err := digraph.Load(h.Context, "", filePath, "")
	require.NoError(t, err)

	return DAG{
		Helper: &h,
		DAG:    dag,
	}
}

type DAG struct {
	*Helper
	*digraph.DAG
}

func (d *DAG) AssertLatestStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	var latestStatusValue scheduler.Status
	assert.Eventually(t, func() bool {
		latestStatus, err := d.Client.GetLatestStatus(d.Context, d.DAG)
		require.NoError(t, err)
		latestStatusValue = latestStatus.Status
		return latestStatus.Status == expected
	}, time.Second*3, time.Millisecond*100, "expected latest status to be %q, got %q", expected, latestStatusValue)
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

// createDefaultContext creates a context with default logger settings
func createDefaultContext() context.Context {
	ctx := context.Background()
	return logger.WithLogger(ctx, logger.NewLogger(
		logger.WithDebug(),
		logger.WithFormat("text"),
	))
}
