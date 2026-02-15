package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSubDAGExecutor_LocalDAG(t *testing.T) {
	t.Parallel()

	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG with local DAGs
	parentDAG := &core.DAG{
		Name: "parent",
		LocalDAGs: map[string]*core.DAG{
			"local-child": {
				Name: "local-child",
				Steps: []core.Step{
					{Name: "step1", Commands: []core.CommandEntry{{Command: "echo", Args: []string{"hello"}}}},
				},
				YamlData: []byte("name: local-child\nsteps:\n  - name: step1\n    command: echo hello"),
			},
		},
	}

	// Set up the DAG context
	mockDB := new(mockDatabase)
	dagCtx := exec1.Context{
		DAG:        parentDAG,
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("parent", "root-123"),
		DAGRunID:   "parent-456",
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	// Test creating executor for local DAG
	executor, err := NewSubDAGExecutor(ctx, "local-child")
	require.NoError(t, err)
	require.NotNil(t, executor)

	// Verify it has yaml data (indicating it's local)
	assert.Equal(t, "local-child", executor.DAG.Name)
	assert.NotEmpty(t, executor.tempFile)
	assert.Contains(t, executor.tempFile, "local-child")
	assert.Contains(t, executor.tempFile, ".yaml")

	// Verify the temp file was created
	assert.FileExists(t, executor.tempFile)

	// Read and verify the content
	content, err := os.ReadFile(executor.tempFile)
	require.NoError(t, err)
	assert.Equal(t, parentDAG.LocalDAGs["local-child"].YamlData, content)

	// Cleanup
	err = executor.Cleanup(ctx)
	assert.NoError(t, err)
	assert.NoFileExists(t, executor.tempFile)
}

func TestNewSubDAGExecutor_RegularDAG(t *testing.T) {
	t.Parallel()

	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG without local DAGs
	parentDAG := &core.DAG{
		Name: "parent",
	}

	// Set up the DAG context
	mockDB := new(mockDatabase)
	dagCtx := exec1.Context{
		DAG:        parentDAG,
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("parent", "root-123"),
		DAGRunID:   "parent-456",
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	// Mock the database call
	expectedDAG := &core.DAG{
		Name:     "regular-child",
		Location: "/path/to/regular-child.yaml",
	}
	mockDB.On("GetDAG", ctx, "regular-child").Return(expectedDAG, nil)

	// Test creating executor for regular DAG
	executor, err := NewSubDAGExecutor(ctx, "regular-child")
	require.NoError(t, err)
	require.NotNil(t, executor)

	// Verify it doesn't have yaml data (not local)
	assert.Equal(t, "regular-child", executor.DAG.Name)
	assert.Empty(t, executor.tempFile)

	// Cleanup should do nothing for regular DAGs
	err = executor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDB.AssertExpectations(t)
}

func TestNewSubDAGExecutor_NotFound(t *testing.T) {
	t.Parallel()

	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG without the requested local DAG
	parentDAG := &core.DAG{
		Name: "parent",
		LocalDAGs: map[string]*core.DAG{
			"other-child": {Name: "other-child"},
		},
	}

	// Set up the DAG context
	mockDB := new(mockDatabase)
	dagCtx := exec1.Context{
		DAG:        parentDAG,
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("parent", "root-123"),
		DAGRunID:   "parent-456",
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	// Mock the database call to return not found
	mockDB.On("GetDAG", ctx, "non-existent").Return(nil, assert.AnError)

	// Test creating executor for non-existent DAG
	executor, err := NewSubDAGExecutor(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, executor)
	assert.Contains(t, err.Error(), "failed to find DAG")

	mockDB.AssertExpectations(t)
}

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	// Create a context with environment
	ctx := context.Background()

	// Set up the DAG context
	mockDB := new(mockDatabase)
	baseEnv := config.NewBaseEnv(nil)
	dagCtx := exec1.Context{
		DAG:        &core.DAG{Name: "parent"},
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("parent", "root-123"),
		DAGRunID:   "parent-456",
		BaseEnv:    &baseEnv,
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	// Create executor
	executor := &SubDAGExecutor{
		DAG: &core.DAG{
			Name:     "test-child",
			Location: "/path/to/test.yaml",
		},
		killed: make(chan struct{}),
	}

	// Build command
	runParams := RunParams{
		RunID:  "child-789",
		Params: "param1=value1 param2=value2",
	}

	cmd, err := executor.buildCommand(ctx, runParams, "/work/dir")
	require.NoError(t, err)
	require.NotNil(t, cmd)

	// Verify command properties
	assert.Equal(t, "/work/dir", cmd.Dir)

	// Verify args
	args := cmd.Args
	assert.Contains(t, args, "start")
	assert.Contains(t, args, "--root=parent:root-123")
	assert.Contains(t, args, "--parent=parent:parent-456")
	assert.Contains(t, args, "--run-id=child-789")
	assert.Contains(t, args, "/path/to/test.yaml")
	assert.Contains(t, args, "--")
	assert.Contains(t, args, "param1=value1 param2=value2")
}

func TestBuildCommand_NoRunID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Set up the DAG context
	mockDB := new(mockDatabase)
	dagCtx := exec1.Context{
		DAG:        &core.DAG{Name: "parent"},
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("parent", "root-123"),
		DAGRunID:   "parent-456",
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	executor := &SubDAGExecutor{
		DAG:    &core.DAG{Name: "test-child"},
		killed: make(chan struct{}),
	}

	// Build command without RunID
	runParams := RunParams{
		RunID: "", // Empty RunID
	}

	cmd, err := executor.buildCommand(ctx, runParams, "/work/dir")
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Contains(t, err.Error(), "DAG run ID is not set")
}

func TestBuildCommand_NoRootDAGRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Set up the DAG context without RootDAGRun
	mockDB := new(mockDatabase)
	dagCtx := exec1.Context{
		DAG: &core.DAG{Name: "parent"},
		DB:  mockDB,
		// RootDAGRun is zero value
		DAGRunID: "parent-456",
	}
	ctx = exec1.WithContext(ctx, dagCtx)

	executor := &SubDAGExecutor{
		DAG: &core.DAG{Name: "test-child"},
	}

	runParams := RunParams{RunID: "child-789"}

	cmd, err := executor.buildCommand(ctx, runParams, "/work/dir")
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Contains(t, err.Error(), "root DAG run ID is not set")
}

func TestCleanup_LocalDAG(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a temporary file using t.TempDir() for automatic cleanup
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(tempFile, []byte("test content"), 0600)
	require.NoError(t, err)

	executor := &SubDAGExecutor{
		DAG:      &core.DAG{Name: "test-child"},
		tempFile: tempFile,
		killed:   make(chan struct{}),
	}

	// Verify file exists
	assert.FileExists(t, tempFile)

	// Cleanup
	err = executor.Cleanup(ctx)
	assert.NoError(t, err)

	// Verify file is removed
	assert.NoFileExists(t, tempFile)
}

func TestCleanup_NonExistentFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	executor := &SubDAGExecutor{
		DAG:      &core.DAG{Name: "test-child"},
		tempFile: "/non/existent/file.yaml",
		killed:   make(chan struct{}),
	}

	// Cleanup should not error on non-existent file
	err := executor.Cleanup(ctx)
	assert.NoError(t, err)
}

func TestExecutablePath(t *testing.T) {
	// Test with environment variable
	testPath := "/custom/path/to/boltbase"
	_ = os.Setenv("BOLTBASE_EXECUTABLE", testPath)
	defer func() { _ = os.Unsetenv("BOLTBASE_EXECUTABLE") }()

	path, err := executablePath()
	assert.NoError(t, err)
	assert.Equal(t, testPath, path)

	// Test without environment variable
	_ = os.Unsetenv("BOLTBASE_EXECUTABLE")
	path, err = executablePath()
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestSubDAGExecutor_Kill_MixedProcesses(t *testing.T) {
	t.Parallel()

	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a DAG context
	dagCtx := exec1.Context{
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("root-dag", "root-run-id"),
		DAGRunID:   "parent-run-id",
	}

	// Create a sub DAG
	subDAG := &core.DAG{
		Name: "sub-dag",
	}

	// Create child executor with both local and distributed processes
	executor := &SubDAGExecutor{
		DAG:    subDAG,
		dagCtx: dagCtx,
		cmds: map[string]*exec.Cmd{
			"local-run-1": {Process: &os.Process{Pid: 999999999}},
			"local-run-2": {Process: &os.Process{Pid: 999999998}},
		},
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
		killed: make(chan struct{}),
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", dagCtx.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", dagCtx.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error (killProcessGroup will fail for fake PIDs but that's expected)
	// We're mainly testing that both distributed and local processes are handled
	assert.Error(t, err) // Expected error from trying to kill fake processes

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestSubDAGExecutor_Kill_OnlyDistributed(t *testing.T) {
	t.Parallel()

	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a DAG context
	dagCtx := exec1.Context{
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("root-dag", "root-run-id"),
		DAGRunID:   "parent-run-id",
	}

	// Create a sub DAG
	subDAG := &core.DAG{
		Name: "sub-dag",
	}

	// Create child executor with only distributed processes
	executor := &SubDAGExecutor{
		DAG:    subDAG,
		dagCtx: dagCtx,
		cmds:   make(map[string]*exec.Cmd),
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
		killed: make(chan struct{}),
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", dagCtx.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", dagCtx.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestSubDAGExecutor_Kill_OnlyLocal(t *testing.T) {
	t.Parallel()

	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a DAG context
	dagCtx := exec1.Context{
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("root-dag", "root-run-id"),
		DAGRunID:   "parent-run-id",
	}

	// Create a sub DAG
	subDAG := &core.DAG{
		Name: "sub-dag",
	}

	// Create child executor with only local processes
	executor := &SubDAGExecutor{
		DAG:    subDAG,
		dagCtx: dagCtx,
		cmds: map[string]*exec.Cmd{
			"local-run-1": {Process: &os.Process{Pid: 999999999}},
		},
		distributedRuns: make(map[string]bool),
		killed:          make(chan struct{}),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify error (from trying to kill fake process)
	assert.Error(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}

func TestSubDAGExecutor_Kill_Empty(t *testing.T) {
	t.Parallel()

	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a DAG context
	dagCtx := exec1.Context{
		DB:         mockDB,
		RootDAGRun: exec1.NewDAGRunRef("root-dag", "root-run-id"),
		DAGRunID:   "parent-run-id",
	}

	// Create a sub DAG
	subDAG := &core.DAG{
		Name: "sub-dag",
	}

	// Create child executor with no processes
	executor := &SubDAGExecutor{
		DAG:             subDAG,
		dagCtx:          dagCtx,
		cmds:            make(map[string]*exec.Cmd),
		distributedRuns: make(map[string]bool),
		killed:          make(chan struct{}),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}

var _ exec1.Database = (*mockDatabase)(nil)

// mockDatabase is a mock implementation of core.Database
type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) GetDAG(ctx context.Context, name string) (*core.DAG, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDatabase) GetSubDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun exec1.DAGRunRef) (*exec1.RunStatus, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*exec1.RunStatus), args.Error(1)
}

// IsSubDAGRunCompleted implements core.Database.
func (m *mockDatabase) IsSubDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun exec1.DAGRunRef) (bool, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Bool(0), args.Error(1)
}

// RequestChildCancel implements core.Database.
func (m *mockDatabase) RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun exec1.DAGRunRef) error {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Error(0)
}
