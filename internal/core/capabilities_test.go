package core

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/stretchr/testify/assert"
)

// applyEvalOptions applies all options and returns the resulting EvalOptions
func applyEvalOptions(opts []cmdutil.EvalOption) *cmdutil.EvalOptions {
	evalOpts := cmdutil.NewEvalOptions()
	for _, opt := range opts {
		opt(evalOpts)
	}
	return evalOpts
}

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
		step := Step{ExecutorConfig: ExecutorConfig{Type: "any-type"}}
		opts := step.EvalOptions(ctx)
		assert.Len(t, opts, 1)
		assert.True(t, applyEvalOptions(opts).SkipOSEnvExpansion)
	})

	t.Run("WithGetEvalOptions", func(t *testing.T) {
		RegisterExecutorCapabilities("eval-opts-test-v2", ExecutorCapabilities{
			Command: true,
			GetEvalOptions: func(_ context.Context, _ Step) []cmdutil.EvalOption {
				return []cmdutil.EvalOption{cmdutil.WithoutExpandShell()}
			},
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "eval-opts-test-v2"}}
		opts := step.EvalOptions(ctx)
		assert.Len(t, opts, 2)

		evalOpts := applyEvalOptions(opts)
		assert.True(t, evalOpts.SkipOSEnvExpansion)
		assert.False(t, evalOpts.ExpandShell)
	})

	t.Run("WithoutGetEvalOptions", func(t *testing.T) {
		RegisterExecutorCapabilities("no-eval-opts-test-v2", ExecutorCapabilities{
			Command: true,
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "no-eval-opts-test-v2"}}
		opts := step.EvalOptions(ctx)
		assert.Len(t, opts, 1)
		assert.True(t, applyEvalOptions(opts).SkipOSEnvExpansion)
	})

	t.Run("UnregisteredExecutor", func(t *testing.T) {
		step := Step{ExecutorConfig: ExecutorConfig{Type: "unregistered-executor-v2"}}
		opts := step.EvalOptions(ctx)
		assert.Len(t, opts, 1)
		assert.True(t, applyEvalOptions(opts).SkipOSEnvExpansion)
	})
}
