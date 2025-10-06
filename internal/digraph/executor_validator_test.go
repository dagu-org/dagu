package digraph

import (
	"strings"
	"testing"
)

// Test SSH executor validator directly
func TestSSHExecutorValidator_ScriptField(t *testing.T) {
	tests := []struct {
		name      string
		step      *Step
		wantError bool
		errorMsg  string
	}{
		{
			name: "SSH with script should fail",
			step: &Step{
				Name:   "ssh-with-script",
				Script: "echo 'test'",
				ExecutorConfig: ExecutorConfig{
					Type: "ssh",
					Config: map[string]any{
						"host": "example.com",
					},
				},
			},
			wantError: true,
			errorMsg:  "script field is not supported with SSH executor",
		},
		{
			name: "SSH with command should pass",
			step: &Step{
				Name:    "ssh-with-command",
				Command: "echo 'test'",
				ExecutorConfig: ExecutorConfig{
					Type: "ssh",
					Config: map[string]any{
						"host": "example.com",
					},
				},
			},
			wantError: false,
		},
		{
			name: "SSH with empty script and command should pass",
			step: &Step{
				Name:    "ssh-with-command-only",
				Script:  "",
				Command: "ls -la",
				ExecutorConfig: ExecutorConfig{
					Type:   "ssh",
					Config: map[string]any{},
				},
			},
			wantError: false,
		},
		{
			name: "Docker with script should pass",
			step: &Step{
				Name:   "docker-with-script",
				Script: "echo 'test'",
				ExecutorConfig: ExecutorConfig{
					Type: "docker",
					Config: map[string]any{
						"image": "alpine:latest",
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the correct validator from the registry based on executor type
			validator, exists := executorValidatorRegistry[tt.step.ExecutorConfig.Type]
			if !exists {
				t.Fatalf("No validator registered for executor type: %s", tt.step.ExecutorConfig.Type)
			}

			err := validator.ValidateStep(tt.step)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test validator registry
func TestExecutorValidatorRegistry(t *testing.T) {
	expectedExecutors := []string{"ssh", "docker", "http", "mail", "jq"}

	for _, execType := range expectedExecutors {
		t.Run(execType, func(t *testing.T) {
			if _, exists := executorValidatorRegistry[execType]; !exists {
				t.Errorf("%s validator should be registered", execType)
			}
		})
	}
}
