package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLargeOutput_128KB(t *testing.T) {
	th := test.Setup(t)

	// Load DAG that reads a 128KB file
	textFilePath := test.TestdataPath(t, "integration/large-output-128kb.txt")
	dag := th.DAG(t, `steps:
  - name: read-128kb-file
    command: cat `+textFilePath+`
    output: OUTPUT_128KB
`)
	agent := dag.Agent()

	// Run with timeout to detect hanging
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete without hanging with large output")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "read-128kb-file", dagRunStatus.Nodes[0].Step.Name)
}

func TestResult_StringType(t *testing.T) {
	th := test.Setup(t)

	dag := th.DAG(t, `
result: "${OUTPUT_VAR}"
steps:
  - name: step1
    command: echo hello
    output: OUTPUT_VAR
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus.Result)
	assert.Equal(t, "hello", dagRunStatus.Result.Value)
	assert.Equal(t, core.ResultTypeString, dagRunStatus.Result.Type)
}

func TestResult_ObjectType(t *testing.T) {
	th := test.Setup(t)

	dag := th.DAG(t, `
result:
  count: "${COUNT}"
  status: "done"
steps:
  - name: step1
    command: echo 42
    output: COUNT
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus.Result)
	assert.Equal(t, `{"count":"42","status":"done"}`, dagRunStatus.Result.Value)
	assert.Equal(t, core.ResultTypeObject, dagRunStatus.Result.Type)
}

func TestResult_NotDefined(t *testing.T) {
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: step1
    command: echo hello
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	assert.Nil(t, dagRunStatus.Result)
}
