package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/common/maputil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewChildDAGExecutor_LocalDAG(t *testing.T) {
	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG with local DAGs
	parentDAG := &core.DAG{
		Name: "parent",
		LocalDAGs: map[string]*core.DAG{
			"local-child": &core.DAG{
				Name: "local-child",
				Steps: []core.Step{
					{Name: "step1", Command: "echo hello"},
				},
				YamlData: []byte("name: local-child\nsteps:\n  - name: step1\n    command: echo hello"),
			},
		},
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	// Test creating executor for local DAG
	executor, err := NewChildDAGExecutor(ctx, "local-child")
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

func TestNewChildDAGExecutor_RegularDAG(t *testing.T) {
	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG without local DAGs
	parentDAG := &core.DAG{
		Name: "parent",
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	// Mock the database call
	expectedDAG := &core.DAG{
		Name:     "regular-child",
		Location: "/path/to/regular-child.yaml",
	}
	mockDB.On("GetDAG", ctx, "regular-child").Return(expectedDAG, nil)

	// Test creating executor for regular DAG
	executor, err := NewChildDAGExecutor(ctx, "regular-child")
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

func TestNewChildDAGExecutor_NotFound(t *testing.T) {
	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG without the requested local DAG
	parentDAG := &core.DAG{
		Name: "parent",
		LocalDAGs: map[string]*core.DAG{
			"other-child": &core.DAG{Name: "other-child"},
		},
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	// Mock the database call to return not found
	mockDB.On("GetDAG", ctx, "non-existent").Return(nil, assert.AnError)

	// Test creating executor for non-existent DAG
	executor, err := NewChildDAGExecutor(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, executor)
	assert.Contains(t, err.Error(), "failed to find DAG")

	mockDB.AssertExpectations(t)
}

func TestBuildCommand(t *testing.T) {
	// Create a context with environment
	ctx := context.Background()

	// Set up the environment
	mockDB := new(mockDatabase)
	baseEnv := config.NewBaseEnv(nil)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG:        &core.DAG{Name: "parent"},
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       map[string]string{"TEST_ENV": "value"},
			BaseEnv:    &baseEnv,
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	// Create executor
	executor := &ChildDAGExecutor{
		DAG: &core.DAG{
			Name:     "test-child",
			Location: "/path/to/test.yaml",
		},
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
	assert.Contains(t, cmd.Env, "TEST_ENV=value")

	// Verify args
	args := cmd.Args
	assert.Contains(t, args, "start")
	assert.Contains(t, args, "--root=parent:root-123")
	assert.Contains(t, args, "--parent=parent:parent-456")
	assert.Contains(t, args, "--run-id=child-789")
	assert.Contains(t, args, "--no-queue")
	assert.Contains(t, args, "/path/to/test.yaml")
	assert.Contains(t, args, "--")
	assert.Contains(t, args, "param1=value1 param2=value2")
}

func TestBuildCommand_NoRunID(t *testing.T) {
	ctx := context.Background()

	// Set up the environment
	mockDB := new(mockDatabase)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG:        &core.DAG{Name: "parent"},
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	executor := &ChildDAGExecutor{
		DAG: &core.DAG{Name: "test-child"},
	}

	// Build command without RunID
	runParams := RunParams{
		RunID: "", // Empty RunID
	}

	cmd, err := executor.buildCommand(ctx, runParams, "/work/dir")
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Contains(t, err.Error(), "dag-run ID is not set")
}

func TestBuildCommand_NoRootDAGRun(t *testing.T) {
	ctx := context.Background()

	// Set up the environment without RootDAGRun
	mockDB := new(mockDatabase)
	env := core.Env{
		DAGContext: core.DAGContext{
			DAG: &core.DAG{Name: "parent"},
			DB:  mockDB,
			// RootDAGRun is zero value
			DAGRunID: "parent-456",
			Envs:     make(map[string]string),
		},
		Variables: &maputil.SyncMap{},
		Step:      core.Step{},
		Envs:      make(map[string]string),
	}
	ctx = core.WithEnv(ctx, env)

	executor := &ChildDAGExecutor{
		DAG: &core.DAG{Name: "test-child"},
	}

	runParams := RunParams{RunID: "child-789"}

	cmd, err := executor.buildCommand(ctx, runParams, "/work/dir")
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Contains(t, err.Error(), "root dag-run ID is not set")
}

func TestCleanup_LocalDAG(t *testing.T) {
	ctx := context.Background()

	// Create a temporary file
	tempDir := filepath.Join(os.TempDir(), "dagu-test")
	err := os.MkdirAll(tempDir, 0750)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempFile := filepath.Join(tempDir, "test.yaml")
	err = os.WriteFile(tempFile, []byte("test content"), 0600)
	require.NoError(t, err)

	executor := &ChildDAGExecutor{
		DAG:      &core.DAG{Name: "test-child"},
		tempFile: tempFile,
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
	ctx := context.Background()

	executor := &ChildDAGExecutor{
		DAG:      &core.DAG{Name: "test-child"},
		tempFile: "/non/existent/file.yaml",
	}

	// Cleanup should not error on non-existent file
	err := executor.Cleanup(ctx)
	assert.NoError(t, err)
}

func TestCreateTempDAGFile(t *testing.T) {
	dagName := "test-dag"
	yamlData := []byte("name: test-dag\nsteps:\n  - name: step1\n    command: echo test")

	tempFile, err := createTempDAGFile(dagName, yamlData)
	require.NoError(t, err)
	require.NotEmpty(t, tempFile)
	defer func() { _ = os.Remove(tempFile) }()

	// Verify file exists and has correct content
	assert.FileExists(t, tempFile)
	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, yamlData, content)

	// Verify file name pattern
	assert.Contains(t, tempFile, "test-dag")
	assert.Contains(t, tempFile, ".yaml")
}

func TestExecutablePath(t *testing.T) {
	// Test with environment variable
	testPath := "/custom/path/to/dagu"
	_ = os.Setenv("DAGU_EXECUTABLE", testPath)
	defer func() { _ = os.Unsetenv("DAGU_EXECUTABLE") }()

	path, err := executablePath()
	assert.NoError(t, err)
	assert.Equal(t, testPath, path)

	// Test without environment variable
	_ = os.Unsetenv("DAGU_EXECUTABLE")
	path, err = executablePath()
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestChildDAGExecutor_Kill_MixedProcesses(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := core.Env{
		DAGContext: core.DAGContext{
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = core.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &core.DAG{
		Name: "child-dag",
	}

	// Create child executor with both local and distributed processes
	executor := &ChildDAGExecutor{
		DAG: childDAG,
		env: env,
		cmds: map[string]*exec.Cmd{
			"local-run-1": &exec.Cmd{Process: &os.Process{Pid: 1234}},
			"local-run-2": &exec.Cmd{Process: &os.Process{Pid: 5678}},
		},
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", env.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", env.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error (killProcessGroup will fail for fake PIDs but that's expected)
	// We're mainly testing that both distributed and local processes are handled
	assert.Error(t, err) // Expected error from trying to kill fake processes

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestChildDAGExecutor_Kill_OnlyDistributed(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := core.Env{
		DAGContext: core.DAGContext{
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = core.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &core.DAG{
		Name: "child-dag",
	}

	// Create child executor with only distributed processes
	executor := &ChildDAGExecutor{
		DAG:  childDAG,
		env:  env,
		cmds: make(map[string]*exec.Cmd),
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", env.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", env.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestChildDAGExecutor_Kill_OnlyLocal(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := core.Env{
		DAGContext: core.DAGContext{
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = core.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &core.DAG{
		Name: "child-dag",
	}

	// Create child executor with only local processes
	executor := &ChildDAGExecutor{
		DAG: childDAG,
		env: env,
		cmds: map[string]*exec.Cmd{
			"local-run-1": &exec.Cmd{Process: &os.Process{Pid: 1234}},
		},
		distributedRuns: make(map[string]bool),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify error (from trying to kill fake process)
	assert.Error(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}

func TestChildDAGExecutor_Kill_Empty(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := core.Env{
		DAGContext: core.DAGContext{
			DB:         mockDB,
			RootDAGRun: core.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = core.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &core.DAG{
		Name: "child-dag",
	}

	// Create child executor with no processes
	executor := &ChildDAGExecutor{
		DAG:             childDAG,
		env:             env,
		cmds:            make(map[string]*exec.Cmd),
		distributedRuns: make(map[string]bool),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}

var _ core.Database = (*mockDatabase)(nil)

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

func (m *mockDatabase) GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) (*core.RunStatus, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.RunStatus), args.Error(1)
}

// IsChildDAGRunCompleted implements core.Database.
func (m *mockDatabase) IsChildDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) (bool, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Bool(0), args.Error(1)
}

// RequestChildCancel implements core.Database.
func (m *mockDatabase) RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) error {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Error(0)
}
