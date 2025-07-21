package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLargeOutput_64KB(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG that reads a 64KB file
	dag := th.DAG(t, filepath.Join("integration", "large-output-64kb.yaml"))
	agent := dag.Agent()

	// Run with timeout to detect hanging
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete without hanging")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify output capture
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "read-64kb-file", dagRunStatus.Nodes[0].Step.Name)
}

func TestLargeOutput_65KB(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG that reads a 65KB file (exceeds typical pipe buffer)
	dag := th.DAG(t, filepath.Join("integration", "large-output-65kb.yaml"))
	agent := dag.Agent()

	// Run with timeout to detect hanging
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete without hanging when output exceeds 64KB")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "read-65kb-file", dagRunStatus.Nodes[0].Step.Name)
}

func TestLargeOutput_128KB(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG that reads a 128KB file
	dag := th.DAG(t, filepath.Join("integration", "large-output-128kb.yaml"))
	agent := dag.Agent()

	// Run with timeout to detect hanging
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete without hanging with large output")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "read-128kb-file", dagRunStatus.Nodes[0].Step.Name)
}
