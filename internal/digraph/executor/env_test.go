package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv_VariablesMap(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func(env executor.Env) executor.Env
		expected map[string]string
	}{
		{
			name: "combines variables and envs",
			setupEnv: func(env executor.Env) executor.Env {
				env.Variables.Store("VAR1", "VAR1=value1")
				env.Variables.Store("VAR2", "VAR2=value2")
				env.Envs["ENV1"] = "env1"
				env.Envs["ENV2"] = "env2"
				return env
			},
			expected: map[string]string{
				"VAR1":                       "value1",
				"VAR2":                       "value2",
				"ENV1":                       "env1",
				"ENV2":                       "env2",
				digraph.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "envs override variables",
			setupEnv: func(env executor.Env) executor.Env {
				env.Variables.Store("SAME_KEY", "SAME_KEY=from_variables")
				env.Envs["SAME_KEY"] = "from_envs"
				return env
			},
			expected: map[string]string{
				"SAME_KEY":                   "from_envs",
				digraph.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "empty variables and envs",
			setupEnv: func(env executor.Env) executor.Env {
				return env
			},
			expected: map[string]string{
				digraph.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "only variables",
			setupEnv: func(env executor.Env) executor.Env {
				env.Variables.Store("VAR1", "VAR1=value1")
				env.Variables.Store("VAR2", "VAR2=value2")
				return env
			},
			expected: map[string]string{
				"VAR1":                       "value1",
				"VAR2":                       "value2",
				digraph.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "only envs",
			setupEnv: func(env executor.Env) executor.Env {
				env.Envs["ENV1"] = "env1"
				env.Envs["ENV2"] = "env2"
				return env
			},
			expected: map[string]string{
				"ENV1":                       "env1",
				"ENV2":                       "env2",
				digraph.EnvKeyDAGRunStepName: "test-step",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
			env = tt.setupEnv(env)

			result := env.VariablesMap()

			// Check that all expected keys exist with correct values
			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}

			// Also check PWD is set (from NewEnv)
			assert.NotEmpty(t, result["PWD"], "PWD should be set")
		})
	}
}

func TestNewEnv_WorkingDirectory(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		step      digraph.Step
		setupFunc func()
		checkFunc func(t *testing.T, env executor.Env)
	}{
		{
			name: "step with absolute directory",
			step: digraph.Step{
				Name: "test-step",
				Dir:  tempDir,
			},
			setupFunc: func() {},
			checkFunc: func(t *testing.T, env executor.Env) {
				// Resolve symlinks for comparison (macOS /var vs /private/var)
				expectedDir, _ := filepath.EvalSymlinks(tempDir)
				actualDir, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedDir, actualDir)
				// env.Envs["PWD"] should match env.WorkingDir
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "step with relative directory",
			step: digraph.Step{
				Name: "test-step",
				Dir:  "./subdir",
			},
			setupFunc: func() {
				// Create the subdirectory
				require.NoError(t, os.Chdir(tempDir))
				require.NoError(t, os.Mkdir("subdir", 0755))
			},
			checkFunc: func(t *testing.T, env executor.Env) {
				expectedDir := filepath.Join(tempDir, "subdir")
				// Resolve symlinks for comparison
				expectedResolved, _ := filepath.EvalSymlinks(expectedDir)
				actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedResolved, actualResolved)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "step with home directory notation",
			step: digraph.Step{
				Name: "test-step",
				Dir:  "~/testdir",
			},
			setupFunc: func() {
				// Create a directory in home
				homeDir, _ := os.UserHomeDir()
				testDir := filepath.Join(homeDir, "testdir")
				require.NoError(t, os.MkdirAll(testDir, 0755))
			},
			checkFunc: func(t *testing.T, env executor.Env) {
				homeDir, _ := os.UserHomeDir()
				expectedDir := filepath.Join(homeDir, "testdir")
				// Resolve symlinks for comparison
				expectedResolved, _ := filepath.EvalSymlinks(expectedDir)
				actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedResolved, actualResolved)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "step with no directory uses current working directory",
			step: digraph.Step{
				Name: "test-step",
			},
			setupFunc: func() {
				require.NoError(t, os.Chdir(tempDir))
			},
			checkFunc: func(t *testing.T, env executor.Env) {
				// Resolve symlinks for comparison (macOS /var vs /private/var)
				expectedDir, _ := filepath.EvalSymlinks(tempDir)
				actualDir, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedDir, actualDir)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "step with non-existent directory",
			step: digraph.Step{
				Name: "test-step",
				Dir:  "/non/existent/directory",
			},
			setupFunc: func() {
				require.NoError(t, os.Chdir(tempDir))
			},
			checkFunc: func(t *testing.T, env executor.Env) {
				// Non-existent directory gets resolved to absolute path
				assert.Equal(t, "/non/existent/directory", env.WorkingDir)
				assert.Equal(t, "/non/existent/directory", env.Envs["PWD"])
			},
		},
		{
			name: "step with environment variable in path",
			step: digraph.Step{
				Name: "test-step",
				Dir:  "$HOME/testdir",
			},
			setupFunc: func() {
				// Create a directory in home
				homeDir, _ := os.UserHomeDir()
				testDir := filepath.Join(homeDir, "testdir")
				require.NoError(t, os.MkdirAll(testDir, 0755))
			},
			checkFunc: func(t *testing.T, env executor.Env) {
				homeDir, _ := os.UserHomeDir()
				expectedDir := filepath.Join(homeDir, "testdir")
				// Resolve symlinks for comparison
				expectedResolved, _ := filepath.EvalSymlinks(expectedDir)
				actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedResolved, actualResolved)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFunc()

			ctx := context.Background()
			env := executor.NewEnv(ctx, tt.step)

			// Check that DAG_RUN_STEP_NAME is always set
			assert.Equal(t, tt.step.Name, env.Envs[digraph.EnvKeyDAGRunStepName])

			// Run test-specific checks
			tt.checkFunc(t, env)
		})
	}
}

func TestNewEnv_BasicFields(t *testing.T) {
	ctx := context.Background()
	step := digraph.Step{
		Name:    "test-step",
		Command: "echo hello",
		Args:    []string{"arg1", "arg2"},
	}

	env := executor.NewEnv(ctx, step)

	// Check basic fields
	assert.Equal(t, step, env.Step)
	assert.NotNil(t, env.Variables)
	assert.NotNil(t, env.Envs)
	assert.NotNil(t, env.StepMap)
	assert.Equal(t, "test-step", env.Envs[digraph.EnvKeyDAGRunStepName])

	// Check that PWD is set
	assert.NotEmpty(t, env.Envs["PWD"])

	// Check that WorkingDir is set
	assert.NotEmpty(t, env.WorkingDir)
}
