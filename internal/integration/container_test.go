package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
)

func TestContainer(t *testing.T) {
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
container:
  image: alpine:3
steps:
  - name: s1
    command: "echo 123 abc $FOO"
    output: OUT1
`,
			expectedOutputs: map[string]any{
				"OUT1": "123 abc BAR",
			},
		},
		{
			name: "command_with_args",
			dag: `
name: test-command-with-args
container:
  image: alpine:3
steps:
  - name: s1
    command: echo hello world
    output: OUT1
`,
			expectedOutputs: map[string]any{
				"OUT1": "hello world",
			},
		},
		{
			name: "working_directory",
			dag: `
name: test-working-dir
container:
  image: alpine:3
  workDir: /tmp
steps:
  - name: s1
    command: "pwd"
    output: OUT1
`,
			expectedOutputs: map[string]any{
				"OUT1": "/tmp",
			},
		},
		{
			name: "container_with_user",
			dag: `
name: test-user
container:
  image: alpine:3
  user: "nobody"
steps:
  - name: s1
    command: "whoami"
    output: OUT1
`,
			expectedOutputs: map[string]any{
				"OUT1": "nobody",
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
