package executor

import (
	"bytes"
	"context"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandExecutor_ErrexitSimple(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping errexit tests on Windows")
	}

	setupTest := func(step digraph.Step) context.Context {
		// Create a minimal DAG for testing
		dag := &digraph.DAG{
			Name: "test-dag",
			Env:  []string{},
		}

		// Set up the digraph environment
		ctx := digraph.SetupDAGContext(
			context.Background(),
			dag,
			nil, // database
			digraph.DAGRunRef{},
			"test-run-id",
			"/tmp/test.log",
			[]string{},
			nil,
		)

		// Set up the executor environment
		return digraph.WithEnv(ctx, digraph.NewEnv(ctx, step))
	}

	t.Run("DefaultShellWithErrexitFlag", func(t *testing.T) {
		// Simulate how the scheduler would set up a step
		step := digraph.Step{
			Name:         "test",
			ShellCmdArgs: `false && echo "This should not execute"`,
		}

		ctx := setupTest(step)

		cfg, err := createCommandConfig(ctx, step)
		require.NoError(t, err)

		// Check that -e flag was added to shell command
		assert.Contains(t, cfg.ShellCommand, " -e", "Default shell should have -e flag added")

		executor := &commandExecutor{config: cfg}

		var stdout, stderr bytes.Buffer
		executor.SetStdout(&stdout)
		executor.SetStderr(&stderr)

		err = executor.Run(ctx)
		assert.Error(t, err, "Command should fail due to 'false' command")
		assert.NotContains(t, stdout.String(), "This should not execute",
			"Second command should not run after failure with errexit")
	})

	t.Run("UserSpecifiedShellNoErrexitFlag", func(t *testing.T) {
		step := digraph.Step{
			Name:         "test",
			Shell:        "bash",
			ShellCmdArgs: `false; echo "This should execute"`,
		}

		ctx := setupTest(step)

		cfg, err := createCommandConfig(ctx, step)
		require.NoError(t, err)

		// Check that -e flag was NOT added
		assert.Equal(t, "bash", cfg.ShellCommand, "User-specified shell should not have -e flag added")

		executor := &commandExecutor{config: cfg}

		var stdout, stderr bytes.Buffer
		executor.SetStdout(&stdout)
		executor.SetStderr(&stderr)

		err = executor.Run(ctx)
		assert.NoError(t, err, "Command should succeed overall")
		assert.Contains(t, stdout.String(), "This should execute",
			"Second command should run when user specifies shell without errexit")
	})

	t.Run("UserSpecifiedShellWithExplicitErrexitFlag", func(t *testing.T) {
		step := digraph.Step{
			Name:         "test",
			Shell:        "bash -e",
			ShellCmdArgs: `false && echo "This should not execute"`,
		}

		ctx := setupTest(step)

		cfg, err := createCommandConfig(ctx, step)
		require.NoError(t, err)

		// Check that user's shell command is preserved
		assert.Equal(t, "bash -e", cfg.ShellCommand, "User-specified shell with -e should be preserved")

		executor := &commandExecutor{config: cfg}

		var stdout, stderr bytes.Buffer
		executor.SetStdout(&stdout)
		executor.SetStderr(&stderr)

		err = executor.Run(ctx)
		assert.Error(t, err, "Command should fail due to 'false' command with -e flag")
		assert.NotContains(t, stdout.String(), "This should not execute",
			"Second command should not run when user explicitly adds -e flag")
	})
}
