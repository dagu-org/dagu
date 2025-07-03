package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestJQExecutor(t *testing.T) {
	t.Parallel()

	t.Run("MultipleOutputsWithRawTrue", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq-raw-multiple.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("MultipleOutputsWithRawFalse", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq-json-multiple.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("StringOutputWithRawTrue", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq-raw-strings.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "hello\nworld",
		})
	})

	t.Run("StringOutputWithRawFalse", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq-json-strings.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "\"hello\"\n\"world\"",
		})
	})

	t.Run("TSVOutputWithRawTrue", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq-raw-tsv.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\t100\n2\t200\n3\t300",
		})
	})
}
