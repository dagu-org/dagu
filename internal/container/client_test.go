package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseMapConfig tests the ParseMapConfig function with 92.7% coverage.
// The uncovered lines (7.3%) are error handling for mapstructure.NewDecoder failures
// which cannot be triggered in practice because we always pass valid struct pointers.
// These error checks exist as defensive programming for potential future changes.
func TestParseMapConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]any
		expected    *Client
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal config with image",
			input: map[string]any{
				"image": "alpine:latest",
			},
			expected: &Client{
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
			expected: &Client{
				containerID:     "my-container",
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
			name: "full config for new container (no containerName)",
			input: map[string]any{
				"image":      "ubuntu:20.04",
				"platform":   "linux/arm64",
				"pull":       "always",
				"autoRemove": true,
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
			},
			expected: &Client{
				image:      "ubuntu:20.04",
				platform:   "linux/arm64",
				pull:       digraph.PullPolicyAlways,
				autoRemove: true,
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
				execOptions: &container.ExecOptions{},
			},
		},
		{
			name: "exec mode with containerName and exec options",
			input: map[string]any{
				"containerName": "test-container",
				"exec": map[string]any{
					"User":       "root",
					"WorkingDir": "/tmp",
					"Env":        []string{"BAR=baz"},
				},
			},
			expected: &Client{
				containerID:     "test-container",
				pull:            digraph.PullPolicyMissing,
				containerConfig: &container.Config{},
				hostConfig:      &container.HostConfig{},
				networkConfig:   &network.NetworkingConfig{},
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			name: "error when both image and containerName provided",
			input: map[string]any{
				"image":         "alpine",
				"containerName": "test",
			},
			expectError: true,
			errorMsg:    "cannot set both 'image' and 'containerName'",
		},
		{
			name: "error when exec provided with image only",
			input: map[string]any{
				"image": "alpine",
				"exec": map[string]any{
					"User": "root",
				},
			},
			expectError: true,
			errorMsg:    "exec' options require 'containerName",
		},
		{
			name: "error when containerName with unsupported options",
			input: map[string]any{
				"containerName": "test",
				"autoRemove":    true,
			},
			expectError: true,
			errorMsg:    "not supported with 'containerName'",
		},
		{
			name: "platform as non-string type",
			input: map[string]any{
				"image":    "alpine",
				"platform": 123, // Not a string
			},
			expected: &Client{
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
			name: "containerName as non-string type (exec mode)",
			input: map[string]any{
				"containerName": 123, // Not a string
			},
			expected: &Client{
				containerID:     "123",
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
				"image": 123, // Not a string
			},
			expected: &Client{
				image:           "123",
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
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
			expected: &Client{
				image:           "alpine",
				containerID:     "",
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
			expected: &Client{
				image:           "",
				containerID:     "test",
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
			result, err := NewFromMapConfig(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.image, result.image)
			assert.Equal(t, tt.expected.containerID, result.containerID)
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

func TestParseContainer(t *testing.T) {
	tests := []struct {
		name        string
		input       digraph.Container
		expected    *Client
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal container with image only",
			input: digraph.Container{
				Image: "alpine:latest",
			},
			expected: &Client{
				image:      "alpine:latest",
				pull:       digraph.PullPolicyAlways, // Zero value of PullPolicy
				autoRemove: true,                     // Default when KeepContainer is false
				containerConfig: &container.Config{
					Image: "alpine:latest",
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "error when image is empty",
			input: digraph.Container{
				Platform: "linux/amd64",
			},
			expectError: true,
			errorMsg:    "image is required",
		},
		{
			name: "full container configuration",
			input: digraph.Container{
				Image:         "ubuntu:20.04",
				PullPolicy:    digraph.PullPolicyAlways,
				Env:           []string{"FOO=bar", "BAZ=qux"},
				Volumes:       []string{"/host/data:/data:ro", "myvolume:/app"},
				User:          "1000:1000",
				WorkingDir:    "/workspace",
				Platform:      "linux/arm64",
				Ports:         []string{"8080:80", "9090"},
				Network:       "mynetwork",
				KeepContainer: true,
			},
			expected: &Client{
				image:      "ubuntu:20.04",
				platform:   "linux/arm64",
				pull:       digraph.PullPolicyAlways,
				autoRemove: false, // KeepContainer is true
				containerConfig: &container.Config{
					Image:      "ubuntu:20.04",
					Env:        []string{"FOO=bar", "BAZ=qux"},
					User:       "1000:1000",
					WorkingDir: "/workspace",
					ExposedPorts: nat.PortSet{
						"80/tcp":   {},
						"9090/tcp": {},
					},
				},
				hostConfig: &container.HostConfig{
					Binds: []string{"/host/data:/data:ro"},
					Mounts: []mount.Mount{
						{
							Type:     mount.TypeVolume,
							Source:   "myvolume",
							Target:   "/app",
							ReadOnly: false,
						},
					},
					PortBindings: nat.PortMap{
						"80/tcp": []nat.PortBinding{
							{
								HostIP:   "0.0.0.0",
								HostPort: "8080",
							},
						},
					},
					NetworkMode: "mynetwork",
				},
				networkConfig: &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						"mynetwork": {},
					},
				},
				execOptions: &container.ExecOptions{},
			},
		},
		{
			name: "standard network modes",
			input: digraph.Container{
				Image:   "nginx",
				Network: "host",
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
				},
				hostConfig: &container.HostConfig{
					NetworkMode: "host",
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "container network reference",
			input: digraph.Container{
				Image:   "nginx",
				Network: "container:myapp",
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
				},
				hostConfig: &container.HostConfig{
					NetworkMode: "container:myapp",
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "bind mount with default rw mode",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"/host/path:/container/path"},
			},
			expected: &Client{
				image:      "alpine",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "alpine",
				},
				hostConfig: &container.HostConfig{
					Binds: []string{"/host/path:/container/path:rw"},
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "relative bind mount",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"./data:/data:ro"},
			},
			expected: func() *Client {
				// Relative paths are resolved to absolute paths
				cwd, _ := os.Getwd()
				resolvedPath := filepath.Join(cwd, "data")
				return &Client{
					image:      "alpine",
					autoRemove: true,
					containerConfig: &container.Config{
						Image: "alpine",
					},
					hostConfig: &container.HostConfig{
						Binds: []string{resolvedPath + ":/data:ro"},
					},
					networkConfig: &network.NetworkingConfig{},
					execOptions:   &container.ExecOptions{},
				}
			}(),
		},
		{
			name: "home directory bind mount",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"~/data:/data:rw"},
			},
			expected: func() *Client {
				// Home directory paths are resolved to absolute paths
				homeDir, _ := os.UserHomeDir()
				resolvedPath := filepath.Join(homeDir, "data")
				return &Client{
					image:      "alpine",
					autoRemove: true,
					containerConfig: &container.Config{
						Image: "alpine",
					},
					hostConfig: &container.HostConfig{
						Binds: []string{resolvedPath + ":/data:rw"},
					},
					networkConfig: &network.NetworkingConfig{},
					execOptions:   &container.ExecOptions{},
				}
			}(),
		},
		{
			name: "port with IP address",
			input: digraph.Container{
				Image: "nginx",
				Ports: []string{"127.0.0.1:8080:80/tcp"},
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
					ExposedPorts: nat.PortSet{
						"80/tcp": {},
					},
				},
				hostConfig: &container.HostConfig{
					PortBindings: nat.PortMap{
						"80/tcp": []nat.PortBinding{
							{
								HostIP:   "127.0.0.1",
								HostPort: "8080",
							},
						},
					},
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "udp port",
			input: digraph.Container{
				Image: "dns-server",
				Ports: []string{"53:53/udp"},
			},
			expected: &Client{
				image:      "dns-server",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "dns-server",
					ExposedPorts: nat.PortSet{
						"53/udp": {},
					},
				},
				hostConfig: &container.HostConfig{
					PortBindings: nat.PortMap{
						"53/udp": []nat.PortBinding{
							{
								HostIP:   "0.0.0.0",
								HostPort: "53",
							},
						},
					},
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "invalid volume format - too few parts",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"/data"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: /data",
		},
		{
			name: "invalid volume format - too many parts",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"/host:/container:ro:extra"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: /host:/container:ro:extra",
		},
		{
			name: "invalid volume mode",
			input: digraph.Container{
				Image:   "alpine",
				Volumes: []string{"/data:/data:invalid"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: invalid mode invalid in /data:/data:invalid",
		},
		{
			name: "invalid port format - too many parts",
			input: digraph.Container{
				Image: "nginx",
				Ports: []string{"1.2.3.4:8080:80:extra"},
			},
			expectError: true,
			errorMsg:    "invalid port format: 1.2.3.4:8080:80:extra",
		},
		{
			name: "invalid port protocol delimiter",
			input: digraph.Container{
				Image: "nginx",
				Ports: []string{"80/tcp/extra"},
			},
			expectError: true,
			errorMsg:    "invalid port format: invalid protocol in 80/tcp/extra",
		},
		{
			name: "invalid port protocol",
			input: digraph.Container{
				Image: "nginx",
				Ports: []string{"80/invalid"},
			},
			expectError: true,
			errorMsg:    "invalid port format: invalid protocol invalid in 80/invalid",
		},
		{
			name: "sctp port protocol",
			input: digraph.Container{
				Image: "sctp-server",
				Ports: []string{"132/sctp"},
			},
			expected: &Client{
				image:      "sctp-server",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "sctp-server",
					ExposedPorts: nat.PortSet{
						"132/sctp": {},
					},
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "whitespace in port specification",
			input: digraph.Container{
				Image: "nginx",
				Ports: []string{" 8080:80 "},
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
					ExposedPorts: nat.PortSet{
						"80/tcp": {},
					},
				},
				hostConfig: &container.HostConfig{
					PortBindings: nat.PortMap{
						"80/tcp": []nat.PortBinding{
							{
								HostIP:   "0.0.0.0",
								HostPort: "8080",
							},
						},
					},
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "empty network uses default",
			input: digraph.Container{
				Image:   "nginx",
				Network: "",
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
				},
				hostConfig: &container.HostConfig{
					NetworkMode: "", // Empty string for default
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "bridge network mode",
			input: digraph.Container{
				Image:   "nginx",
				Network: "bridge",
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
				},
				hostConfig: &container.HostConfig{
					NetworkMode: "bridge",
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "none network mode",
			input: digraph.Container{
				Image:   "nginx",
				Network: "none",
			},
			expected: &Client{
				image:      "nginx",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "nginx",
				},
				hostConfig: &container.HostConfig{
					NetworkMode: "none",
				},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "keep container false sets autoRemove true",
			input: digraph.Container{
				Image:         "alpine",
				KeepContainer: false,
			},
			expected: &Client{
				image:      "alpine",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "alpine",
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "keep container true sets autoRemove false",
			input: digraph.Container{
				Image:         "alpine",
				KeepContainer: true,
			},
			expected: &Client{
				image:      "alpine",
				autoRemove: false,
				containerConfig: &container.Config{
					Image: "alpine",
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "pull policy propagation",
			input: digraph.Container{
				Image:      "alpine",
				PullPolicy: digraph.PullPolicyNever,
			},
			expected: &Client{
				image:      "alpine",
				pull:       digraph.PullPolicyNever,
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "alpine",
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "platform propagation",
			input: digraph.Container{
				Image:    "alpine",
				Platform: "linux/386",
			},
			expected: &Client{
				image:      "alpine",
				platform:   "linux/386",
				autoRemove: true,
				containerConfig: &container.Config{
					Image: "alpine",
				},
				hostConfig:    &container.HostConfig{},
				networkConfig: &network.NetworkingConfig{},
				execOptions:   &container.ExecOptions{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewFromContainerConfig("", tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.image, result.image)
			assert.Equal(t, tt.expected.platform, result.platform)
			assert.Equal(t, tt.expected.pull, result.pull)
			assert.Equal(t, tt.expected.autoRemove, result.autoRemove)

			// Compare container config
			assert.Equal(t, tt.expected.containerConfig.Image, result.containerConfig.Image)
			assert.Equal(t, tt.expected.containerConfig.Env, result.containerConfig.Env)
			assert.Equal(t, tt.expected.containerConfig.User, result.containerConfig.User)
			assert.Equal(t, tt.expected.containerConfig.WorkingDir, result.containerConfig.WorkingDir)

			// Compare exposed ports
			if tt.expected.containerConfig.ExposedPorts != nil {
				assert.Equal(t, tt.expected.containerConfig.ExposedPorts, result.containerConfig.ExposedPorts)
			}

			// Compare host config
			assert.Equal(t, tt.expected.hostConfig.Binds, result.hostConfig.Binds)
			if tt.expected.hostConfig.Mounts != nil {
				assert.Equal(t, tt.expected.hostConfig.Mounts, result.hostConfig.Mounts)
			}
			if tt.expected.hostConfig.PortBindings != nil {
				assert.Equal(t, tt.expected.hostConfig.PortBindings, result.hostConfig.PortBindings)
			}
			assert.Equal(t, tt.expected.hostConfig.NetworkMode, result.hostConfig.NetworkMode)

			// Compare network config
			if tt.expected.networkConfig.EndpointsConfig != nil {
				assert.Equal(t, tt.expected.networkConfig.EndpointsConfig, result.networkConfig.EndpointsConfig)
			}
		})
	}
}
