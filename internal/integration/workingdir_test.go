package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkingDirectoryResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	th := test.SetupCommand(t)

	// Create directory structure for testing
	dagsDir := th.Config.Paths.DAGsDir
	parentScripts := filepath.Join(dagsDir, "parent_scripts")
	childScripts := filepath.Join(dagsDir, "child_scripts")
	require.NoError(t, os.MkdirAll(parentScripts, 0755))
	require.NoError(t, os.MkdirAll(childScripts, 0755))

	// Parent DAG: tests relative workingDir and step relative dir
	th.CreateDAGFile(t, "parent_wd.yaml", `
workingDir: ./parent_scripts
steps:
  - name: parent_pwd
    command: pwd
    output: PARENT_DIR
  - name: parent_relative_step
    dir: ../child_scripts
    command: pwd
    output: PARENT_STEP_DIR
  - name: call_child
    call: child_wd
`)

	// Child DAG: tests subDAG has its own workingDir context
	th.CreateDAGFile(t, "child_wd.yaml", `
workingDir: ./child_scripts
steps:
  - name: child_pwd
    command: pwd
    output: CHILD_DIR
  - name: child_relative_step
    dir: ../parent_scripts
    command: pwd
    output: CHILD_STEP_DIR
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "parent_wd"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Verify results
	ctx := context.Background()
	ref := execution.NewDAGRunRef("parent_wd", dagRunID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	parentStatus, err := parentAttempt.ReadStatus(ctx)
	require.NoError(t, err)

	// Parent DAG workingDir resolved relative to DAG file location
	parentPwdNode := parentStatus.Nodes[0]
	require.Equal(t, parentScripts, parentPwdNode.OutputVariables.Variables()["PARENT_DIR"])

	// Parent step relative dir resolved against parent's workingDir
	parentStepNode := parentStatus.Nodes[1]
	require.Equal(t, childScripts, parentStepNode.OutputVariables.Variables()["PARENT_STEP_DIR"])

	// Verify child subDAG
	callChildNode := parentStatus.Nodes[2]
	childAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, callChildNode.SubRuns[0].DAGRunID)
	require.NoError(t, err)

	childStatus, err := childAttempt.ReadStatus(ctx)
	require.NoError(t, err)

	// Child DAG workingDir resolved relative to child DAG file location
	childPwdNode := childStatus.Nodes[0]
	require.Equal(t, childScripts, childPwdNode.OutputVariables.Variables()["CHILD_DIR"])

	// Child step relative dir resolved against child's workingDir (not parent's)
	childStepNode := childStatus.Nodes[1]
	require.Equal(t, parentScripts, childStepNode.OutputVariables.Variables()["CHILD_STEP_DIR"])
}
