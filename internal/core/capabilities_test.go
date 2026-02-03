package core

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/stretchr/testify/assert"
)

func TestExecutorCapabilities_Get(t *testing.T) {
	registry := &executorCapabilitiesRegistry{
		caps: make(map[string]ExecutorCapabilities),
	}

	// Test case 1: Registered executor
	caps := ExecutorCapabilities{Command: true, MultipleCommands: true}
	registry.Register("test-executor", caps)
	assert.Equal(t, caps, registry.Get("test-executor"))

	// Test case 2: Unregistered executor should return empty capabilities (strict default)
	assert.Equal(t, ExecutorCapabilities{}, registry.Get("unregistered"))
}

func TestSupportsHelpers(t *testing.T) {
	// Register a test executor with specific capabilities
	caps := ExecutorCapabilities{
		Command:        true,
		Script:         false,
		WorkerSelector: true,
	}
	RegisterExecutorCapabilities("helper-test", caps)

	assert.True(t, SupportsCommand("helper-test"))
	assert.False(t, SupportsScript("helper-test"))
	assert.True(t, SupportsWorkerSelector("helper-test"))

	// Unregistered executor should return false for everything
	assert.False(t, SupportsCommand("unknown"))
	assert.False(t, SupportsScript("unknown"))
	assert.False(t, SupportsShell("unknown"))
}

func TestStep_EvalOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("AlwaysIncludesSkipOSEnvExpansion", func(t *testing.T) {
		// Any step should always have WithoutOSEnvExpansion as the first option
		step := Step{ExecutorConfig: ExecutorConfig{Type: "any-type"}}
		opts := step.EvalOptions(ctx)
		assert.NotNil(t, opts)
		assert.Len(t, opts, 1, "Should have at least the default SkipOSEnvExpansion option")

		// Verify it sets SkipOSEnvExpansion flag
		evalOpts := cmdutil.NewEvalOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		assert.True(t, evalOpts.SkipOSEnvExpansion, "SkipOSEnvExpansion should be enabled")
	})

	t.Run("WithGetEvalOptions", func(t *testing.T) {
		// Register executor with GetEvalOptions callback
		RegisterExecutorCapabilities("eval-opts-test-v2", ExecutorCapabilities{
			Command: true,
			GetEvalOptions: func(_ context.Context, _ Step) []cmdutil.EvalOption {
				return []cmdutil.EvalOption{cmdutil.WithoutExpandShell()}
			},
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "eval-opts-test-v2"}}
		opts := step.EvalOptions(ctx)
		// Should have 2 options: WithoutOSEnvExpansion() + WithoutExpandShell()
		assert.Len(t, opts, 2)

		// Verify both flags are set
		evalOpts := cmdutil.NewEvalOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		assert.True(t, evalOpts.SkipOSEnvExpansion, "SkipOSEnvExpansion should be enabled")
		assert.False(t, evalOpts.ExpandShell, "ExpandShell should be disabled")
	})

	t.Run("WithoutGetEvalOptions", func(t *testing.T) {
		// Register executor without GetEvalOptions
		RegisterExecutorCapabilities("no-eval-opts-test-v2", ExecutorCapabilities{
			Command: true,
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "no-eval-opts-test-v2"}}
		opts := step.EvalOptions(ctx)
		// Should still have 1 option: WithoutOSEnvExpansion()
		assert.Len(t, opts, 1)

		evalOpts := cmdutil.NewEvalOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		assert.True(t, evalOpts.SkipOSEnvExpansion, "SkipOSEnvExpansion should be enabled")
	})

	t.Run("UnregisteredExecutor", func(t *testing.T) {
		step := Step{ExecutorConfig: ExecutorConfig{Type: "unregistered-executor-v2"}}
		opts := step.EvalOptions(ctx)
		// Should still have 1 option: WithoutOSEnvExpansion()
		assert.Len(t, opts, 1)

		evalOpts := cmdutil.NewEvalOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		assert.True(t, evalOpts.SkipOSEnvExpansion, "SkipOSEnvExpansion should be enabled")
	})
}
