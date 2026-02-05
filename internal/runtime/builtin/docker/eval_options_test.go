package docker

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestDockerExecutor_GetEvalOptions(t *testing.T) {
	ctx := context.Background()

	t.Run("StepContainerShell", func(t *testing.T) {
		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "docker"},
			Container: &core.Container{
				Image: "alpine",
				Shell: []string{"/bin/sh", "-c"},
			},
		}

		opts := step.EvalOptions(ctx)
		evalOpts := eval.NewOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		require.False(t, evalOpts.EscapeDollar)
	})

	t.Run("ExecutorConfigShell", func(t *testing.T) {
		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{
				Type: "docker",
				Config: map[string]any{
					"image": "alpine",
					"shell": []string{"/bin/sh", "-c"},
				},
			},
		}

		opts := step.EvalOptions(ctx)
		evalOpts := eval.NewOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		require.False(t, evalOpts.EscapeDollar)
	})

	t.Run("DAGContainerShell", func(t *testing.T) {
		dag := &core.DAG{
			Name: "test-dag",
			Container: &core.Container{
				Image: "alpine",
				Shell: []string{"/bin/sh", "-c"},
			},
		}
		dagCtx := runtime.NewContextForTest(ctx, dag, "run-1", "log.txt")

		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "container"},
		}

		opts := step.EvalOptions(dagCtx)
		evalOpts := eval.NewOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		require.False(t, evalOpts.EscapeDollar)
	})

	t.Run("NoShellConfigured", func(t *testing.T) {
		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "docker"},
			Container:      &core.Container{Image: "alpine"},
		}

		opts := step.EvalOptions(ctx)
		evalOpts := eval.NewOptions()
		for _, opt := range opts {
			opt(evalOpts)
		}
		require.True(t, evalOpts.EscapeDollar)
	})
}
