package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/localdag"
	"github.com/dagu-org/dagu/internal/persistence/localhistory"
	"github.com/dagu-org/dagu/internal/persistence/localproc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var setupLock sync.Mutex

// HelperOption defines functional options for Helper
type HelperOption func(*Options)

type Options struct {
	CaptureLoggingOutput bool // CaptureLoggingOutput enables capturing of logging output
	DAGsDir              string
	ServerConfig         *config.Server
}

// WithCaptureLoggingOutput creates a logging capture option
func WithCaptureLoggingOutput() HelperOption {
	return func(opts *Options) {
		opts.CaptureLoggingOutput = true
	}
}

func WithDAGsDir(dir string) HelperOption {
	return func(opts *Options) {
		opts.DAGsDir = dir
	}
}

func WithServerConfig(cfg *config.Server) HelperOption {
	return func(opts *Options) {
		opts.ServerConfig = cfg
	}
}

// Setup creates a new Helper instance for testing
func Setup(t *testing.T, opts ...HelperOption) Helper {
	setupLock.Lock()
	defer setupLock.Unlock()

	var options Options
	for _, opt := range opts {
		opt(&options)
	}

	// Set the log level to debug
	_ = os.Setenv("DEBUG", "true")

	random := uuid.New().String()
	tmpDir := fileutil.MustTempDir(fmt.Sprintf("dagu-test-%s", random))
	require.NoError(t, os.Setenv("DAGU_HOME", tmpDir))

	root := getProjectRoot(t)
	executablePath := path.Join(root, ".local", "bin", "dagu")
	_ = os.Setenv("DAGU_EXECUTABLE", executablePath)

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.Paths.Executable = executablePath
	cfg.Paths.LogDir = filepath.Join(tmpDir, "logs")
	if options.DAGsDir != "" {
		cfg.Paths.DAGsDir = options.DAGsDir
	}

	if options.ServerConfig != nil {
		cfg.Server = *options.ServerConfig
	}

	dagStore := localdag.New(cfg.Paths.DAGsDir, localdag.WithFlagsBaseDir(cfg.Paths.SuspendFlagsDir))
	runStore := localhistory.New(cfg.Paths.DAGRunsDir)
	procStore := localproc.New(cfg.Paths.ProcDir)

	historyManager := history.New(runStore, cfg.Paths.Executable, cfg.Global.WorkDir)

	helper := Helper{
		Context:      createDefaultContext(),
		Config:       cfg,
		HistoryMgr:   historyManager,
		DAGStore:     dagStore,
		HistoryStore: runStore,
		ProcStore:    procStore,

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

	ctx, cancel := context.WithCancel(helper.Context)
	helper.Context = ctx
	helper.Cancel = cancel

	// setup the default shell for reproducible result
	setShell(t, "sh")

	t.Cleanup(helper.Cleanup)
	return helper
}

// Helper provides test utilities and configuration
type Helper struct {
	Context       context.Context
	Cancel        context.CancelFunc
	Config        *config.Config
	LoggingOutput *SyncBuffer
	DAGStore      models.DAGStore
	HistoryStore  models.DAGRunStore
	HistoryMgr    history.DAGRunManager
	ProcStore     models.ProcStore

	tmpDir string
}

// Cleanup removes temporary test directories
func (h Helper) Cleanup() {
	if h.Cancel != nil {
		h.Cancel()
	}
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

	require.Eventually(t, func() bool {
		latest, err := d.HistoryMgr.GetLatestStatus(d.Context, d.DAG)
		if err != nil {
			return false
		}
		t.Logf("latest status=%s errors=%v", latest.Status.String(), latest.Errors())
		return latest.Status == expected
	}, time.Second*5, time.Second)
}

func (d *DAG) AssertHistoryCount(t *testing.T, expected int) {
	t.Helper()

	// the +1 to the limit is needed to ensure that the number of therunstore
	// entries is exactly the expected number
	runstore := d.HistoryMgr.ListRecentStatus(d.Context, d.Name, expected+1)
	require.Len(t, runstore, expected)
}

func (d *DAG) AssertCurrentStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	assert.Eventually(t, func() bool {
		curr, _ := d.HistoryMgr.GetCurrentStatus(d.Context, d.DAG, "")
		if curr == nil {
			return false
		}
		t.Logf("current status=%s errors=%v", curr.Status.String(), curr.Errors())
		return curr.Status == expected
	}, time.Second*5, time.Second)
}

// AssertOutputs checks the given outputs against the actual outputs of the DAG
// Note that this function does not respect dependencies between nodes
// making the outputs with the same key indeterministic
func (d *DAG) AssertOutputs(t *testing.T, outputs map[string]any) {
	t.Helper()

	status, err := d.HistoryMgr.GetLatestStatus(d.Context, d.DAG)
	require.NoError(t, err)

	// collect the actual outputs from the status
	var actualOutputs = make(map[string]string)
	for _, node := range status.Nodes {
		if node.OutputVariables == nil {
			continue
		}
		value, ok := node.OutputVariables.Load(node.Step.Output)
		if ok {
			actualOutputs[node.Step.Output] = value.(string)
		}
	}

	// compare the actual outputs with the expected outputs
	for key, expected := range outputs {
		if expected == "" {
			_, ok := actualOutputs[key]
			assert.False(t, ok, "expected output %q to be empty", key)
			continue
		}

		if actual, ok := actualOutputs[key]; ok {
			switch expected := expected.(type) {
			case string:
				assert.Equal(t, fmt.Sprintf("%s=%s", key, expected), actual)

			case Contains:
				assert.Contains(t, actual, string(expected), "expected output %q to include %q", key, expected)

			case []Contains:
				for _, c := range expected {
					assert.Contains(t, actual, string(c), "expected output %q to include %q", key, c)
				}

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

type Contains string

type AgentOption func(*Agent)

func WithAgentOptions(options agent.Options) AgentOption {
	return func(a *Agent) {
		a.opts = options
	}
}

func (d *DAG) Agent(opts ...AgentOption) *Agent {
	helper := &Agent{Helper: d.Helper, DAG: d.DAG}

	for _, opt := range opts {
		opt(helper)
	}

	var dagRunID string
	if helper.opts.RetryTarget != nil {
		dagRunID = helper.opts.RetryTarget.DAGRunID
	} else {
		dagRunID = genDAGRunID()
	}

	logDir := d.Config.Paths.LogDir
	logFile := filepath.Join(d.Config.Paths.LogDir, dagRunID+".log")
	root := digraph.NewDAGRunRef(d.Name, dagRunID)

	helper.Agent = agent.New(
		dagRunID,
		d.DAG,
		logDir,
		logFile,
		d.HistoryMgr,
		d.DAGStore,
		d.HistoryStore,
		d.ProcStore,
		root,
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

	err := a.Run(a.Context)
	assert.Error(t, err)

	status := a.Status().Status
	require.Equal(t, scheduler.StatusError.String(), status.String())
}

func (a *Agent) RunCancel(t *testing.T) {
	t.Helper()

	err := a.Run(a.Context)
	assert.NoError(t, err)

	status := a.Status().Status
	require.Equal(t, scheduler.StatusCancel.String(), status.String())
}

func (a *Agent) RunCheckErr(t *testing.T, expectedErr string) {
	t.Helper()

	err := a.Run(a.Context)
	require.Error(t, err, "expected error %q, got nil", expectedErr)
	require.Contains(t, err.Error(), expectedErr)
	status := a.Status()
	require.Equal(t, scheduler.StatusCancel.String(), status.Status.String())
}

func (a *Agent) RunSuccess(t *testing.T) {
	t.Helper()

	err := a.Run(a.Context)
	assert.NoError(t, err, "failed to run agent")

	status := a.Status().Status
	require.Equal(t, scheduler.StatusSuccess.String(), status.String(), "expected status %q, got %q", scheduler.StatusSuccess, status)

	// check all nodes are in success or skipped state
	for _, node := range a.Status().Nodes {
		status := node.Status
		if status == scheduler.NodeStatusSkipped || status == scheduler.NodeStatusSuccess {
			continue
		}
		t.Errorf("expected node %q to be in success state, got %q", node.Step.Name, status.String())
	}
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

// getShell returns the path to the default shell.
func setShell(t *testing.T, shell string) {
	t.Helper()

	shPath, err := exec.LookPath(shell)
	require.NoError(t, err, "failed to find shell")
	_ = os.Setenv("SHELL", shPath)
}

// genDAGRunID generates a new unique DAG run ID using UUID v7.
func genDAGRunID() string {
	id, err := uuid.NewV7()
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
	data, err := os.ReadFile(path) //nolint:gosec
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
