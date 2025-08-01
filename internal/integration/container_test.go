package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
)

func TestContainer(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	testCases := []struct {
		name            string
		dag             string
		expectedOutputs map[string]any
	}{
		{
			name: "basic",
			dag: `
container:
  image: alpine:3
steps:
  - name: s1
    command: "echo 123 abc"
    output: OUT1
`,
			expectedOutputs: map[string]any{
				"OUT1": "123 abc",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dag := th.DAG(t, tc.dag)
			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, status.Success)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}
