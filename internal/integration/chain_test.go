package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestChainExecution(t *testing.T) {
	th := test.Setup(t)

	// Load chain DAG
	dag := th.DAG(t, `type: chain
steps:
  - name: step1
    command: echo "step 1"
  - name: step2
    command: echo "step 2"
  - name: step3
    command: echo "step 3"
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify execution order
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 3)

	// Verify steps ran in order (chain type adds implicit dependencies)
	require.Equal(t, "step1", dagRunStatus.Nodes[0].Step.Name)
	require.Equal(t, "step2", dagRunStatus.Nodes[1].Step.Name)
	require.Equal(t, "step3", dagRunStatus.Nodes[2].Step.Name)
}
