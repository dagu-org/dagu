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
		name        string
		step        digraph.Step
		pull        PullPolicy
		expectError bool
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
			pull:        Missing,
			expectError: false,
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
			pull:        Always,
			expectError: false,
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
			pull:        Always,
			expectError: false,
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
			pull:        Never,
			expectError: false,
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
			pull:        Never,
			expectError: false,
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
			pull:        Missing,
			expectError: false,
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
			pull:        Always,
			expectError: false,
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
			pull:        Never,
			expectError: false,
		},
		{
			name: "Error",
			step: digraph.Step{
				Name: "docker-exec",
				ExecutorConfig: digraph.ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "testimage",
						"pull":  "random pull policy should not exist",
					},
				}},
			pull:        Missing,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := newDocker(context.Background(), tt.step)

			// Check error expectation
			if (err != nil) != tt.expectError {
				t.Errorf("error = %v, expectError %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				return // No need to check the result if we expected an error
			}

			dockerExec, ok := exec.(*docker)
			require.True(t, ok)

			assert.Equal(t, tt.pull, dockerExec.pull)
		})
	}
}
