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
		name string
		dag  string
	}

	testCases := []testCase{
		{
			name: "Depends",
			dag:  "depends.yaml",
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
		})
	}
}
