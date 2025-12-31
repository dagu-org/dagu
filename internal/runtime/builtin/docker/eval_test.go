package docker

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalContainerFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) context.Context
		input    core.Container
		expected core.Container
	}{
		{
			name: "NoVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image:      "alpine:latest",
				Name:       "my-container",
				WorkingDir: "/app",
			},
			expected: core.Container{
				Image:      "alpine:latest",
				Name:       "my-container",
				WorkingDir: "/app",
			},
		},
		{
			name: "ImageVariable",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["IMAGE"] = "myimage:v1.0"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image: "${IMAGE}",
			},
			expected: core.Container{
				Image: "myimage:v1.0",
			},
		},
		{
			name: "MultipleVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["IMAGE"] = "nginx:latest"
				env.Envs["CONTAINER_NAME"] = "web-server"
				env.Envs["WORK_DIR"] = "/var/www"
				env.Envs["NET"] = "my-network"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image:      "${IMAGE}",
				Name:       "${CONTAINER_NAME}",
				WorkingDir: "${WORK_DIR}",
				Network:    "${NET}",
			},
			expected: core.Container{
				Image:      "nginx:latest",
				Name:       "web-server",
				WorkingDir: "/var/www",
				Network:    "my-network",
			},
		},
		{
			name: "VolumesWithVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["HOST_PATH"] = "/host/data"
				env.Envs["CONTAINER_PATH"] = "/container/data"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image:   "alpine",
				Volumes: []string{"${HOST_PATH}:${CONTAINER_PATH}:ro"},
			},
			expected: core.Container{
				Image:   "alpine",
				Volumes: []string{"/host/data:/container/data:ro"},
			},
		},
		{
			name: "PortsWithVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["HOST_PORT"] = "8080"
				env.Envs["CONTAINER_PORT"] = "80"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image: "nginx",
				Ports: []string{"${HOST_PORT}:${CONTAINER_PORT}"},
			},
			expected: core.Container{
				Image: "nginx",
				Ports: []string{"8080:80"},
			},
		},
		{
			name: "EnvWithVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["DB_HOST"] = "localhost"
				env.Envs["DB_PORT"] = "5432"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image: "myapp",
				Env:   []string{"DATABASE_URL=postgres://${DB_HOST}:${DB_PORT}/db"},
			},
			expected: core.Container{
				Image: "myapp",
				Env:   []string{"DATABASE_URL=postgres://localhost:5432/db"},
			},
		},
		{
			name: "CommandWithVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["SCRIPT"] = "run.sh"
				env.Envs["ARG1"] = "value1"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image:   "alpine",
				Command: []string{"/bin/sh", "${SCRIPT}", "${ARG1}"},
			},
			expected: core.Container{
				Image:   "alpine",
				Command: []string{"/bin/sh", "run.sh", "value1"},
			},
		},
		{
			name: "UserWithVariable",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["UID"] = "1000"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image: "alpine",
				User:  "${UID}",
			},
			expected: core.Container{
				Image: "alpine",
				User:  "1000",
			},
		},
		{
			name: "NonEvaluatedFieldsRemainUnchanged",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["POLICY"] = "always"
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image:         "alpine",
				PullPolicy:    core.PullPolicyAlways,
				KeepContainer: true,
				Startup:       core.StartupCommand,
				WaitFor:       core.WaitForHealthy,
				Platform:      "linux/amd64",
				LogPattern:    "ready.*started",
				RestartPolicy: "on-failure",
			},
			expected: core.Container{
				Image:         "alpine",
				PullPolicy:    core.PullPolicyAlways,
				KeepContainer: true,
				Startup:       core.StartupCommand,
				WaitFor:       core.WaitForHealthy,
				Platform:      "linux/amd64",
				LogPattern:    "ready.*started",
				RestartPolicy: "on-failure",
			},
		},
		{
			name: "OutputFromPreviousStep",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Variables.Store("IMAGE_TAG", "IMAGE_TAG=v2.0.0")
				return runtime.WithEnv(ctx, env)
			},
			input: core.Container{
				Image: "myapp:${IMAGE_TAG}",
			},
			expected: core.Container{
				Image: "myapp:v2.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup(context.Background())

			result, err := EvalContainerFields(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) context.Context
		input    []string
		expected []string
	}{
		{
			name: "EmptySlice",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				return runtime.WithEnv(ctx, env)
			},
			input:    []string{},
			expected: []string{},
		},
		{
			name: "NilSlice",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				return runtime.WithEnv(ctx, env)
			},
			input:    nil,
			expected: nil,
		},
		{
			name: "NoVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				return runtime.WithEnv(ctx, env)
			},
			input:    []string{"hello", "world"},
			expected: []string{"hello", "world"},
		},
		{
			name: "WithVariables",
			setup: func(ctx context.Context) context.Context {
				env := runtime.NewEnv(ctx, core.Step{Name: "test"})
				env.Envs["VAR1"] = "value1"
				env.Envs["VAR2"] = "value2"
				return runtime.WithEnv(ctx, env)
			},
			input:    []string{"${VAR1}", "${VAR2}", "static"},
			expected: []string{"value1", "value2", "static"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup(context.Background())

			result, err := evalStringSlice(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
