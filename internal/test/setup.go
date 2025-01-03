package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"
	"github.com/google/uuid"
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

	random := uuid.New().String()
	tmpDir := fileutil.MustTempDir(fmt.Sprintf("dagu-test-%s", random))
	require.NoError(t, os.Setenv("DAGU_HOME", tmpDir))

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.Paths.Executable = executablePath
	cfg.Paths.LogDir = filepath.Join(tmpDir, "logs")

	dagStore := local.NewDAGStore(cfg.Paths.DAGsDir)
	historyStore := jsondb.New(cfg.Paths.DataDir)
	flagStore := local.NewFlagStore(
		storage.NewStorage(cfg.Paths.SuspendFlagsDir),
	)

	client := client.New(dagStore, historyStore, flagStore, cfg.Paths.Executable, cfg.WorkDir)

	helper := Helper{
		Context:      createDefaultContext(),
		Config:       cfg,
		Client:       client,
		DAGStore:     dagStore,
		HistoryStore: historyStore,

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
	HistoryStore  persistence.HistoryStore
	DAGStore      persistence.DAGStore

	tmpDir string
}

// Cleanup removes temporary test directories
func (h Helper) Cleanup() {
	_ = os.RemoveAll(h.tmpDir)
}

func (h Helper) LoadDAGFile(t *testing.T, filename string) DAG {
	t.Helper()

	filePath := filepath.Join(fileutil.MustGetwd(), "testdata", filename)
	dag, err := digraph.Load(h.Context, filePath)
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
	}, time.Second*3, time.Millisecond*50, "expected latest status to be %q, got %q", expected, latestStatusValue)
}

func (d *DAG) AssertHistoryCount(t *testing.T, expected int) {
	t.Helper()

	// the +1 to the limit is needed to ensure that the number of the history
	// entries is exactly the expected number
	history := d.Client.GetRecentHistory(d.Context, d.DAG, expected+1)
	require.Len(t, history, expected)
}

func (d *DAG) AssertCurrentStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	var lastCurrentStatus scheduler.Status
	assert.Eventuallyf(t, func() bool {
		currentStatus, err := d.Client.GetCurrentStatus(d.Context, d.DAG)
		require.NoError(t, err)
		lastCurrentStatus = currentStatus.Status
		return currentStatus.Status == expected
	}, time.Second*2, time.Millisecond*50, "expected current status to be %q, got %q", expected, lastCurrentStatus)
}

type AgentOption func(*Agent)

func WithAgentOptions(options agent.Options) AgentOption {
	return func(a *Agent) {
		a.opts = options
	}
}

func (d *DAG) Agent(opts ...AgentOption) *Agent {
	requestID := genRequestID()
	logDir := d.Config.Paths.LogDir
	logFile := filepath.Join(d.Config.Paths.LogDir, requestID+".log")

	helper := &Agent{
		Helper: d.Helper,
		DAG:    d.DAG,
	}

	for _, opt := range opts {
		opt(helper)
	}

	helper.Agent = agent.New(
		requestID,
		d.DAG,
		logDir,
		logFile,
		d.Client,
		d.DAGStore,
		d.HistoryStore,
		helper.opts,
	)

	return helper
}

func genRequestID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
}

type Agent struct {
	*Helper
	*digraph.DAG
	*agent.Agent
	opts agent.Options
}

func (a *Agent) RunError(t *testing.T) {
	t.Helper()

	err := a.Agent.Run(a.Context)
	assert.Error(t, err)

	status := a.Agent.Status().Status
	require.Equal(t, scheduler.StatusError.String(), status.String())
}

func (a *Agent) RunCancel(t *testing.T) {
	t.Helper()

	err := a.Agent.Run(a.Context)
	assert.NoError(t, err)

	status := a.Agent.Status().Status
	require.Equal(t, scheduler.StatusCancel.String(), status.String())
}

func (a *Agent) RunCheckErr(t *testing.T, expectedErr string) {
	t.Helper()

	err := a.Agent.Run(a.Context)
	require.Error(t, err, "expected error %q, got nil", expectedErr)
	require.Contains(t, err.Error(), expectedErr)
	status := a.Agent.Status()
	require.Equal(t, scheduler.StatusCancel.String(), status.Status.String())
}

func (a *Agent) RunSuccess(t *testing.T) {
	t.Helper()

	err := a.Agent.Run(a.Context)
	assert.NoError(t, err)

	status := a.Agent.Status().Status
	require.Equal(t, scheduler.StatusSuccess.String(), status.String())
}

func (a *Agent) Abort() {
	a.Signal(a.Context, syscall.SIGTERM)
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
