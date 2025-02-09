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
			name: "issue-810",
			dag:  "issue-810.yaml",
			expectedOutputs: map[string]string{
				"OUT1": "start",
				"OUT2": "foo",
				"OUT3": "bar",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			th := test.Setup(t)

			dag := th.DAG(t, filepath.Join("integration", tc.dag))
			agent := dag.Agent()

			agent.RunSuccess(t)

			dag.AssertLatestStatus(t, scheduler.StatusSuccess)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}
