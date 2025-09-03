package integration_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
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
