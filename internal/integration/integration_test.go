package integration

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestIntegration(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		dag             string
		expectedOutputs map[string]string
	}

	testCases := []testCase{
		{
			name: "Depends",
			dag:  "depends.yaml",
		},
		{
			name: "Pipe",
			dag:  "pipe.yaml",
			expectedOutputs: map[string]string{
				"OUT1": "hello foo",
			},
		},
		{
			name: "NamedParams",
			dag:  "named-params.yaml",
			expectedOutputs: map[string]string{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "NamedParamsList",
			dag:  "named-params-list.yaml",
			expectedOutputs: map[string]string{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "issue-810",
			dag:  "issue-810.yaml",
			expectedOutputs: map[string]string{
				"OUT1": "start",
				"OUT2": "foo",
				"OUT3": "bar",
				"OUT4": "baz",
			},
		},
	}

	th := test.Setup(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dag := th.DAG(t, filepath.Join("integration", tc.dag))
			agent := dag.Agent()

			agent.RunSuccess(t)

			dag.AssertLatestStatus(t, scheduler.StatusSuccess)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}
