package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv_VariablesMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupEnv func(env execution.Env) execution.Env
		expected map[string]string
	}{
		{
			name: "CombinesVariablesAndEnvs",
			setupEnv: func(env execution.Env) execution.Env {
				env.Variables.Store("VAR1", "VAR1=value1")
				env.Variables.Store("VAR2", "VAR2=value2")
				env.Envs["ENV1"] = "env1"
				env.Envs["ENV2"] = "env2"
				return env
			},
			expected: map[string]string{
				"VAR1":                         "value1",
				"VAR2":                         "value2",
				"ENV1":                         "env1",
				"ENV2":                         "env2",
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "EnvsOverrideVariables",
			setupEnv: func(env execution.Env) execution.Env {
				env.Variables.Store("SAME_KEY", "SAME_KEY=from_variables")
				env.Envs["SAME_KEY"] = "from_envs"
				return env
			},
			expected: map[string]string{
				"SAME_KEY":                     "from_envs",
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "EmptyVariablesAndEnvs",
			setupEnv: func(env execution.Env) execution.Env {
				return env
			},
			expected: map[string]string{
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "OnlyVariables",
			setupEnv: func(env execution.Env) execution.Env {
				env.Variables.Store("VAR1", "VAR1=value1")
				env.Variables.Store("VAR2", "VAR2=value2")
				return env
			},
			expected: map[string]string{
				"VAR1":                         "value1",
				"VAR2":                         "value2",
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "OnlyEnvs",
			setupEnv: func(env execution.Env) execution.Env {
				env.Envs["ENV1"] = "env1"
				env.Envs["ENV2"] = "env2"
				return env
			},
			expected: map[string]string{
				"ENV1":                         "env1",
				"ENV2":                         "env2",
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory to ensure we have a valid working directory
			tempDir := t.TempDir()
			originalWd, err := os.Getwd()
			if err == nil {
				defer func() { _ = os.Chdir(originalWd) }()
			}
			require.NoError(t, os.Chdir(tempDir))

			ctx := context.Background()
			env := execution.NewEnv(ctx, core.Step{Name: "test-step"})
			env = tt.setupEnv(env)

			result := env.VariablesMap()

			// Check that all expected keys exist with correct values
			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}
		})
	}
}

func TestNewEnv_WorkingDirectory(t *testing.T) {
	// Don't run in parallel since we're changing working directory

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
		step      core.Step
		setupFunc func()
		checkFunc func(t *testing.T, env execution.Env)
	}{
		{
			name: "StepWithAbsoluteDirectory",
			step: core.Step{
				Name: "test-step",
				Dir:  tempDir,
			},
			setupFunc: func() {},
			checkFunc: func(t *testing.T, env execution.Env) {
				// Resolve symlinks for comparison (macOS /var vs /private/var)
				expectedDir, _ := filepath.EvalSymlinks(tempDir)
				actualDir, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedDir, actualDir)
				// env.Envs["PWD"] should match env.WorkingDir
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "StepWithRelativeDirectory",
			step: core.Step{
				Name: "test-step",
				Dir:  "./subdir",
			},
			setupFunc: func() {
				// Create the subdirectory
				require.NoError(t, os.Chdir(tempDir))
				require.NoError(t, os.Mkdir("subdir", 0755))
			},
			checkFunc: func(t *testing.T, env execution.Env) {
				expectedDir := filepath.Join(tempDir, "subdir")
				// Resolve symlinks for comparison
				expectedResolved, _ := filepath.EvalSymlinks(expectedDir)
				actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedResolved, actualResolved)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "StepWithHomeDirectoryNotation",
			step: core.Step{
				Name: "test-step",
				Dir:  "~/testdir",
			},
			setupFunc: func() {
				// Create a directory in home
				homeDir, _ := os.UserHomeDir()
				testDir := filepath.Join(homeDir, "testdir")
				require.NoError(t, os.MkdirAll(testDir, 0755))
			},
			checkFunc: func(t *testing.T, env execution.Env) {
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
			name: "StepWithNoDirectoryUsesCurrentWorkingDirectory",
			step: core.Step{
				Name: "test-step",
			},
			setupFunc: func() {
				require.NoError(t, os.Chdir(tempDir))
			},
			checkFunc: func(t *testing.T, env execution.Env) {
				// Resolve symlinks for comparison (macOS /var vs /private/var)
				expectedDir, _ := filepath.EvalSymlinks(tempDir)
				actualDir, _ := filepath.EvalSymlinks(env.WorkingDir)
				assert.Equal(t, expectedDir, actualDir)
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "StepWithNonExistentDirectory",
			step: core.Step{
				Name: "test-step",
				Dir:  "/non/existent/directory",
			},
			setupFunc: func() {
				require.NoError(t, os.Chdir(tempDir))
			},
			checkFunc: func(t *testing.T, env execution.Env) {
				// Non-existent directory gets resolved to absolute path
				// On Windows, this will include drive letter
				if runtime.GOOS == "windows" {
					assert.Contains(t, env.WorkingDir, "\\non\\existent\\directory")
				} else {
					assert.Equal(t, "/non/existent/directory", env.WorkingDir)
				}
				assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
			},
		},
		{
			name: "StepWithEnvironmentVariableInPath",
			step: core.Step{
				Name: "test-step",
				Dir: func() string {
					if runtime.GOOS == "windows" {
						return "$USERPROFILE/testdir"
					}
					return "$HOME/testdir"
				}(),
			},
			setupFunc: func() {
				// Create a directory in home
				homeDir, _ := os.UserHomeDir()
				testDir := filepath.Join(homeDir, "testdir")
				require.NoError(t, os.MkdirAll(testDir, 0755))
			},
			checkFunc: func(t *testing.T, env execution.Env) {
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
			// Don't run in parallel since we're changing working directory

			tt.setupFunc()

			ctx := context.Background()
			env := execution.NewEnv(ctx, tt.step)

			// Check that DAG_RUN_STEP_NAME is always set
			assert.Equal(t, tt.step.Name, env.Envs[execution.EnvKeyDAGRunStepName])

			// Run test-specific checks
			tt.checkFunc(t, env)
		})
	}
}

func TestNewEnv_BasicFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	step := core.Step{
		Name:    "test-step",
		Command: "echo hello",
		Args:    []string{"arg1", "arg2"},
	}

	env := execution.NewEnv(ctx, step)

	// Check basic fields
	assert.Equal(t, step, env.Step)
	assert.NotNil(t, env.Variables)
	assert.NotNil(t, env.Envs)
	assert.NotNil(t, env.StepMap)
	assert.Equal(t, "test-step", env.Envs[execution.EnvKeyDAGRunStepName])

	// Check that PWD is set
	assert.NotEmpty(t, env.Envs["PWD"])

	// Check that WorkingDir is set
	assert.NotEmpty(t, env.WorkingDir)
}

func TestEnv_EvalString_Precedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) (context.Context, execution.Env)
		input    string
		expected string
	}{
		{
			name: "StepEnvOverridesOutputVariablesAndDAGEnv",
			setup: func(ctx context.Context) (context.Context, execution.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"FOO=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, core.DAGRunRef{}, "test-run", "test.log", nil, nil)

				// Create executor env
				env := execution.NewEnv(ctx, core.Step{Name: "test"})

				// Set output variable
				env.Variables.Store("FOO", "FOO=from_output")

				// Set step env (highest precedence)
				env.Envs["FOO"] = "from_step"

				return ctx, env
			},
			input:    "${FOO}",
			expected: "from_step",
		},
		{
			name: "OutputVariablesOverrideDAGEnv",
			setup: func(ctx context.Context) (context.Context, execution.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"BAR=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, core.DAGRunRef{}, "test-run", "test.log", nil, nil)

				// Create executor env
				env := execution.NewEnv(ctx, core.Step{Name: "test"})

				// Set output variable (higher precedence than DAG)
				env.Variables.Store("BAR", "BAR=from_output")

				return ctx, env
			},
			input:    "${BAR}",
			expected: "from_output",
		},
		{
			name: "DAGEnvUsedWhenNoOverrideExists",
			setup: func(ctx context.Context) (context.Context, execution.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"BAZ=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, core.DAGRunRef{}, "test-run", "test.log", nil, nil)

				// Create executor env
				env := execution.NewEnv(ctx, core.Step{Name: "test"})

				return ctx, env
			},
			input:    "${BAZ}",
			expected: "from_dag",
		},
		{
			name: "MultipleVariablesWithDifferentPrecedence",
			setup: func(ctx context.Context) (context.Context, execution.Env) {
				// Create DAG with multiple env variables
				dag := &core.DAG{
					Env: []string{"VAR1=dag1", "VAR2=dag2", "VAR3=dag3"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, core.DAGRunRef{}, "test-run", "test.log", nil, nil)

				// Create executor env
				env := execution.NewEnv(ctx, core.Step{Name: "test"})

				// Set output variables
				env.Variables.Store("VAR1", "VAR1=output1")
				env.Variables.Store("VAR2", "VAR2=output2")

				// Set step env (only for VAR1)
				env.Envs["VAR1"] = "step1"

				return ctx, env
			},
			input:    "VAR1=${VAR1}, VAR2=${VAR2}, VAR3=${VAR3}",
			expected: "VAR1=step1, VAR2=output2, VAR3=dag3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctx, env := tt.setup(ctx)

			result, err := env.EvalString(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
