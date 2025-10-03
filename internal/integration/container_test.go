package integration_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	testImage       = "alpine:3"
	nginxTestImage  = "nginx:alpine"
	containerPrefix = "dagu-test"
)

type dockerExecutorTest struct {
	name            string
	dagConfig       string
	expectedOutputs map[string]any
}

func TestDockerExecutor(t *testing.T) {
	t.Parallel()

	tests := []dockerExecutorTest{
		{
			name: "BasicExecution",
			dagConfig: `
env:
  - FOO: BAR
  - ABC=XYZ
steps:
  - executor:
      type: docker
      config:
        image: alpine:3
        autoRemove: true
    command: echo 123 abc $FOO $ABC
    output: DOCKER_EXEC_OUT1
`,
			expectedOutputs: map[string]any{
				"DOCKER_EXEC_OUT1": "123 abc BAR XYZ",
			},
		},
		{
			name: "AutoStartContainer",
			dagConfig: `
steps:
  - executor:
      type: docker
      config:
        image: alpine:3
        autoRemove: true
        containerName: dagu-autostart
    command: echo "container started"
    output: DOCKER_EXEC_OUT1
`,
			expectedOutputs: map[string]any{
				"DOCKER_EXEC_OUT1": "container started",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)
			dag := th.DAG(t, tt.dagConfig)
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, status.Success)
			dag.AssertOutputs(t, tt.expectedOutputs)
		})
	}
}

type containerTest struct {
	name            string
	dagConfigFunc   func(tempDir string) string
	expectedOutputs map[string]any
	setupFunc       func(t *testing.T, tempDir string)
}

func TestDAGLevelContainer(t *testing.T) {
	t.Parallel()

	tests := []containerTest{
		{
			name: "VolumeBindMounts",
			dagConfigFunc: func(tempDir string) string {
				return fmt.Sprintf(`
container:
  image: %s
  volumes:
    - %s:/data:rw
steps:
  - sh -c "echo 'Hello from step 1' > /data/test.txt"
  - command: cat /data/test.txt
    output: BIND_MOUNT_OUT1
  - sh -c "echo 'Hello from step 3' >> /data/test.txt"
  - command: cat /data/test.txt
    output: BIND_MOUNT_OUT2
`, testImage, tempDir)
			},
			expectedOutputs: map[string]any{
				"BIND_MOUNT_OUT1": "Hello from step 1",
				"BIND_MOUNT_OUT2": "Hello from step 1\nHello from step 3",
			},
		},
		{
			name: "BasicExecution",
			dagConfigFunc: func(_ string) string {
				return fmt.Sprintf(`
env:
  - FOO: BAR
container:
  image: %s
steps:
  - command: echo 123 abc $FOO
    output: CONTAINER_BASIC_OUT1
`, testImage)
			},
			expectedOutputs: map[string]any{
				"CONTAINER_BASIC_OUT1": "123 abc BAR",
			},
		},
		{
			name: "CommandWithArguments",
			dagConfigFunc: func(_ string) string {
				return fmt.Sprintf(`
container:
  image: %s
steps:
  - command: echo hello world
    output: CMD_WITH_ARGS_OUT1
`, testImage)
			},
			expectedOutputs: map[string]any{
				"CMD_WITH_ARGS_OUT1": "hello world",
			},
		},
		{
			name: "WorkingDirectory",
			dagConfigFunc: func(_ string) string {
				return fmt.Sprintf(`
container:
  image: %s
  workingDir: /tmp
steps:
  - command: pwd
    output: WORK_DIR_OUT1
`, testImage)
			},
			expectedOutputs: map[string]any{
				"WORK_DIR_OUT1": "/tmp",
			},
		},
		{
			name: "UserSpecification",
			dagConfigFunc: func(_ string) string {
				return fmt.Sprintf(`
container:
  image: %s
  user: "nobody"
steps:
  - command: whoami
    output: WITH_USER_OUT1
`, testImage)
			},
			expectedOutputs: map[string]any{
				"WITH_USER_OUT1": "nobody",
			},
		},
		{
			name: "NamedVolume",
			dagConfigFunc: func(_ string) string {
				return fmt.Sprintf(`
container:
  image: %s
  volumes:
    - test-volume:/data
steps:
  - sh -c "echo 'Data in named volume' > /data/volume.txt"
  - command: cat /data/volume.txt
    output: NAMED_VOL_OUT1
  - command: ls -la /data/
    output: NAMED_VOL_OUT2
`, testImage)
			},
			expectedOutputs: map[string]any{
				"NAMED_VOL_OUT1": "Data in named volume",
			},
		},
		{
			name: "RelativeBindMountsWithWorkingDirectory",
			setupFunc: func(t *testing.T, tempDir string) {
				subDir := fmt.Sprintf("%s/work", tempDir)
				if err := os.MkdirAll(subDir, 0o755); err != nil {
					t.Fatalf("failed to create subdirectory %s: %v", subDir, err)
				}

				testFile := fmt.Sprintf("%s/initial.txt", subDir)
				if err := os.WriteFile(testFile, []byte("Initial content"), 0o644); err != nil {
					t.Fatalf("failed to create test file %s: %v", testFile, err)
				}
			},
			dagConfigFunc: func(tempDir string) string {
				subDir := fmt.Sprintf("%s/work", tempDir)
				return fmt.Sprintf(`
workingDir: %s
container:
  image: %s
  volumes:
    - ./:/workspace:rw
steps:
  - command: cat /workspace/initial.txt
    output: WORK_DIR_VOL_OUT1
  - sh -c "echo 'New content' > /workspace/new.txt"
  - command: cat /workspace/new.txt
    output: WORK_DIR_VOL_OUT2
  - command: ls -la /workspace/
    output: WORK_DIR_VOL_OUT3
`, subDir, testImage)
			},
			expectedOutputs: map[string]any{
				"WORK_DIR_VOL_OUT1": "Initial content",
				"WORK_DIR_VOL_OUT2": "New content",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir, err := os.MkdirTemp("", fmt.Sprintf("%s-%s-*", containerPrefix, tt.name))
			require.NoError(t, err, "failed to create temporary directory")
			t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, tempDir)
			}

			th := test.Setup(t)
			dag := th.DAG(t, tt.dagConfigFunc(tempDir))
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, status.Success)
			dag.AssertOutputs(t, tt.expectedOutputs)
		})
	}
}

func TestContainerPullPolicy(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	pullPolicyTestDAG := fmt.Sprintf(`
container:
  image: %s
  pullPolicy: never
steps:
  - command: echo 'pull policy test'
    output: OUT1
`, testImage)

	ensureImageDAG := fmt.Sprintf(`
container:
  image: %s
steps:
  - "true"
`, testImage)

	// First, ensure the image exists by running with default pull policy
	ensureImageDag := th.DAG(t, ensureImageDAG)
	ensureImageDag.Agent().RunSuccess(t)

	// Now test that pull policy "never" works with the pre-existing image
	dag := th.DAG(t, pullPolicyTestDAG)
	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"OUT1": "pull policy test",
	})
}

func TestContainerStartup_Entrypoint_WithHealthyFallback(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Use nginx which stays up by default; most tags have no healthcheck,
	// so waitFor: healthy should fall back to running.
	dagConfig := fmt.Sprintf(`
container:
  image: %s
  startup: entrypoint
  waitFor: healthy
steps:
  - command: echo entrypoint-ok
    output: ENTRYPOINT_OK
`, nginxTestImage)

	dag := th.DAG(t, dagConfig)
	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"ENTRYPOINT_OK": "entrypoint-ok",
	})
}

func TestContainerStartup_Command_LongRunning(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	dagConfig := fmt.Sprintf(`
container:
  image: %s
  startup: command
  command: ["sh", "-c", "while true; do sleep 3600; done"]
steps:
  - command: echo command-ok
    output: COMMAND_OK
`, testImage)

	dag := th.DAG(t, dagConfig)
	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"COMMAND_OK": "command-ok",
	})
}

func TestDockerExecutor_ExecInExistingContainer(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "failed to create docker client")
	defer func() { _ = dockerClient.Close() }()

	containerName := fmt.Sprintf("dagu-existing-%d", time.Now().UnixNano())
	containerID := createLongRunningContainer(t, th, dockerClient, containerName)
	defer removeContainer(t, th, dockerClient, containerID)

	dagConfig := fmt.Sprintf(`
steps:
  - executor:
      type: docker
      config:
        containerName: %s
        exec:
          workingDir: /
    command: echo hello-existing
    output: EXEC_EXISTING_OUT
`, containerName)

	dag := th.DAG(t, dagConfig)
	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"EXEC_EXISTING_OUT": "hello-existing",
	})
}

func TestDockerExecutor_ErrorIncludesRecentStderr(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	dagConfig := fmt.Sprintf(`
steps:
  - executor:
      type: docker
      config:
        image: %s
        autoRemove: true
    command: sh -c 'echo first 1>&2; echo second 1>&2; exit 7'
`, testImage)

	dag := th.DAG(t, dagConfig)
	agent := dag.Agent()

	err := agent.Run(agent.Context)
	require.Error(t, err)

	// Should contain recent stderr from docker executor
	require.Contains(t, err.Error(), "first")
	require.Contains(t, err.Error(), "second")
}

// Helper functions
func createLongRunningContainer(t *testing.T, th test.Helper, dockerClient *client.Client, containerName string) string {
	t.Helper()

	info, err := dockerClient.Info(th.Context)
	if err != nil {
		t.Fatalf("failed to get docker info: %v", err)
	}

	var platform specs.Platform
	platform.Architecture = info.Architecture
	platform.OS = info.OSType

	pullOpts := image.PullOptions{Platform: platforms.Format(platform)}

	// Pull the image to ensure it exists; consume the stream so the daemon registers it
	reader, err := dockerClient.ImagePull(th.Context, testImage, pullOpts)
	if err != nil {
		t.Fatalf("failed to pull image %s: %v", testImage, err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		_ = reader.Close()
		t.Fatalf("failed to read pull response for %s: %v", testImage, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("failed to close pull response for %s: %v", testImage, err)
	}

	// Create and start the container
	created, err := dockerClient.ContainerCreate(
		th.Context,
		&container.Config{
			Image: testImage,
			Cmd:   []string{"sh", "-c", "while true; do sleep 3600; done"},
		},
		&container.HostConfig{AutoRemove: true},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	if err := dockerClient.ContainerStart(th.Context, created.ID, container.StartOptions{}); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to be running
	if err := waitForContainerRunning(th.Context, dockerClient, created.ID); err != nil {
		t.Fatalf("failed to wait for container to be running: %v", err)
	}

	return created.ID
}

func removeContainer(t *testing.T, th test.Helper, dockerClient *client.Client, containerID string) {
	t.Helper()

	const (
		stopTimeout  = 5 * time.Second
		pollInterval = 100 * time.Millisecond
	)

	// Stop the container gracefully
	if err := dockerClient.ContainerStop(th.Context, containerID, container.StopOptions{}); err != nil {
		t.Logf("failed to stop container %s: %v", containerID, err)
	}

	// Wait for container to stop with timeout
	if !waitForContainerStop(t, th, dockerClient, containerID, stopTimeout, pollInterval) {
		t.Logf("timeout waiting for container %s to stop, forcing removal", containerID)
	}

	// Remove the container
	if err := dockerClient.ContainerRemove(th.Context, containerID, container.RemoveOptions{Force: true}); err != nil {
		t.Logf("failed to remove container %s: %v", containerID, err)
	}
}

func waitForContainerRunning(ctx context.Context, dockerClient *client.Client, containerID string) error {
	const (
		timeout      = 10 * time.Second
		pollInterval = 100 * time.Millisecond
	)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for container %s to be running", containerID)
		case <-ticker.C:
			inspect, err := dockerClient.ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
			}
			if inspect.State.Running {
				return nil
			}
		}
	}
}

func waitForContainerStop(t *testing.T, th test.Helper, dockerClient *client.Client, containerID string, timeout, pollInterval time.Duration) bool {
	t.Helper()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-timeoutChan:
			return false
		case <-ticker.C:
			inspect, err := dockerClient.ContainerInspect(th.Context, containerID)
			if err != nil {
				// Container might have been removed or doesn't exist
				return true
			}
			if !inspect.State.Running {
				return true
			}
		}
	}
}
