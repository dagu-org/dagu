package gha

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRunnerConfig(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "/workdir", config.Workdir)
		assert.True(t, config.BindWorkdir)
		assert.Equal(t, defaultEventName, config.EventName)
		assert.Equal(t, "/event.json", config.EventPath)
		assert.Equal(t, defaultGitHubHost, config.GitHubInstance)
		assert.True(t, config.LogOutput)
		assert.Equal(t, defaultRunnerImage, config.Platforms[defaultPlatform])
		assert.True(t, config.AutoRemove)
		assert.False(t, config.ReuseContainers)
		assert.False(t, config.ForceRebuild)
		assert.False(t, config.Privileged)
		assert.Empty(t, config.ContainerDaemonSocket)
		assert.Empty(t, config.ContainerOptions)
		assert.Empty(t, config.ContainerCapAdd)
		assert.Empty(t, config.ContainerCapDrop)
		assert.Empty(t, config.ArtifactServerPath)
		assert.Empty(t, config.ArtifactServerPort)
	})

	t.Run("CustomRunner", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"runner": "node:24-bookworm-slim",
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "node:24-bookworm-slim", config.Platforms[defaultPlatform])
	})

	t.Run("AutoRemoveFalse", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"autoRemove": false,
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.False(t, config.AutoRemove)
	})

	t.Run("NetworkMode", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"network": "host",
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, container.NetworkMode("host"), config.ContainerNetworkMode)
	})

	t.Run("GitHubInstance", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"githubInstance": "github.company.com",
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "github.company.com", config.GitHubInstance)
	})

	t.Run("DockerSocket", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"dockerSocket": "/custom/docker.sock",
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "/custom/docker.sock", config.ContainerDaemonSocket)
	})

	t.Run("ContainerOptions", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"containerOptions": "--memory=2g --cpus=2",
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "--memory=2g --cpus=2", config.ContainerOptions)
	})

	t.Run("ReuseContainers", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"reuseContainers": true,
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.True(t, config.ReuseContainers)
	})

	t.Run("ForceRebuild", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"forceRebuild": true,
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.True(t, config.ForceRebuild)
	})

	t.Run("Privileged", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"privileged": true,
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.True(t, config.Privileged)
	})

	t.Run("CapabilitiesAdd", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"capabilities": map[string]any{
							"add": []string{"SYS_ADMIN", "NET_ADMIN"},
						},
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		require.Len(t, config.ContainerCapAdd, 2)
		assert.Equal(t, "SYS_ADMIN", config.ContainerCapAdd[0])
		assert.Equal(t, "NET_ADMIN", config.ContainerCapAdd[1])
	})

	t.Run("CapabilitiesDrop", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"capabilities": map[string]any{
							"drop": []string{"NET_RAW", "CHOWN"},
						},
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		require.Len(t, config.ContainerCapDrop, 2)
		assert.Equal(t, "NET_RAW", config.ContainerCapDrop[0])
		assert.Equal(t, "CHOWN", config.ContainerCapDrop[1])
	})

	t.Run("CapabilitiesAddAndDrop", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"capabilities": map[string]any{
							"add":  []string{"SYS_ADMIN"},
							"drop": []string{"NET_RAW"},
						},
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		require.Len(t, config.ContainerCapAdd, 1)
		assert.Equal(t, "SYS_ADMIN", config.ContainerCapAdd[0])
		require.Len(t, config.ContainerCapDrop, 1)
		assert.Equal(t, "NET_RAW", config.ContainerCapDrop[0])
	})

	t.Run("Artifacts", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"artifacts": map[string]any{
							"path": "/tmp/artifacts",
							"port": "34567",
						},
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "/tmp/artifacts", config.ArtifactServerPath)
		assert.Equal(t, "34567", config.ArtifactServerPort)
	})

	t.Run("EnvAndSecrets", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{},
				},
			},
		}

		actEnv := map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		}
		actSecrets := map[string]string{
			"SECRET1": "value1",
			"SECRET2": "value2",
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", actEnv, actSecrets)

		assert.Equal(t, actEnv, config.Env)
		assert.Equal(t, actSecrets, config.Secrets)
	})

	t.Run("ComplexConfig", func(t *testing.T) {
		e := &githubAction{
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Config: map[string]any{
						"runner":           "catthehacker/ubuntu:full-latest",
						"autoRemove":       false,
						"network":          "host",
						"githubInstance":   "github.enterprise.com",
						"dockerSocket":     "/custom/socket",
						"containerOptions": "--memory=4g",
						"reuseContainers":  true,
						"forceRebuild":     true,
						"privileged":       true,
						"capabilities": map[string]any{
							"add":  []string{"SYS_ADMIN"},
							"drop": []string{"NET_RAW"},
						},
						"artifacts": map[string]any{
							"path": "/artifacts",
							"port": "8080",
						},
					},
				},
			},
		}

		config := e.buildRunnerConfig("/workdir", "/event.json", map[string]string{}, map[string]string{})

		assert.Equal(t, "catthehacker/ubuntu:full-latest", config.Platforms[defaultPlatform])
		assert.False(t, config.AutoRemove)
		assert.Equal(t, container.NetworkMode("host"), config.ContainerNetworkMode)
		assert.Equal(t, "github.enterprise.com", config.GitHubInstance)
		assert.Equal(t, "/custom/socket", config.ContainerDaemonSocket)
		assert.Equal(t, "--memory=4g", config.ContainerOptions)
		assert.True(t, config.ReuseContainers)
		assert.True(t, config.ForceRebuild)
		assert.True(t, config.Privileged)
		assert.Len(t, config.ContainerCapAdd, 1)
		assert.Equal(t, "SYS_ADMIN", config.ContainerCapAdd[0])
		assert.Len(t, config.ContainerCapDrop, 1)
		assert.Equal(t, "NET_RAW", config.ContainerCapDrop[0])
		assert.Equal(t, "/artifacts", config.ArtifactServerPath)
		assert.Equal(t, "8080", config.ArtifactServerPort)
	})
}

func TestParseStringSlice(t *testing.T) {
	t.Run("StringSlice", func(t *testing.T) {
		input := []string{"foo", "bar", "baz"}
		result := parseStringSlice(input)
		assert.Equal(t, input, result)
	})

	t.Run("EmptyStringSlice", func(t *testing.T) {
		input := []string{}
		result := parseStringSlice(input)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("AnySlice", func(t *testing.T) {
		input := []any{"foo", "bar", 123}
		result := parseStringSlice(input)
		require.Len(t, result, 3)
		assert.Equal(t, "foo", result[0])
		assert.Equal(t, "bar", result[1])
		assert.Equal(t, "123", result[2])
	})

	t.Run("EmptyAnySlice", func(t *testing.T) {
		input := []any{}
		result := parseStringSlice(input)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("AnySliceWithMixedTypes", func(t *testing.T) {
		input := []any{"string", 42, true, 3.14}
		result := parseStringSlice(input)
		require.Len(t, result, 4)
		assert.Equal(t, "string", result[0])
		assert.Equal(t, "42", result[1])
		assert.Equal(t, "true", result[2])
		assert.Equal(t, "3.14", result[3])
	})

	t.Run("AnySliceWithNil", func(t *testing.T) {
		input := []any{"foo", nil, "bar"}
		result := parseStringSlice(input)
		require.Len(t, result, 3)
		assert.Equal(t, "foo", result[0])
		assert.Equal(t, "<nil>", result[1])
		assert.Equal(t, "bar", result[2])
	})

	t.Run("SingleString", func(t *testing.T) {
		input := "single"
		result := parseStringSlice(input)
		require.Len(t, result, 1)
		assert.Equal(t, "single", result[0])
	})

	t.Run("EmptyString", func(t *testing.T) {
		input := ""
		result := parseStringSlice(input)
		assert.Nil(t, result)
	})

	t.Run("NilInput", func(t *testing.T) {
		result := parseStringSlice(nil)
		assert.Nil(t, result)
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		input := 123
		result := parseStringSlice(input)
		assert.Nil(t, result)
	})

	t.Run("UnsupportedTypeMap", func(t *testing.T) {
		input := map[string]string{"key": "value"}
		result := parseStringSlice(input)
		assert.Nil(t, result)
	})
}
