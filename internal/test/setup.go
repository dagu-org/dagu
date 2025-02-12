package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
type TestHelperOption func(*TestOptions)

type TestOptions struct {
	CaptureLoggingOutput bool // CaptureLoggingOutput enables capturing of logging output
}

// WithCaptureLoggingOutput creates a logging capture option
func WithCaptureLoggingOutput() TestHelperOption {
	return func(opts *TestOptions) {
		opts.CaptureLoggingOutput = true
	}
}

// Setup creates a new Helper instance for testing
func Setup(t *testing.T, opts ...TestHelperOption) Helper {
	setupLock.Lock()
	defer setupLock.Unlock()

	var options TestOptions
	for _, opt := range opts {
		opt(&options)
	}

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

	if options.CaptureLoggingOutput {
		helper.LoggingOutput = &SyncBuffer{buf: new(bytes.Buffer)}
		loggerInstance := logger.NewLogger(
			logger.WithDebug(),
			logger.WithFormat("text"),
			logger.WithWriter(helper.LoggingOutput),
		)
		helper.Context = logger.WithFixedLogger(helper.Context, loggerInstance)
	}

	// setup the default shell for reproducible result
	setShell(t, "sh")

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

// DAG loads a test DAG from the testdata directory
func (h Helper) DAG(t *testing.T, name string) DAG {
	t.Helper()

	filePath := TestdataPath(t, name)
	dag, err := digraph.Load(h.Context, filePath)
	require.NoError(t, err, "failed to load test DAG %q", name)

	return DAG{
		Helper: &h,
		DAG:    dag,
	}
}

func (h Helper) DAGExpectError(t *testing.T, name string, expectedErr string) {
	t.Helper()

	filePath := TestdataPath(t, name)
	_, err := digraph.Load(h.Context, filePath)
	require.Error(t, err, "expected error loading test DAG %q", name)
	require.Contains(t, err.Error(), expectedErr, "expected error %q, got %q", expectedErr, err.Error())
}

type DAG struct {
	*Helper
	*digraph.DAG
}

func (d *DAG) AssertLatestStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	var status scheduler.Status
	var lock sync.Mutex

	require.Eventually(t, func() bool {
		lock.Lock()
		defer lock.Unlock()

		latest, err := d.Client.GetLatestStatus(d.Context, d.DAG)
		require.NoError(t, err)
		status = latest.Status
		return latest.Status == expected
	}, time.Second*3, time.Millisecond*50, "expected latest status to be %q, got %q", expected, status)
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

	var status scheduler.Status
	var lock sync.Mutex

	assert.Eventually(t, func() bool {
		lock.Lock()
		defer lock.Unlock()

		curr, err := d.Client.GetCurrentStatus(d.Context, d.DAG)
		require.NoError(t, err)
		status = curr.Status
		return curr.Status == expected
	}, time.Second*3, time.Millisecond*50, "expected current status to be %q, got %q", expected, status)
}

// AssertOutputs checks the given outputs against the actual outputs of the DAG
// Note that this function does not respect dependencies between nodes
// making the outputs with the same key indeterministic
func (d *DAG) AssertOutputs(t *testing.T, outputs map[string]any) {
	t.Helper()

	status, err := d.Client.GetLatestStatus(d.Context, d.DAG)
	require.NoError(t, err)

	// collect the actual outputs from the status
	var actualOutputs = make(map[string]string)
	for _, node := range status.Nodes {
		if node.Step.OutputVariables == nil {
			continue
		}
		value, ok := node.Step.OutputVariables.Load(node.Step.Output)
		if ok {
			actualOutputs[node.Step.Output] = value.(string)
		}
	}

	// compare the actual outputs with the expected outputs
	for key, expected := range outputs {
		if actual, ok := actualOutputs[key]; ok {
			switch expected := expected.(type) {
			case string:
				assert.Equal(t, fmt.Sprintf("%s=%s", key, expected), actual)

			case NotEmpty:
				parts := strings.SplitN(actual, "=", 2)
				assert.Len(t, parts, 2, "expected output %q to be in the form key=value", key)
				assert.NotEmpty(t, parts[1], "expected output %q to be not empty", key)

			default:
				t.Errorf("unsupported value matcher type %T", expected)

			}
		} else {
			t.Errorf("expected output %q not found", key)
		}
	}
}

type NotEmpty struct{}

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
	assert.NoError(t, err, "failed to run agent")

	status := a.Agent.Status().Status
	require.Equal(t, scheduler.StatusSuccess.String(), status.String(), "expected status %q, got %q", scheduler.StatusSuccess, status)
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

func setShell(t *testing.T, shell string) {
	t.Helper()

	shPath, err := exec.LookPath(shell)
	require.NoError(t, err, "failed to find shell")
	os.Setenv("SHELL", shPath)
}

func genRequestID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
}

// TestdataPath returns the path to a testdata file.
func TestdataPath(t *testing.T, filename string) string {
	t.Helper()

	rootDir := getProjectRoot(t)
	return filepath.Join(rootDir, "internal", "testdata", filename)
}

// ReadTestdata reads the content of a testdata file.
func ReadTestdata(t *testing.T, filename string) []byte {
	t.Helper()

	path := TestdataPath(t, filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read testdata file %q", filename)

	return data
}

// getProjectRoot returns the root directory of the project.
// This allows to read testdata files from the testdata directory.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller information")
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..")

	return filepath.Clean(rootDir)
}
