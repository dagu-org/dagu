package docker

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func getEscapeDollar(ctx context.Context, step core.Step) bool {
	opts := eval.NewOptions()
	for _, opt := range step.EvalOptions(ctx) {
		opt(opts)
	}
	return opts.EscapeDollar
}

func TestDockerExecutor_GetEvalOptions(t *testing.T) {
	ctx := context.Background()

	t.Run("ShellDisablesEscape", func(t *testing.T) {
		tests := []struct {
			name string
			step core.Step
		}{
			{
				name: "StepContainerShell",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{Type: "docker"},
					Container: &core.Container{
						Image: "alpine",
						Shell: []string{"/bin/sh", "-c"},
					},
				},
			},
			{
				name: "ExecutorConfigShellAsSlice",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": []string{"/bin/sh", "-c"}},
					},
				},
			},
			{
				name: "ExecutorConfigShellAsString",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": "/bin/bash"},
					},
				},
			},
			{
				name: "ExecutorConfigShellAsAnySlice",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": []any{"/bin/sh", "-c"}},
					},
				},
			},
			{
				name: "ExecutorConfigShellAsAnySliceWithNonString",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": []any{123}},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				require.False(t, getEscapeDollar(ctx, tt.step))
			})
		}
	})

	t.Run("NoShellKeepsEscapeEnabled", func(t *testing.T) {
		tests := []struct {
			name string
			step core.Step
		}{
			{
				name: "NoShellConfigured",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{Type: "docker"},
					Container:      &core.Container{Image: "alpine"},
				},
			},
			{
				name: "EmptyShellString",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": ""},
					},
				},
			},
			{
				name: "NilShellValue",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": nil},
					},
				},
			},
			{
				name: "EmptyAnySlice",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": []any{}},
					},
				},
			},
			{
				name: "AnySliceWithWhitespace",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Type:   "docker",
						Config: map[string]any{"image": "alpine", "shell": []any{"  ", "\t"}},
					},
				},
			},
			{
				name: "NoContainerNoConfigNoDAG",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{Type: "docker"},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				require.True(t, getEscapeDollar(ctx, tt.step))
			})
		}
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
		step := core.Step{ExecutorConfig: core.ExecutorConfig{Type: "container"}}

		require.False(t, getEscapeDollar(dagCtx, step))
	})

	t.Run("DAGContainerNilShell", func(t *testing.T) {
		dag := &core.DAG{
			Name:      "test-dag",
			Container: &core.Container{Image: "alpine"},
		}
		dagCtx := runtime.NewContextForTest(ctx, dag, "run-1", "log.txt")
		step := core.Step{ExecutorConfig: core.ExecutorConfig{Type: "container"}}

		require.True(t, getEscapeDollar(dagCtx, step))
	})

	t.Run("DAGNilContainer", func(t *testing.T) {
		dag := &core.DAG{Name: "test-dag"}
		dagCtx := runtime.NewContextForTest(ctx, dag, "run-1", "log.txt")
		step := core.Step{ExecutorConfig: core.ExecutorConfig{Type: "docker"}}

		require.True(t, getEscapeDollar(dagCtx, step))
	})
}
