package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewChildDAGExecutor_LocalDAG(t *testing.T) {
	// Create a context with environment
	ctx := context.Background()

	// Create a parent DAG with local DAGs
	parentDAG := &digraph.DAG{
		Name: "parent",
		LocalDAGs: map[string]*digraph.DAG{
			"local-child": &digraph.DAG{
				Name: "local-child",
				Steps: []digraph.Step{
					{Name: "step1", Command: "echo hello"},
				},
				YamlData: []byte("name: local-child\nsteps:\n  - name: step1\n    command: echo hello"),
			},
		},
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

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
	parentDAG := &digraph.DAG{
		Name: "parent",
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

	// Mock the database call
	expectedDAG := &digraph.DAG{
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
	parentDAG := &digraph.DAG{
		Name: "parent",
		LocalDAGs: map[string]*digraph.DAG{
			"other-child": &digraph.DAG{Name: "other-child"},
		},
	}

	// Set up the environment
	mockDB := new(mockDatabase)
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG:        parentDAG,
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

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
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG:        &digraph.DAG{Name: "parent"},
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       map[string]string{"TEST_ENV": "value"},
			BaseEnv:    &baseEnv,
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

	// Create executor
	executor := &ChildDAGExecutor{
		DAG: &digraph.DAG{
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
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG:        &digraph.DAG{Name: "parent"},
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("parent", "root-123"),
			DAGRunID:   "parent-456",
			Envs:       make(map[string]string),
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

	executor := &ChildDAGExecutor{
		DAG: &digraph.DAG{Name: "test-child"},
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
	env := Env{
		DAGContext: digraph.DAGContext{
			DAG: &digraph.DAG{Name: "parent"},
			DB:  mockDB,
			// RootDAGRun is zero value
			DAGRunID: "parent-456",
			Envs:     make(map[string]string),
		},
		Variables: &digraph.SyncMap{},
		Step:      digraph.Step{},
		Envs:      make(map[string]string),
	}
	ctx = WithEnv(ctx, env)

	executor := &ChildDAGExecutor{
		DAG: &digraph.DAG{Name: "test-child"},
	}

	runParams := RunParams{
		RunID: "child-789",
	}

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
		DAG:      &digraph.DAG{Name: "test-child"},
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
		DAG:      &digraph.DAG{Name: "test-child"},
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

var _ digraph.Database = (*mockDatabase)(nil)

// mockDatabase is a mock implementation of digraph.Database
type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) GetDAG(ctx context.Context, name string) (*digraph.DAG, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *mockDatabase) GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun digraph.DAGRunRef) (*digraph.RunStatus, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.RunStatus), args.Error(1)
}

// IsChildDAGRunCompleted implements digraph.Database.
func (m *mockDatabase) IsChildDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun digraph.DAGRunRef) (bool, error) {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Bool(0), args.Error(1)
}

// RequestChildCancel implements digraph.Database.
func (m *mockDatabase) RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun digraph.DAGRunRef) error {
	args := m.Called(ctx, dagRunID, rootDAGRun)
	return args.Error(0)
}
