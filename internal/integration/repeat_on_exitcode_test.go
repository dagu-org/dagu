package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatPolicy_OnExitCode(t *testing.T) {
	counterFile := "/tmp/dagu-test-counter-repeat-on-exitcode"
	_ = os.Remove(counterFile)
	t.Cleanup(func() {
		_ = os.Remove(counterFile)
	})

	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - name: repeat-on-fail
    command: |
      #!/bin/bash
      COUNTER_FILE="/tmp/dagu-test-counter-repeat-on-exitcode"
      if [ ! -f "$COUNTER_FILE" ]; then
          echo 1 > "$COUNTER_FILE"
          exit 1
      fi

      count=$(cat "$COUNTER_FILE")
      if [ "$count" -lt 3 ]; then
          echo $((count + 1)) > "$COUNTER_FILE"
          exit 1
      else
          echo $((count + 1)) > "$COUNTER_FILE"
          exit 0
      fi
    repeatPolicy:
      exitCode: [1]
      limit: 5
      intervalSec: 1
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 15*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	dag.AssertLatestStatus(t, status.Success)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	require.Len(t, dagRunStatus.Nodes, 1)
	nodeStatus := dagRunStatus.Nodes[0]

	assert.Equal(t, status.NodeSuccess, nodeStatus.Status, "The final status of the node should be Success")
	assert.True(t, nodeStatus.Repeated, "The step should be marked as repeated")
	assert.GreaterOrEqual(t, nodeStatus.DoneCount, 3, "The step should have executed at least 3 times")
}
