package api

import (
	"testing"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequiredInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		step      core.Step
		body      *api.ApproveStepRequest
		expectErr bool
		errMsg    string
	}{
		{
			name: "no config - always valid",
			step: core.Step{
				Name: "test",
			},
			body:      nil,
			expectErr: false,
		},
		{
			name: "empty config - always valid",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type:   "hitl",
					Config: map[string]any{},
				},
			},
			body:      nil,
			expectErr: false,
		},
		{
			name: "no required fields - always valid",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"input": []any{"reason", "approver"},
					},
				},
			},
			body:      nil,
			expectErr: false,
		},
		{
			name: "required fields provided",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"input":    []any{"reason", "approver"},
						"required": []any{"reason"},
					},
				},
			},
			body: &api.ApproveStepRequest{
				Inputs: &map[string]string{
					"reason": "approved for testing",
				},
			},
			expectErr: false,
		},
		{
			name: "required fields missing - no body",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"required": []any{"reason"},
					},
				},
			},
			body:      nil,
			expectErr: true,
			errMsg:    "missing required inputs: [reason]",
		},
		{
			name: "required fields missing - empty inputs",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"required": []any{"reason", "approver"},
					},
				},
			},
			body: &api.ApproveStepRequest{
				Inputs: &map[string]string{},
			},
			expectErr: true,
			errMsg:    "missing required inputs: [reason approver]",
		},
		{
			name: "partial required fields provided",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"required": []any{"reason", "approver"},
					},
				},
			},
			body: &api.ApproveStepRequest{
				Inputs: &map[string]string{
					"reason": "approved",
				},
			},
			expectErr: true,
			errMsg:    "missing required inputs: [approver]",
		},
		{
			name: "all required fields provided with extras",
			step: core.Step{
				Name: "test",
				ExecutorConfig: core.ExecutorConfig{
					Type: "hitl",
					Config: map[string]any{
						"required": []any{"reason"},
					},
				},
			},
			body: &api.ApproveStepRequest{
				Inputs: &map[string]string{
					"reason":  "approved",
					"comment": "extra field",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRequiredInputs(tt.step, tt.body)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
