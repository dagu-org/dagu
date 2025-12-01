package runtime_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv_VariablesMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupEnv func(env runtime.Env) runtime.Env
		expected map[string]string
	}{
		{
			name: "CombinesVariablesAndEnvs",
			setupEnv: func(env runtime.Env) runtime.Env {
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
			setupEnv: func(env runtime.Env) runtime.Env {
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
			setupEnv: func(env runtime.Env) runtime.Env {
				return env
			},
			expected: map[string]string{
				execution.EnvKeyDAGRunStepName: "test-step",
			},
		},
		{
			name: "OnlyVariables",
			setupEnv: func(env runtime.Env) runtime.Env {
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
			setupEnv: func(env runtime.Env) runtime.Env {
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

			// Create a temporary directory to use as DAG working directory
			tempDir := t.TempDir()

			// Set up DAG context with WorkingDir
			dag := &core.DAG{
				Name:       "test-dag",
				WorkingDir: tempDir,
			}
			dagCtx := execution.DAGContext{
				DAG: dag,
			}
			ctx := execution.WithDAGContext(context.Background(), dagCtx)

			env := runtime.NewEnvForStep(ctx, core.Step{Name: "test-step"})
			env = tt.setupEnv(env)

			result := env.VariablesMap()

			// Check that all expected keys exist with correct values
			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}
		})
	}
}

func TestNewEnvForStep_WorkingDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create subdirectory for relative path tests
	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	// Create testdir in home for tilde tests
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	homeTempDir := filepath.Join(homeDir, "dagu_test_workdir")
	require.NoError(t, os.MkdirAll(homeTempDir, 0755))
	t.Cleanup(func() { os.RemoveAll(homeTempDir) })

	tests := []struct {
		name        string
		step        core.Step
		dagWorkDir  string // DAG's WorkingDir for context
		expectedDir string
	}{
		{
			name: "StepWithAbsoluteDirectory",
			step: core.Step{
				Name: "test-step",
				Dir:  tempDir,
			},
			dagWorkDir:  "/some/dag/workdir",
			expectedDir: tempDir,
		},
		{
			name: "StepWithRelativeDirectory_ResolvesAgainstDAGWorkDir",
			step: core.Step{
				Name: "test-step",
				Dir:  "./subdir",
			},
			dagWorkDir:  tempDir,
			expectedDir: subDir,
		},
		{
			name: "StepWithRelativeDirectory_NoLeadingDot",
			step: core.Step{
				Name: "test-step",
				Dir:  "subdir",
			},
			dagWorkDir:  tempDir,
			expectedDir: subDir,
		},
		{
			name: "StepWithHomeDirectoryNotation",
			step: core.Step{
				Name: "test-step",
				Dir:  "~/dagu_test_workdir",
			},
			dagWorkDir:  tempDir,
			expectedDir: homeTempDir,
		},
		{
			name: "StepWithNonExistentAbsoluteDirectory",
			step: core.Step{
				Name: "test-step",
				Dir: func() string {
					if goruntime.GOOS == "windows" {
						return "C:\\non\\existent\\directory"
					}
					return "/non/existent/directory"
				}(),
			},
			dagWorkDir: tempDir,
			expectedDir: func() string {
				if goruntime.GOOS == "windows" {
					return "C:\\non\\existent\\directory"
				}
				return "/non/existent/directory"
			}(),
		},
		{
			name: "StepWithEnvironmentVariableInPath_Absolute",
			step: core.Step{
				Name: "test-step",
				Dir: func() string {
					if goruntime.GOOS == "windows" {
						return "$USERPROFILE\\dagu_test_workdir"
					}
					return "$HOME/dagu_test_workdir"
				}(),
			},
			dagWorkDir:  tempDir,
			expectedDir: homeTempDir,
		},
		{
			name: "StepWithNoDir_InheritsDAGWorkDir",
			step: core.Step{
				Name: "test-step",
				Dir:  "",
			},
			dagWorkDir:  tempDir,
			expectedDir: tempDir,
		},
		{
			name: "StepWithParentRelativeDirectory",
			step: core.Step{
				Name: "test-step",
				Dir:  "../",
			},
			dagWorkDir:  subDir,
			expectedDir: tempDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Set up DAG context with WorkingDir
			dag := &core.DAG{
				Name:       "test-dag",
				WorkingDir: tt.dagWorkDir,
			}
			dagCtx := execution.DAGContext{
				DAG: dag,
			}
			ctx := execution.WithDAGContext(context.Background(), dagCtx)

			env := runtime.NewEnvForStep(ctx, tt.step)

			// Check that DAG_RUN_STEP_NAME is always set
			assert.Equal(t, tt.step.Name, env.Envs[execution.EnvKeyDAGRunStepName])

			// Resolve symlinks for comparison (macOS /var vs /private/var)
			expectedResolved, _ := filepath.EvalSymlinks(tt.expectedDir)
			actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
			assert.Equal(t, expectedResolved, actualResolved)

			// env.Envs["PWD"] should match env.WorkingDir
			assert.Equal(t, env.WorkingDir, env.Envs["PWD"])
		})
	}
}

func TestNewEnvForStep_BasicFields(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Set up DAG context with WorkingDir
	dag := &core.DAG{
		Name:       "test-dag",
		WorkingDir: tempDir,
	}
	dagCtx := execution.DAGContext{
		DAG: dag,
	}
	ctx := execution.WithDAGContext(context.Background(), dagCtx)

	step := core.Step{
		Name:    "test-step",
		Command: "echo hello",
		Args:    []string{"arg1", "arg2"},
	}

	env := runtime.NewEnvForStep(ctx, step)

	// Check basic fields
	assert.Equal(t, step, env.Step)
	assert.NotNil(t, env.Variables)
	assert.NotNil(t, env.Envs)
	assert.NotNil(t, env.StepMap)
	assert.Equal(t, "test-step", env.Envs[execution.EnvKeyDAGRunStepName])

	// Check that PWD is set to DAG's WorkingDir
	assert.Equal(t, tempDir, env.Envs["PWD"])

	// Check that WorkingDir is set to DAG's WorkingDir
	assert.Equal(t, tempDir, env.WorkingDir)
}

func TestNewEnvForStep_WorkingDirectory_DAGEnvExpansion(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	// Set up DAG context with WorkingDir and env vars
	dag := &core.DAG{
		Name:       "test-dag",
		WorkingDir: tempDir,
		Env:        []string{"MY_SUBDIR=subdir"},
	}
	dagCtx := execution.DAGContext{
		DAG: dag,
	}
	ctx := execution.WithDAGContext(context.Background(), dagCtx)

	step := core.Step{
		Name: "test-step",
		Dir:  "./$MY_SUBDIR", // Uses DAG env var in relative path
	}

	env := runtime.NewEnvForStep(ctx, step)

	// Resolve symlinks for comparison
	expectedResolved, _ := filepath.EvalSymlinks(subDir)
	actualResolved, _ := filepath.EvalSymlinks(env.WorkingDir)
	assert.Equal(t, expectedResolved, actualResolved)
}

func TestEnv_UserEnvsMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) (context.Context, runtime.Env)
		expected map[string]string
	}{
		{
			name: "IncludesVariablesFromPreviousSteps",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				dag := &core.DAG{Env: []string{"DAG_VAR=dag_value"}}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})
				env.Variables.Store("OUTPUT_VAR", "OUTPUT_VAR=output_value")
				return ctx, env
			},
			expected: map[string]string{
				"DAG_VAR":    "dag_value",
				"OUTPUT_VAR": "output_value",
			},
		},
		{
			name: "StepEnvOverridesAll",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				dag := &core.DAG{Env: []string{"KEY=dag"}}
				secrets := []string{"KEY=secret"}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, secrets)

				step := core.Step{Name: "test", Env: []string{"KEY=step"}}
				env := runtime.NewEnvForStep(ctx, step)
				env.Variables.Store("KEY", "KEY=variable")

				envCtx := runtime.WithEnv(ctx, env)
				parts := strings.SplitN(step.Env[0], "=", 2)
				evaluated, err := env.EvalString(envCtx, parts[1])
				if err != nil {
					panic(fmt.Sprintf("failed to evaluate step env: %v", err))
				}
				vars := &collections.SyncMap{}
				vars.Store(parts[0], fmt.Sprintf("%s=%s", parts[0], evaluated))
				env.ForceLoadOutputVariables(vars)

				return envCtx, env
			},
			expected: map[string]string{
				"KEY": "step",
			},
		},
		{
			name: "StepEnvKeepsEvaluatedSecrets",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				dag := &core.DAG{}
				secrets := []string{"MY_SECRET=super-secret"}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, secrets)

				step := core.Step{Name: "test", Env: []string{"GITHUB_TOKEN=${MY_SECRET}"}}
				env := runtime.NewEnvForStep(ctx, step)

				envCtx := runtime.WithEnv(ctx, env)
				parts := strings.SplitN(step.Env[0], "=", 2)
				evaluated, err := env.EvalString(envCtx, parts[1])
				if err != nil {
					panic(fmt.Sprintf("failed to evaluate step env: %v", err))
				}

				vars := &collections.SyncMap{}
				vars.Store(parts[0], fmt.Sprintf("%s=%s", parts[0], evaluated))
				env.ForceLoadOutputVariables(vars)

				return envCtx, env
			},
			expected: map[string]string{
				"GITHUB_TOKEN": "super-secret",
			},
		},
		{
			name: "ExcludesOSEnvironment",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				dag := &core.DAG{Env: []string{"USER_VAR=user"}}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})
				return ctx, env
			},
			expected: map[string]string{
				"USER_VAR": "user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			_, env := tt.setup(ctx)

			result := env.UserEnvsMap()

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}
			// Ensure OS env is not included (PATH should not be in result)
			_, hasPath := result["PATH"]
			assert.False(t, hasPath, "UserEnvsMap should not include OS environment variables like PATH")
		})
	}
}

func TestEnv_EvalString_Precedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) (context.Context, runtime.Env)
		input    string
		expected string
	}{
		{
			name: "StepEnvOverridesOutputVariablesAndDAGEnv",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"FOO=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)

				// Create executor env
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})

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
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"BAR=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)

				// Create executor env
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})

				// Set output variable (higher precedence than DAG)
				env.Variables.Store("BAR", "BAR=from_output")

				return ctx, env
			},
			input:    "${BAR}",
			expected: "from_output",
		},
		{
			name: "DAGEnvUsedWhenNoOverrideExists",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				// Create DAG with env variable
				dag := &core.DAG{
					Env: []string{"BAZ=from_dag"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)

				// Create executor env
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})

				return ctx, env
			},
			input:    "${BAZ}",
			expected: "from_dag",
		},
		{
			name: "MultipleVariablesWithDifferentPrecedence",
			setup: func(ctx context.Context) (context.Context, runtime.Env) {
				// Create DAG with multiple env variables
				dag := &core.DAG{
					Env: []string{"VAR1=dag1", "VAR2=dag2", "VAR3=dag3"},
				}
				ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil, nil)

				// Create executor env
				env := runtime.NewEnvForStep(ctx, core.Step{Name: "test"})

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
