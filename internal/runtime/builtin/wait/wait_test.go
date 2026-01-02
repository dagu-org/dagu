package wait

import (
	"bytes"
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitExecutor_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         map[string]any
		expectPrompt   string
		expectInputs   bool
		expectRequired bool
	}{
		{
			name:   "basic wait step",
			config: nil,
		},
		{
			name: "wait with prompt",
			config: map[string]any{
				"prompt": "Please approve this deployment",
			},
			expectPrompt: "Please approve this deployment",
		},
		{
			name: "wait with inputs",
			config: map[string]any{
				"prompt":   "Enter approval details",
				"input":    []string{"reason", "approver"},
				"required": []string{"reason"},
			},
			expectPrompt:   "Enter approval details",
			expectInputs:   true,
			expectRequired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := core.Step{
				Name: "test-wait",
				ExecutorConfig: core.ExecutorConfig{
					Type:   "wait",
					Config: tt.config,
				},
			}

			exec, err := newWait(context.Background(), step)
			require.NoError(t, err)

			stdout := &bytes.Buffer{}
			exec.SetStdout(stdout)

			err = exec.Run(context.Background())
			require.NoError(t, err)

			output := stdout.String()
			assert.Contains(t, output, "Waiting for human approval")

			if tt.expectPrompt != "" {
				assert.Contains(t, output, tt.expectPrompt)
			}
			if tt.expectInputs {
				assert.Contains(t, output, "Expected inputs")
			}
			if tt.expectRequired {
				assert.Contains(t, output, "Required inputs")
			}
		})
	}
}

func TestWaitExecutor_DetermineNodeStatus(t *testing.T) {
	t.Parallel()

	step := core.Step{
		Name: "test-wait",
		ExecutorConfig: core.ExecutorConfig{
			Type: "wait",
		},
	}

	exec, err := newWait(context.Background(), step)
	require.NoError(t, err)

	determiner, ok := exec.(interface {
		DetermineNodeStatus() (core.NodeStatus, error)
	})
	require.True(t, ok, "executor should implement NodeStatusDeterminer")

	status, err := determiner.DetermineNodeStatus()
	require.NoError(t, err)
	assert.Equal(t, core.NodeWaiting, status)
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    map[string]any
		expectErr bool
	}{
		{
			name:   "nil config is valid",
			config: nil,
		},
		{
			name:   "empty config is valid",
			config: map[string]any{},
		},
		{
			name: "required fields in input list is valid",
			config: map[string]any{
				"input":    []string{"reason", "approver"},
				"required": []string{"reason"},
			},
		},
		{
			name: "required field not in input list is invalid",
			config: map[string]any{
				"input":    []string{"reason"},
				"required": []string{"approver"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := core.Step{
				Name: "test-wait",
				ExecutorConfig: core.ExecutorConfig{
					Type:   "wait",
					Config: tt.config,
				},
			}

			err := validateConfig(step)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
