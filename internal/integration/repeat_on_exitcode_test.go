package integration_test

import (
	"context"
	"os"
	"path/filepath"
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

	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	dag := th.DAG(t, filepath.Join("integration", "repeat-on-exitcode-fail.yaml"))
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 15*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	dag.AssertLatestStatus(t, status.StatusSuccess)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	require.Len(t, dagRunStatus.Nodes, 1)
	nodeStatus := dagRunStatus.Nodes[0]

	assert.Equal(t, status.NodeStatusSuccess, nodeStatus.Status, "The final status of the node should be Success")
	assert.True(t, nodeStatus.Repeated, "The step should be marked as repeated")
	assert.GreaterOrEqual(t, nodeStatus.DoneCount, 3, "The step should have executed at least 3 times")
}
