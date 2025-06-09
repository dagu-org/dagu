package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestChainExecution(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load chain DAG
	dag := th.DAG(t, filepath.Join("integration", "chain.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify execution order
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 3)

	// Verify steps ran in order (chain type adds implicit dependencies)
	require.Equal(t, "step1", status.Nodes[0].Step.Name)
	require.Equal(t, "step2", status.Nodes[1].Step.Name)
	require.Equal(t, "step3", status.Nodes[2].Step.Name)
}