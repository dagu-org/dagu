package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

func TestDockerExecutor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		dag             string
		expectedOutputs map[string]any
	}{
		{
			name: "basic",
			dag: `
name: test-basic
env:
  - FOO: BAR
  - ABC=XYZ
steps:
  - name: s1
    executor:
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)
			dag := th.DAG(t, tc.dag)
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, status.Success)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}

func TestContainer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		dagFunc         func(tempDir string) string
		expectedOutputs map[string]any
	}{
		{
			name: "volume_bind_mount_persistence",
			dagFunc: func(tempDir string) string {
				return fmt.Sprintf(`
name: test-bind-mount
container:
  image: alpine:3
  volumes:
    - %s:/data:rw
steps:
  - name: write_data
    command: sh -c "echo 'Hello from step 1' > /data/test.txt"
  - name: read_data
    command: cat /data/test.txt
    output: BIND_MOUNT_OUT1
  - name: append_data
    command: sh -c "echo 'Hello from step 3' >> /data/test.txt"
  - name: read_all
    command: cat /data/test.txt
    output: BIND_MOUNT_OUT2
`, tempDir)
			},
			expectedOutputs: map[string]any{
				"BIND_MOUNT_OUT1": "Hello from step 1",
				"BIND_MOUNT_OUT2": "Hello from step 1\nHello from step 3",
			},
		},
		{
			name: "basic",
			dagFunc: func(_ string) string {
				return `
name: test-basic
env:
  - FOO: BAR
container:
  image: alpine:3
steps:
  - name: s1
    command: echo 123 abc $FOO
    output: CONTAINER_BASIC_OUT1
`
			},
			expectedOutputs: map[string]any{
				"CONTAINER_BASIC_OUT1": "123 abc BAR",
			},
		},
		{
			name: "command_with_args",
			dagFunc: func(_ string) string {
				return `
name: test-command-with-args
container:
  image: alpine:3
steps:
  - name: s1
    command: echo hello world
    output: CMD_WITH_ARGS_OUT1
`
			},
			expectedOutputs: map[string]any{
				"CMD_WITH_ARGS_OUT1": "hello world",
			},
		},
		{
			name: "working_directory",
			dagFunc: func(_ string) string {
				return `
name: test-working-dir
container:
  image: alpine:3
  workingDir: /tmp
steps:
  - name: s1
    command: "pwd"
    output: WORK_DIR_OUT1
`
			},
			expectedOutputs: map[string]any{
				"WORK_DIR_OUT1": "/tmp",
			},
		},
		{
			name: "container_with_user",
			dagFunc: func(_ string) string {
				return `
name: test-user
container:
  image: alpine:3
  user: "nobody"
steps:
  - name: s1
    command: "whoami"
    output: WITH_USER_OUT1
`
			},
			expectedOutputs: map[string]any{
				"WITH_USER_OUT1": "nobody",
			},
		},
		{
			name: "volume_named_persistence",
			dagFunc: func(_ string) string {
				return `
name: test-named-volume
container:
  image: alpine:3
  volumes:
    - test-volume:/data
steps:
  - name: create_file
    command: sh -c "echo 'Data in named volume' > /data/volume.txt"
  - name: verify_file
    command: "cat /data/volume.txt"
    output: NAMED_VOL_OUT1
  - name: list_files
    command: "ls -la /data/"
    output: NAMED_VOL_OUT2
`
			},
			expectedOutputs: map[string]any{
				"NAMED_VOL_OUT1": "Data in named volume",
			},
		},
		{
			name: "relative_volume_with_working_dir",
			dagFunc: func(tempDir string) string {
				// Create a subdirectory to use as working directory
				subDir := fmt.Sprintf("%s/work", tempDir)
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("Failed to create subdirectory: %v", err)
				}

				// Create a test file in the working directory
				testFile := fmt.Sprintf("%s/initial.txt", subDir)
				if err := os.WriteFile(testFile, []byte("Initial content"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}

				return fmt.Sprintf(`
name: test-relative-volume
workingDir: %s
container:
  image: alpine:3
  volumes:
    - ./:/workspace:rw
steps:
  - name: read_initial
    command: cat /workspace/initial.txt
    output: WORK_DIR_VOL_OUT1
  - name: write_new
    command: sh -c "echo 'New content' > /workspace/new.txt"
  - name: verify_new
    command: cat /workspace/new.txt
    output: WORK_DIR_VOL_OUT2
  - name: list_workspace
    command: ls -la /workspace/
    output: WORK_DIR_VOL_OUT3
`, subDir)
			},
			expectedOutputs: map[string]any{
				"WORK_DIR_VOL_OUT1": "Initial content",
				"WORK_DIR_VOL_OUT2": "New content",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a unique temporary directory for this test
			tempDir, err := os.MkdirTemp("", fmt.Sprintf("dagu-test-%s-*", tc.name))
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() {
				_ = os.RemoveAll(tempDir)
			}()

			th := test.Setup(t)
			dag := th.DAG(t, tc.dagFunc(tempDir))
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, status.Success)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}

func TestContainerPullPolicy(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Test that pull policy "never" works with a pre-existing image
	dag := th.DAG(t, `
container:
  image: alpine:3
  pullPolicy: never
steps:
  - name: s1
    command: "echo 'pull policy test'"
    output: OUT1
`)

	// First, ensure the image exists by running with default pull policy
	ensureImageDag := th.DAG(t, `
container:
  image: alpine:3
steps:
  - name: s1
    command: "true"
`)
	ensureImageDag.Agent().RunSuccess(t)

	// Now run with "never" pull policy
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
	dag := th.DAG(t, `
name: container-startup-entrypoint
container:
  image: nginx:alpine
  startup: entrypoint
  waitFor: healthy
steps:
  - name: s1
    command: echo entrypoint-ok
    output: ENTRYPOINT_OK
`)

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"ENTRYPOINT_OK": "entrypoint-ok",
	})
}

func TestContainerStartup_Command_LongRunning(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	dag := th.DAG(t, `
name: container-startup-command
container:
  image: alpine:3
  startup: command
  command: ["sh", "-c", "while true; do sleep 3600; done"]
steps:
  - name: s1
    command: echo command-ok
    output: COMMAND_OK
`)

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"COMMAND_OK": "command-ok",
	})
}

// TestDockerExecutor_ExecInExistingContainer verifies that a step-level Docker executor
// can execute a command in an already-running container by specifying `containerName`
// without an `image`. This reproduces the reported regression where containerName
// was ignored and caused an error: "containerName or image must be specified".
func TestDockerExecutor_ExecInExistingContainer(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Ensure the alpine image exists locally (avoid pulling via SDK in tests)
	ensureImage := th.DAG(t, `
container:
  image: alpine:3
steps:
  - name: s1
    command: "true"
`)
	ensureImage.Agent().RunSuccess(t)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	// Create a long-running container we can exec into
	cname := fmt.Sprintf("dagu-integ-existing-%d", time.Now().UnixNano())
	created, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: "alpine:3",
			Cmd:   []string{"sh", "-c", "while true; do sleep 3600; done"},
		},
		&container.HostConfig{AutoRemove: true},
		nil,
		nil,
		cname,
	)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Ensure cleanup
	t.Cleanup(func() {
		// Stop (ignore error if already stopped) and remove the container
		_ = cli.ContainerStop(context.Background(), created.ID, container.StopOptions{})
		_ = cli.ContainerRemove(context.Background(), created.ID, container.RemoveOptions{Force: true})
	})

	// Run a DAG step that execs into the existing container via containerName
	dag := th.DAG(t, fmt.Sprintf(`
name: exec-in-existing-container
steps:
  - name: exec-existing
    executor:
      type: docker
      config:
        containerName: %s
        exec:
          workingDir: /
    command: echo hello-existing
    output: EXEC_EXISTING_OUT
`, cname))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"EXEC_EXISTING_OUT": "hello-existing",
	})
}

func TestDockerExecutor_ErrorIncludesRecentStderr(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: fail
    executor:
      type: docker
      config:
        image: alpine:3
        autoRemove: true
    command: sh -c 'echo first 1>&2; echo second 1>&2; exit 7'
`)

	agent := dag.Agent()

	err := agent.Run(agent.Context)
	require.Error(t, err)
	// Should contain recent stderr from docker executor
	require.Contains(t, err.Error(), "first")
	require.Contains(t, err.Error(), "second")
}
