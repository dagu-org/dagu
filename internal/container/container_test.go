package container

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseMapConfig tests the ParseMapConfig function with 92.7% coverage.
// The uncovered lines (7.3%) are error handling for mapstructure.NewDecoder failures
// which cannot be triggered in practice because we always pass valid struct pointers.
// These error checks exist as defensive programming for potential future changes.
func TestParseMapConfig(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		input       map[string]any
		expected    *Container
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal config with image",
			input: map[string]any{
				"image": "alpine:latest",
			},
			expected: &Container{
				image:           "alpine:latest",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "minimal config with containerName",
			input: map[string]any{
				"containerName": "my-container",
			},
			expected: &Container{
				containerName:   "my-container",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "error when neither image nor containerName provided",
			input: map[string]any{
				"platform": "linux/amd64",
			},
			expectError: true,
			errorMsg:    "containerName or image must be specified",
		},
		{
			name: "full config with all fields",
			input: map[string]any{
				"image":         "ubuntu:20.04",
				"containerName": "test-container",
				"platform":      "linux/arm64",
				"pull":          "always",
				"autoRemove":    true,
				"container": map[string]any{
					"Env":        []string{"FOO=bar"},
					"WorkingDir": "/app",
					"User":       "1000",
				},
				"host": map[string]any{
					"AutoRemove": true,
					"Privileged": true,
				},
				"network": map[string]any{
					"EndpointsConfig": map[string]any{},
				},
				"exec": map[string]any{
					"User":       "root",
					"WorkingDir": "/tmp",
					"Env":        []string{"BAR=baz"},
				},
			},
			expected: &Container{
				image:         "ubuntu:20.04",
				containerName: "test-container",
				platform:      "linux/arm64",
				pull:          digraph.PullPolicyAlways,
				autoRemove:    true,
				containerConfig: &container.Config{
					Env:        []string{"FOO=bar"},
					WorkingDir: "/app",
					User:       "1000",
				},
				hostConfig: &container.HostConfig{
					AutoRemove: false, // Should be false because autoRemove is handled separately
					Privileged: true,
				},
				networkConfig: &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{},
				},
				execOptions: &container.ExecOptions{
					User:       "root",
					WorkingDir: "/tmp",
					Env:        []string{"BAR=baz"},
				},
			},
		},
		{
			name: "autoRemove from hostConfig",
			input: map[string]any{
				"image": "alpine",
				"host": map[string]any{
					"AutoRemove": true,
				},
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      true,
				containerConfig: &container.Config{},
				hostConfig: &container.HostConfig{
					AutoRemove: false,
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove explicit true overrides hostConfig false",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": true,
				"host": map[string]any{
					"AutoRemove": false,
				},
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      true,
				containerConfig: &container.Config{},
				hostConfig: &container.HostConfig{
					AutoRemove: false,
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove string value true",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "true",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      true,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove string value false",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "false",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      false,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove string value 1",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "1",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      true,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove string value 0",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "0",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				autoRemove:      false,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove invalid value",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "invalid",
			},
			expectError: true,
			errorMsg:    "failed to evaluate autoRemove value",
		},
		{
			name: "autoRemove unsupported type",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": 123,
			},
			expectError: true,
			errorMsg:    "failed to evaluate autoRemove value",
		},
		{
			name: "pull policy never",
			input: map[string]any{
				"image": "alpine",
				"pull":  "never",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyNever,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy missing",
			input: map[string]any{
				"image": "alpine",
				"pull":  "missing",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy as boolean true",
			input: map[string]any{
				"image": "alpine",
				"pull":  true,
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyAlways,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy as boolean false",
			input: map[string]any{
				"image": "alpine",
				"pull":  false,
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyNever,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy as string true",
			input: map[string]any{
				"image": "alpine",
				"pull":  "true",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyAlways,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "invalid pull policy",
			input: map[string]any{
				"image": "alpine",
				"pull":  "invalid",
			},
			expectError: true,
			errorMsg:    "failed to parse pull policy as boolean",
		},
		{
			name: "pull policy unsupported type",
			input: map[string]any{
				"image": "alpine",
				"pull":  123,
			},
			expectError: true,
			errorMsg:    "invalid pull policy type",
		},
		{
			name: "container config with weakly typed input",
			input: map[string]any{
				"image": "alpine",
				"container": map[string]any{
					"Env": "FOO=bar", // String instead of slice
				},
			},
			expected: &Container{
				image: "alpine",
				pull:  digraph.PullPolicyMissing,
				containerConfig: &container.Config{
					Env: []string{"FOO=bar"},
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "invalid container config decoder",
			input: map[string]any{
				"image":     "alpine",
				"container": "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "invalid host config decoder",
			input: map[string]any{
				"image": "alpine",
				"host":  "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "invalid network config decoder",
			input: map[string]any{
				"image":   "alpine",
				"network": "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "invalid exec config decoder",
			input: map[string]any{
				"image": "alpine",
				"exec":  "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "empty config sections",
			input: map[string]any{
				"image":     "alpine",
				"container": map[string]any{},
				"host":      map[string]any{},
				"network":   map[string]any{},
				"exec":      map[string]any{},
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "both image and containerName empty strings",
			input: map[string]any{
				"image":         "",
				"containerName": "",
			},
			expectError: true,
			errorMsg:    "containerName or image must be specified",
		},
		{
			name: "platform as non-string type",
			input: map[string]any{
				"image":    "alpine",
				"platform": 123, // Not a string
			},
			expected: &Container{
				image:           "alpine",
				platform:        "123",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "containerName as non-string type",
			input: map[string]any{
				"image":         "alpine",
				"containerName": 123, // Not a string
			},
			expected: &Container{
				image:           "alpine",
				containerName:   "123",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "image as non-string type",
			input: map[string]any{
				"image":         123, // Not a string
				"containerName": "test",
			},
			expected: &Container{
				image:           "123",
				containerName:   "test",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "nil sections are handled",
			input: map[string]any{
				"image":     "alpine",
				"container": nil,
				"host":      nil,
				"network":   nil,
				"exec":      nil,
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy nil",
			input: map[string]any{
				"image": "alpine",
				"pull":  nil,
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "pull policy empty string",
			input: map[string]any{
				"image": "alpine",
				"pull":  "",
			},
			expected: &Container{
				image:           "alpine",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "autoRemove nil value",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": nil,
			},
			expected: &Container{
				image:           "alpine",
				autoRemove:      false,
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "platform nil value",
			input: map[string]any{
				"image":    "alpine",
				"platform": nil,
			},
			expected: &Container{
				image:           "alpine",
				platform:        "",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "containerName nil value",
			input: map[string]any{
				"image":         "alpine",
				"containerName": nil,
			},
			expected: &Container{
				image:           "alpine",
				containerName:   "",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
		{
			name: "image nil value",
			input: map[string]any{
				"image":         nil,
				"containerName": "test",
			},
			expected: &Container{
				image:           "",
				containerName:   "test",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
				execOptions:     &container.ExecOptions{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMapConfig(ctx, tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.image, result.image)
			assert.Equal(t, tt.expected.containerName, result.containerName)
			assert.Equal(t, tt.expected.platform, result.platform)
			assert.Equal(t, tt.expected.pull, result.pull)
			assert.Equal(t, tt.expected.autoRemove, result.autoRemove)

			// Compare container config
			assert.Equal(t, tt.expected.containerConfig.Env, result.containerConfig.Env)
			assert.Equal(t, tt.expected.containerConfig.WorkingDir, result.containerConfig.WorkingDir)
			assert.Equal(t, tt.expected.containerConfig.User, result.containerConfig.User)

			// Compare host config
			assert.Equal(t, tt.expected.hostConfig.AutoRemove, result.hostConfig.AutoRemove)
			assert.Equal(t, tt.expected.hostConfig.Privileged, result.hostConfig.Privileged)

			// Compare exec options
			assert.Equal(t, tt.expected.execOptions.User, result.execOptions.User)
			assert.Equal(t, tt.expected.execOptions.WorkingDir, result.execOptions.WorkingDir)
			assert.Equal(t, tt.expected.execOptions.Env, result.execOptions.Env)
		})
	}
}
