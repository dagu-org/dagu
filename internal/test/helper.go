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
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/builder"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/filedag"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/persistence/fileproc"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/persistence/fileserviceregistry"
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
	CoordinatorHost      string
	CoordinatorPort      int
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

	// Set the CI flag
	_ = os.Setenv("CI", "true")

	// Disable the DAG run queue for tests
	_ = os.Setenv("DISABLE_DAG_RUN_QUEUE", "true")

	random := uuid.New().String()
	tmpDir := fileutil.MustTempDir(fmt.Sprintf("dagu-test-%s", random))
	require.NoError(t, os.Setenv("DAGU_HOME", tmpDir))

	root := getProjectRoot(t)
	executablePath := path.Join(root, ".local", "bin", "dagu")
	_ = os.Setenv("DAGU_EXECUTABLE", executablePath)

	ctx := createDefaultContext()
	cfg, err := config.Load()
	require.NoError(t, err)

	ctx = config.WithConfig(ctx, cfg)

	cfg.Paths.Executable = executablePath
	cfg.Paths.LogDir = filepath.Join(tmpDir, "logs")
	if options.DAGsDir != "" {
		cfg.Paths.DAGsDir = options.DAGsDir
	}

	if options.ServerConfig != nil {
		cfg.Server = *options.ServerConfig
	}

	if options.CoordinatorHost != "" {
		cfg.Coordinator.Host = options.CoordinatorHost
	}
	if options.CoordinatorPort != 0 {
		cfg.Coordinator.Port = options.CoordinatorPort
	}

	dagStore := filedag.New(cfg.Paths.DAGsDir, filedag.WithFlagsBaseDir(cfg.Paths.SuspendFlagsDir), filedag.WithSkipExamples(true))
	runStore := filedagrun.New(cfg.Paths.DAGRunsDir)
	procStore := fileproc.New(cfg.Paths.ProcDir)
	queueStore := filequeue.New(cfg.Paths.QueueDir)
	serviceMonitor := fileserviceregistry.New(cfg.Paths.ServiceRegistryDir)

	drm := dagrun.New(runStore, procStore, cfg)

	helper := Helper{
		Context:         ctx,
		Config:          cfg,
		DAGRunMgr:       drm,
		DAGStore:        dagStore,
		DAGRunStore:     runStore,
		ProcStore:       procStore,
		QueueStore:      queueStore,
		ServiceRegistry: serviceMonitor,
		SubCmdBuilder:   dagrun.NewSubCmdBuilder(cfg),

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
	if runtime.GOOS == "windows" {
		// On Windows, try PowerShell first, then cmd
		if _, err := exec.LookPath("powershell"); err == nil {
			setShell(t, "powershell")
		} else if _, err := exec.LookPath("cmd"); err == nil {
			setShell(t, "cmd")
		} else {
			t.Fatal("No suitable shell found on Windows")
		}
	} else {
		setShell(t, "sh")
	}

	t.Cleanup(helper.Cleanup)
	return helper
}

// Helper provides test utilities and configuration
type Helper struct {
	Context         context.Context
	Cancel          context.CancelFunc
	Config          *config.Config
	LoggingOutput   *SyncBuffer
	DAGStore        models.DAGStore
	DAGRunStore     models.DAGRunStore
	DAGRunMgr       dagrun.Manager
	ProcStore       models.ProcStore
	QueueStore      models.QueueStore
	ServiceRegistry models.ServiceRegistry
	SubCmdBuilder   *dagrun.SubCmdBuilder

	tmpDir string
}

// Cleanup removes temporary test directories
func (h Helper) Cleanup() {
	if h.Cancel != nil {
		h.Cancel()
	}
	_ = os.RemoveAll(h.tmpDir)
}

// TempFile creates a temp file with specified name and content.
func (h Helper) TempFile(t *testing.T, name string, data []byte) string {
	t.Helper()

	filename := filepath.Join(h.tmpDir, name)
	err := os.WriteFile(filename, data, 0600)
	require.NoError(t, err)
	return filename
}

// DAG creates a test DAG from YAML content
func (h Helper) DAG(t *testing.T, yamlContent string) DAG {
	t.Helper()

	err := os.MkdirAll(h.Config.Paths.DAGsDir, 0750)
	require.NoError(t, err, "failed to create DAGs directory %q", h.Config.Paths.DAGsDir)

	// Generate a unique filename for the test DAG
	filename := fmt.Sprintf("%s.yaml", uuid.New().String())
	testFile := filepath.Join(h.Config.Paths.DAGsDir, filename)
	err = os.WriteFile(testFile, []byte(yamlContent), 0600)
	require.NoError(t, err, "failed to write test DAG")

	dag, err := builder.Load(h.Context, testFile)
	require.NoError(t, err, "failed to load test DAG")

	return DAG{
		Helper: &h,
		DAG:    dag,
	}
}

// CreateDAGFile creates a DAG file in a given directory for tests that need separate DAG files
func (h Helper) CreateDAGFile(t *testing.T, dir string, name string, yamlContent []byte) string {
	t.Helper()

	// Create the directory if it doesn't exist
	err := os.MkdirAll(dir, 0750)
	require.NoError(t, err, "failed to create directory %q", dir)

	if !fileutil.IsYAMLFile(name) {
		name = fmt.Sprintf("%s.yaml", name)
	}

	dagFile := filepath.Join(dir, name)
	err = os.WriteFile(dagFile, yamlContent, 0600)
	require.NoError(t, err, "failed to write DAG file %q", name)

	t.Cleanup(func() { _ = os.Remove(dagFile) })

	return dagFile
}

func (h Helper) DAGExpectError(t *testing.T, name string, expectedErr string) {
	t.Helper()

	filePath := TestdataPath(t, name)
	_, err := builder.Load(h.Context, filePath)
	require.Error(t, err, "expected error loading test DAG %q", name)
	require.Contains(t, err.Error(), expectedErr, "expected error %q, got %q", expectedErr, err.Error())
}

type DAG struct {
	*Helper
	*core.DAG
}

func (d *DAG) AssertLatestStatus(t *testing.T, expected status.Status) {
	t.Helper()

	require.Eventually(t, func() bool {
		latest, err := d.DAGRunMgr.GetLatestStatus(d.Context, d.DAG)
		if err != nil {
			return false
		}
		t.Logf("latest status=%s errors=%v", latest.Status.String(), latest.Errors())
		return latest.Status == expected
	}, time.Second*10, time.Second)
}

func (d *DAG) AssertDAGRunCount(t *testing.T, expected int) {
	t.Helper()

	// the +1 to the limit is needed to ensure that the number of dag-run
	// entries is exactly the expected number
	runstore := d.DAGRunMgr.ListRecentStatus(d.Context, d.Name, expected+1)
	require.Len(t, runstore, expected)
}

func (d *DAG) AssertCurrentStatus(t *testing.T, expected status.Status) {
	t.Helper()

	assert.Eventually(t, func() bool {
		curr, _ := d.DAGRunMgr.GetCurrentStatus(d.Context, d.DAG, "")
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

	status, err := d.DAGRunMgr.GetLatestStatus(d.Context, d.DAG)
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

func WithDAGRunID(dagRunID string) AgentOption {
	return func(a *Agent) {
		a.dagRunID = dagRunID
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
	} else if helper.dagRunID != "" {
		dagRunID = helper.dagRunID
	} else {
		dagRunID = genDAGRunID()
	}

	logDir := d.Config.Paths.LogDir
	logFile := filepath.Join(d.Config.Paths.LogDir, dagRunID+".log")
	root := core.NewDAGRunRef(d.Name, dagRunID)

	helper.Agent = agent.New(
		dagRunID,
		d.DAG,
		logDir,
		logFile,
		d.DAGRunMgr,
		d.DAGStore,
		d.DAGRunStore,
		d.ServiceRegistry,
		root,
		d.Config.Global.Peer,
		helper.opts,
	)

	return helper
}

type Agent struct {
	*Helper
	*core.DAG
	*agent.Agent
	opts     agent.Options
	dagRunID string // the dag-run ID for this agent
}

func (a *Agent) RunError(t *testing.T) {
	t.Helper()

	err := a.Run(a.Context)
	assert.Error(t, err)

	st := a.Status(a.Context).Status
	require.Equal(t, status.Error.String(), st.String())
}

func (a *Agent) RunCancel(t *testing.T) {
	t.Helper()

	proc, err := a.ProcStore.Acquire(a.Context, a.DAG.ProcGroup(), core.DAGRunRef{
		Name: a.DAG.Name,
		ID:   a.dagRunID,
	})
	require.NoError(t, err, "failed to acquire proc")
	t.Cleanup(func() {
		proc.Stop(a.Context)
	})

	err = a.Run(a.Context)
	assert.NoError(t, err)

	st := a.Status(a.Context).Status
	require.Equal(t, status.Cancel.String(), st.String())
}

func (a *Agent) RunCheckErr(t *testing.T, expectedErr string) {
	t.Helper()

	err := a.Run(a.Context)
	require.Error(t, err, "expected error %q, got nil", expectedErr)
	require.Contains(t, err.Error(), expectedErr)
	st := a.Status(a.Context)
	require.Equal(t, status.Cancel.String(), st.Status.String())
}

func (a *Agent) RunSuccess(t *testing.T) {
	t.Helper()

	err := a.Run(a.Context)
	assert.NoError(t, err, "failed to run agent")

	st := a.Status(a.Context).Status
	require.Equal(t, status.Success.String(), st.String(), "expected status %q, got %q", status.Success, st)

	// check all nodes are in success or skipped state
	for _, node := range a.Status(a.Context).Nodes {
		st := node.Status
		if st == status.NodeSkipped || st == status.NodeSuccess {
			continue
		}
		t.Errorf("expected node %q to be in success state, got %q", node.Step.Name, st.String())
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

// genDAGRunID generates a new unique dag-run ID using UUID v7.
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

	return filepath.Join(rootDir, "internal", "testdata", filepath.Clean(filename))
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
