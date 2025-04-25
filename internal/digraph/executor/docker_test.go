package executor

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullPolicy(t *testing.T) {
	tests := []struct {
		name string
		step digraph.Step
		pull PullPolicy
	}{
		{
			name: "Missing",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
					},
				}},
			pull: Missing,
		},
		{
			name: "TrueBool",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  true,
					},
				}},
			pull: Always,
		},
		{
			name: "TrueString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "true",
					},
				}},
			pull: Always,
		},
		{
			name: "FalseBool",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  false,
					},
				}},
			pull: Never,
		},
		{
			name: "FalseString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "false",
					},
				}},
			pull: Never,
		},
		{
			name: "MissingString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "missing",
					},
				}},
			pull: Missing,
		},
		{
			name: "AlwaysString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "always",
					},
				}},
			pull: Always,
		},
		{
			name: "NeverString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "never",
					},
				}},
			pull: Never,
		},
		{
			name: "FallbackString",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "random pull policy should not exist",
					},
				}},
			pull: Missing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := newDocker(context.Background(), tt.step)
			require.NoError(t, err)

			dockerExec, ok := exec.(*docker)
			require.True(t, ok)

			assert.Equal(t, tt.pull, dockerExec.pull)
		})
	}
}
