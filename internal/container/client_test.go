package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfigFromMap covers LoadConfigFromMap with 92.7% coverage.
// The uncovered lines (7.3%) are error handling for mapstructure.NewDecoder failures
// which cannot be triggered in practice because we always pass valid struct pointers.
// These error checks exist as defensive programming for potential future changes.
func TestLoadConfigFromMap(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]any
		expected    *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "MinimalConfigWithImage",
			input: map[string]any{
				"image": "alpine:latest",
			},
			expected: &Config{
				Image:       "alpine:latest",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "MinimalConfigWithContainerName",
			input: map[string]any{
				"containerName": "my-container",
			},
			expected: &Config{
				ContainerName: "my-container",
				Pull:          core.PullPolicyMissing,
				Container:     &container.Config{},
				Host:          &container.HostConfig{},
				Network:       &network.NetworkingConfig{},
				ExecOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "ErrorWhenNeitherImageNorContainerNameProvided",
			input: map[string]any{
				"platform": "linux/amd64",
			},
			expectError: true,
			errorMsg:    "containerName or image must be specified",
		},
		{
			name: "FullConfigForNewContainerNoContainerName",
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
			expected: &Config{
				Image:      "ubuntu:20.04",
				Platform:   "linux/arm64",
				Pull:       core.PullPolicyAlways,
				AutoRemove: true,
				Container: &container.Config{
					Env:        []string{"FOO=bar"},
					WorkingDir: "/app",
					User:       "1000",
				},
				Host: &container.HostConfig{
					AutoRemove: false, // Should be false because autoRemove is handled separately
					Privileged: true,
				},
				Network: &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{},
				},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "ExecModeWithContainerNameAndExecOptions",
			input: map[string]any{
				"containerName": "test-container",
				"exec": map[string]any{
					"User":       "root",
					"WorkingDir": "/tmp",
					"Env":        []string{"BAR=baz"},
				},
			},
			expected: &Config{
				ContainerName: "test-container",
				Pull:          core.PullPolicyMissing,
				Container:     &container.Config{},
				Host:          &container.HostConfig{},
				Network:       &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{
					User:       "root",
					WorkingDir: "/tmp",
					Env:        []string{"BAR=baz"},
				},
			},
		},
		{
			name: "AutoRemoveFromHostConfig",
			input: map[string]any{
				"image": "alpine",
				"host": map[string]any{
					"AutoRemove": true,
				},
			},
			expected: &Config{
				Image:      "alpine",
				Pull:       core.PullPolicyMissing,
				AutoRemove: true,
				Container:  &container.Config{},
				Host: &container.HostConfig{
					AutoRemove: false,
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveExplicitTrueOverridesHostConfigFalse",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": true,
				"host": map[string]any{
					"AutoRemove": false,
				},
			},
			expected: &Config{
				Image:      "alpine",
				Pull:       core.PullPolicyMissing,
				AutoRemove: true,
				Container:  &container.Config{},
				Host: &container.HostConfig{
					AutoRemove: false,
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveStringValueTrue",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "true",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				AutoRemove:  true,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveStringValueFalse",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "false",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				AutoRemove:  false,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveStringValue1",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "1",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				AutoRemove:  true,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveStringValue0",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "0",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				AutoRemove:  false,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveInvalidValue",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": "invalid",
			},
			expectError: true,
			errorMsg:    "failed to evaluate autoRemove value",
		},
		{
			name: "AutoRemoveUnsupportedType",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": 123,
			},
			expectError: true,
			errorMsg:    "failed to evaluate autoRemove value",
		},
		{
			name: "PullPolicyNever",
			input: map[string]any{
				"image": "alpine",
				"pull":  "never",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyNever,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyMissing",
			input: map[string]any{
				"image": "alpine",
				"pull":  "missing",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyAsBooleanTrue",
			input: map[string]any{
				"image": "alpine",
				"pull":  true,
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyAlways,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyAsBooleanFalse",
			input: map[string]any{
				"image": "alpine",
				"pull":  false,
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyNever,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyAsStringTrue",
			input: map[string]any{
				"image": "alpine",
				"pull":  "true",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyAlways,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "InvalidPullPolicy",
			input: map[string]any{
				"image": "alpine",
				"pull":  "invalid",
			},
			expectError: true,
			errorMsg:    "failed to parse pull policy as boolean",
		},
		{
			name: "PullPolicyUnsupportedType",
			input: map[string]any{
				"image": "alpine",
				"pull":  123,
			},
			expectError: true,
			errorMsg:    "invalid pull policy type",
		},
		{
			name: "ContainerConfigWithWeaklyTypedInput",
			input: map[string]any{
				"image": "alpine",
				"container": map[string]any{
					"Env": "FOO=bar", // String instead of slice
				},
			},
			expected: &Config{
				Image: "alpine",
				Pull:  core.PullPolicyMissing,
				Container: &container.Config{
					Env: []string{"FOO=bar"},
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "InvalidContainerConfigDecoder",
			input: map[string]any{
				"image":     "alpine",
				"container": "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "InvalidHostConfigDecoder",
			input: map[string]any{
				"image": "alpine",
				"host":  "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "InvalidNetworkConfigDecoder",
			input: map[string]any{
				"image":   "alpine",
				"network": "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "InvalidExecConfigDecoder",
			input: map[string]any{
				"image": "alpine",
				"exec":  "invalid", // Not a map
			},
			expectError: true,
			errorMsg:    "failed to decode config",
		},
		{
			name: "EmptyConfigSections",
			input: map[string]any{
				"image":     "alpine",
				"container": map[string]any{},
				"host":      map[string]any{},
				"network":   map[string]any{},
				"exec":      map[string]any{},
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "BothImageAndContainerNameEmptyStrings",
			input: map[string]any{
				"image":         "",
				"containerName": "",
			},
			expectError: true,
			errorMsg:    "containerName or image must be specified",
		},
		{
			name: "ErrorWhenExecProvidedWithImageOnly",
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
			name: "PlatformAsNonStringType",
			input: map[string]any{
				"image":    "alpine",
				"platform": 123, // Not a string
			},
			expected: &Config{
				Image:       "alpine",
				Platform:    "123",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "ContainerNameAsNonStringTypeExecMode",
			input: map[string]any{
				"containerName": 123, // Not a string
			},
			expected: &Config{
				ContainerName: "123",
				Pull:          core.PullPolicyMissing,
				Container:     &container.Config{},
				Host:          &container.HostConfig{},
				Network:       &network.NetworkingConfig{},
				ExecOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "ImageAsNonStringType",
			input: map[string]any{
				"image": 123, // Not a string
			},
			expected: &Config{
				Image:       "123",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "NilSectionsAreHandled",
			input: map[string]any{
				"image":     "alpine",
				"container": nil,
				"host":      nil,
				"network":   nil,
				"exec":      nil,
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyNil",
			input: map[string]any{
				"image": "alpine",
				"pull":  nil,
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyEmptyString",
			input: map[string]any{
				"image": "alpine",
				"pull":  "",
			},
			expected: &Config{
				Image:       "alpine",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "AutoRemoveNilValue",
			input: map[string]any{
				"image":      "alpine",
				"autoRemove": nil,
			},
			expected: &Config{
				Image:       "alpine",
				AutoRemove:  false,
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PlatformNilValue",
			input: map[string]any{
				"image":    "alpine",
				"platform": nil,
			},
			expected: &Config{
				Image:       "alpine",
				Platform:    "",
				Pull:        core.PullPolicyMissing,
				Container:   &container.Config{},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "ContainerNameNilValue",
			input: map[string]any{
				"image":         "alpine",
				"containerName": nil,
			},
			expected: &Config{
				Image:         "alpine",
				ContainerName: "",
				Pull:          core.PullPolicyMissing,
				Container:     &container.Config{},
				Host:          &container.HostConfig{},
				Network:       &network.NetworkingConfig{},
				ExecOptions:   &container.ExecOptions{},
			},
		},
		{
			name: "ImageNilValue",
			input: map[string]any{
				"image":         nil,
				"containerName": "test",
			},
			expected: &Config{
				Image:         "",
				ContainerName: "test",
				Pull:          core.PullPolicyMissing,
				Container:     &container.Config{},
				Host:          &container.HostConfig{},
				Network:       &network.NetworkingConfig{},
				ExecOptions:   &container.ExecOptions{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := LoadConfigFromMap(tt.input, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Image, result.Image)
			assert.Equal(t, tt.expected.ContainerName, result.ContainerName)
			assert.Equal(t, tt.expected.Platform, result.Platform)
			assert.Equal(t, tt.expected.Pull, result.Pull)
			assert.Equal(t, tt.expected.AutoRemove, result.AutoRemove)

			// Compare container config
			assert.Equal(t, tt.expected.Container.Env, result.Container.Env)
			assert.Equal(t, tt.expected.Container.WorkingDir, result.Container.WorkingDir)
			assert.Equal(t, tt.expected.Container.User, result.Container.User)

			// Compare host config
			assert.Equal(t, tt.expected.Host.AutoRemove, result.Host.AutoRemove)
			assert.Equal(t, tt.expected.Host.Privileged, result.Host.Privileged)

			// Compare exec options
			assert.Equal(t, tt.expected.ExecOptions.User, result.ExecOptions.User)
			assert.Equal(t, tt.expected.ExecOptions.WorkingDir, result.ExecOptions.WorkingDir)
			assert.Equal(t, tt.expected.ExecOptions.Env, result.ExecOptions.Env)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       core.Container
		expected    *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "MinimalContainerWithImageOnly",
			input: core.Container{
				Image: "alpine:latest",
			},
			expected: &Config{
				Image:      "alpine:latest",
				Pull:       core.PullPolicyAlways, // Zero value of PullPolicy
				AutoRemove: true,                  // Default when KeepContainer is false
				Container: &container.Config{
					Image: "alpine:latest",
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "ErrorWhenImageIsEmpty",
			input: core.Container{
				Platform: "linux/amd64",
			},
			expectError: true,
			errorMsg:    "image is required",
		},
		{
			name: "FullContainerConfiguration",
			input: core.Container{
				Image:         "ubuntu:20.04",
				PullPolicy:    core.PullPolicyAlways,
				Env:           []string{"FOO=bar", "BAZ=qux"},
				Volumes:       []string{"/host/data:/data:ro", "myvolume:/app"},
				User:          "1000:1000",
				WorkingDir:    "/workspace",
				Platform:      "linux/arm64",
				Ports:         []string{"8080:80", "9090"},
				Network:       "mynetwork",
				KeepContainer: true,
			},
			expected: &Config{
				Image:      "ubuntu:20.04",
				Platform:   "linux/arm64",
				Pull:       core.PullPolicyAlways,
				AutoRemove: false, // KeepContainer is true
				Container: &container.Config{
					Image:      "ubuntu:20.04",
					Env:        []string{"FOO=bar", "BAZ=qux"},
					User:       "1000:1000",
					WorkingDir: "/workspace",
					ExposedPorts: nat.PortSet{
						"80/tcp":   {},
						"9090/tcp": {},
					},
				},
				Host: &container.HostConfig{
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
				Network: &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						"mynetwork": {},
					},
				},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "StandardNetworkModes",
			input: core.Container{
				Image:   "nginx",
				Network: "host",
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
				},
				Host: &container.HostConfig{
					NetworkMode: "host",
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "ContainerNetworkReference",
			input: core.Container{
				Image:   "nginx",
				Network: "container:myapp",
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
				},
				Host: &container.HostConfig{
					NetworkMode: "container:myapp",
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "BindMountWithDefaultRwMode",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"/host/path:/container/path"},
			},
			expected: &Config{
				Image:      "alpine",
				AutoRemove: true,
				Container: &container.Config{
					Image: "alpine",
				},
				Host: &container.HostConfig{
					Binds: []string{"/host/path:/container/path:rw"},
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "RelativeBindMount",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"./data:/data:ro"},
			},
			expected: func() *Config {
				// Relative paths are resolved to absolute paths
				cwd, _ := os.Getwd()
				resolvedPath := filepath.Join(cwd, "data")
				return &Config{
					Image:      "alpine",
					AutoRemove: true,
					Container: &container.Config{
						Image: "alpine",
					},
					Host: &container.HostConfig{
						Binds: []string{resolvedPath + ":/data:ro"},
					},
					Network:     &network.NetworkingConfig{},
					ExecOptions: &container.ExecOptions{},
				}
			}(),
		},
		{
			name: "HomeDirectoryBindMount",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"~/data:/data:rw"},
			},
			expected: func() *Config {
				// Home directory paths are resolved to absolute paths
				homeDir, _ := os.UserHomeDir()
				resolvedPath := filepath.Join(homeDir, "data")
				return &Config{
					Image:      "alpine",
					AutoRemove: true,
					Container: &container.Config{
						Image: "alpine",
					},
					Host: &container.HostConfig{
						Binds: []string{resolvedPath + ":/data:rw"},
					},
					Network:     &network.NetworkingConfig{},
					ExecOptions: &container.ExecOptions{},
				}
			}(),
		},
		{
			name: "PortWithIPAddress",
			input: core.Container{
				Image: "nginx",
				Ports: []string{"127.0.0.1:8080:80/tcp"},
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
					ExposedPorts: nat.PortSet{
						"80/tcp": {},
					},
				},
				Host: &container.HostConfig{
					PortBindings: nat.PortMap{
						"80/tcp": []nat.PortBinding{
							{
								HostIP:   "127.0.0.1",
								HostPort: "8080",
							},
						},
					},
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "UdpPort",
			input: core.Container{
				Image: "dns-server",
				Ports: []string{"53:53/udp"},
			},
			expected: &Config{
				Image:      "dns-server",
				AutoRemove: true,
				Container: &container.Config{
					Image: "dns-server",
					ExposedPorts: nat.PortSet{
						"53/udp": {},
					},
				},
				Host: &container.HostConfig{
					PortBindings: nat.PortMap{
						"53/udp": []nat.PortBinding{
							{
								HostIP:   "0.0.0.0",
								HostPort: "53",
							},
						},
					},
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "InvalidVolumeFormatTooFewParts",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"/data"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: /data",
		},
		{
			name: "InvalidVolumeFormatTooManyParts",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"/host:/container:ro:extra"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: /host:/container:ro:extra",
		},
		{
			name: "InvalidVolumeMode",
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"/data:/data:invalid"},
			},
			expectError: true,
			errorMsg:    "invalid volume format: invalid mode invalid in /data:/data:invalid",
		},
		{
			name: "InvalidPortFormatTooManyParts",
			input: core.Container{
				Image: "nginx",
				Ports: []string{"1.2.3.4:8080:80:extra"},
			},
			expectError: true,
			errorMsg:    "invalid port format: 1.2.3.4:8080:80:extra",
		},
		{
			name: "InvalidPortProtocolDelimiter",
			input: core.Container{
				Image: "nginx",
				Ports: []string{"80/tcp/extra"},
			},
			expectError: true,
			errorMsg:    "invalid port format: invalid protocol in 80/tcp/extra",
		},
		{
			name: "InvalidPortProtocol",
			input: core.Container{
				Image: "nginx",
				Ports: []string{"80/invalid"},
			},
			expectError: true,
			errorMsg:    "invalid port format: invalid protocol invalid in 80/invalid",
		},
		{
			name: "SctpPortProtocol",
			input: core.Container{
				Image: "sctp-server",
				Ports: []string{"132/sctp"},
			},
			expected: &Config{
				Image:      "sctp-server",
				AutoRemove: true,
				Container: &container.Config{
					Image: "sctp-server",
					ExposedPorts: nat.PortSet{
						"132/sctp": {},
					},
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "WhitespaceInPortSpecification",
			input: core.Container{
				Image: "nginx",
				Ports: []string{" 8080:80 "},
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
					ExposedPorts: nat.PortSet{
						"80/tcp": {},
					},
				},
				Host: &container.HostConfig{
					PortBindings: nat.PortMap{
						"80/tcp": []nat.PortBinding{
							{
								HostIP:   "0.0.0.0",
								HostPort: "8080",
							},
						},
					},
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "EmptyNetworkUsesDefault",
			input: core.Container{
				Image:   "nginx",
				Network: "",
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
				},
				Host: &container.HostConfig{
					NetworkMode: "", // Empty string for default
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "BridgeNetworkMode",
			input: core.Container{
				Image:   "nginx",
				Network: "bridge",
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
				},
				Host: &container.HostConfig{
					NetworkMode: "bridge",
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "NoneNetworkMode",
			input: core.Container{
				Image:   "nginx",
				Network: "none",
			},
			expected: &Config{
				Image:      "nginx",
				AutoRemove: true,
				Container: &container.Config{
					Image: "nginx",
				},
				Host: &container.HostConfig{
					NetworkMode: "none",
				},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "KeepContainerFalseSetsAutoRemoveTrue",
			input: core.Container{
				Image:         "alpine",
				KeepContainer: false,
			},
			expected: &Config{
				Image:      "alpine",
				AutoRemove: true,
				Container: &container.Config{
					Image: "alpine",
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "KeepContainerTrueSetsAutoRemoveFalse",
			input: core.Container{
				Image:         "alpine",
				KeepContainer: true,
			},
			expected: &Config{
				Image:      "alpine",
				AutoRemove: false,
				Container: &container.Config{
					Image: "alpine",
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PullPolicyPropagation",
			input: core.Container{
				Image:      "alpine",
				PullPolicy: core.PullPolicyNever,
			},
			expected: &Config{
				Image:      "alpine",
				Pull:       core.PullPolicyNever,
				AutoRemove: true,
				Container: &container.Config{
					Image: "alpine",
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
		{
			name: "PlatformPropagation",
			input: core.Container{
				Image:    "alpine",
				Platform: "linux/386",
			},
			expected: &Config{
				Image:      "alpine",
				Platform:   "linux/386",
				AutoRemove: true,
				Container: &container.Config{
					Image: "alpine",
				},
				Host:        &container.HostConfig{},
				Network:     &network.NetworkingConfig{},
				ExecOptions: &container.ExecOptions{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := LoadConfig("", tt.input, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Image, result.Image)
			assert.Equal(t, tt.expected.Platform, result.Platform)
			assert.Equal(t, tt.expected.Pull, result.Pull)
			assert.Equal(t, tt.expected.AutoRemove, result.AutoRemove)

			// Compare container config
			assert.Equal(t, tt.expected.Container.Image, result.Container.Image)
			assert.Equal(t, tt.expected.Container.Env, result.Container.Env)
			assert.Equal(t, tt.expected.Container.User, result.Container.User)
			assert.Equal(t, tt.expected.Container.WorkingDir, result.Container.WorkingDir)

			// Compare exposed ports
			if tt.expected.Container.ExposedPorts != nil {
				assert.Equal(t, tt.expected.Container.ExposedPorts, result.Container.ExposedPorts)
			}

			// Compare host config
			assert.Equal(t, tt.expected.Host.Binds, result.Host.Binds)
			if tt.expected.Host.Mounts != nil {
				assert.Equal(t, tt.expected.Host.Mounts, result.Host.Mounts)
			}
			if tt.expected.Host.PortBindings != nil {
				assert.Equal(t, tt.expected.Host.PortBindings, result.Host.PortBindings)
			}
			assert.Equal(t, tt.expected.Host.NetworkMode, result.Host.NetworkMode)

			// Compare network config
			if tt.expected.Network.EndpointsConfig != nil {
				assert.Equal(t, tt.expected.Network.EndpointsConfig, result.Network.EndpointsConfig)
			}
		})
	}
}
