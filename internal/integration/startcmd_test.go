package integration_test

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServer_StartWithConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setupFunc  func(t *testing.T) (string, string) // returns configFile and tempDir
		dagPath    func(t *testing.T, tempDir string) string
		envVarName string
	}{
		{
			name: "GlobalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				configFile := filepath.Join(tempDir, "config.yaml")
				configContent := `logDir: ${TMP_LOGS_DIR}/logs`
				require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0600))
				return configFile, tempDir
			},
			dagPath: func(t *testing.T, _ string) string {
				return test.TestdataPath(t, path.Join("integration", "basic.yaml"))
			},
			envVarName: "TMP_LOGS_DIR",
		},
		{
			name: "DAGLocalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				dagFile := filepath.Join(tempDir, "basic.yaml")
				dagContent := `
logDir: ${DAG_TMP_LOGS_DIR}/logs
steps:
  - name: step1
    command: echo "Hello, world!"
`
				require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0600))
				return dagFile, tempDir
			},
			dagPath: func(_ *testing.T, tempDir string) string {
				return filepath.Join(tempDir, "basic.yaml")
			},
			envVarName: "DAG_TMP_LOGS_DIR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test case
			configFile, tempDir := tc.setupFunc(t)
			_ = os.Setenv(tc.envVarName, tempDir)

			// Get DAG path
			dagPath := tc.dagPath(t, tempDir)

			// Run command
			th := test.SetupCommand(t)
			args := []string{"start"}
			if tc.name == "GlobalLogDir" {
				args = append(args, "--config", configFile)
			}
			args = append(args, dagPath)

			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
				Args:        args,
				ExpectedOut: []string{"DAG run finished"},
			})

			// Verify log directory and files
			verifyLogs(t, tempDir)
		})
	}
}

func TestServer_RetrySubDAG(t *testing.T) {
	// Get DAG path
	th := test.SetupCommand(t)

	createDAGFile := func(name, content string) {
		// Create temporary DAG file
		dagFile := filepath.Join(th.Config.Paths.DAGsDir, name)
		// Create the directory if it doesn't exist
		err := os.MkdirAll(filepath.Dir(dagFile), 0750)
		require.NoError(t, err)
		// Write the DAG file
		err = os.WriteFile(dagFile, []byte(content), 0600)
		require.NoError(t, err)
	}

	createDAGFile("parent.yaml", `
steps:
  - name: parent
    run: child_1
    params: "PARAM=FOO"
`)

	createDAGFile("child_1.yaml", `
params: "PARAM=BAR"
steps:
  - name: child_2
    run: child_2
    params: "PARAM=$PARAM"
`)

	createDAGFile("child_2.yaml", `
params: "PARAM=BAZ"
steps:
  - name: child_2
    command: echo "Hello, $PARAM"
`)

	requestID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--request-id", requestID, "parent"}
	th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Update the child_2 status to "failed" to simulate a retry
	// First, find the child_2 request ID to update its status
	ctx := context.Background()
	parentRec, err := th.HistoryStore.FindByRequestID(ctx, "parent", requestID)
	require.NoError(t, err)

	updateStatus := func(rec persistence.Record, run *persistence.Run) {
		err = rec.Open(ctx)
		require.NoError(t, err)
		err = rec.Write(ctx, run.Status)
		require.NoError(t, err)
		err = rec.Close(ctx)
		require.NoError(t, err)
	}

	// (1) Find the child_1 node and update its status to "failed"
	parentRun, err := parentRec.ReadRun(ctx)
	require.NoError(t, err)

	child1Node := parentRun.Status.Nodes[0]
	child1Node.Status = scheduler.NodeStatusError
	updateStatus(parentRec, parentRun)

	// (2) Find the history record for child_1
	rootDAG := digraph.NewRootDAG("parent", requestID)
	child1Rec, err := th.HistoryStore.FindBySubRequestID(ctx, child1Node.SubRuns[0].RequestID, rootDAG)
	require.NoError(t, err)

	child1Run, err := child1Rec.ReadRun(ctx)
	require.NoError(t, err)

	// (3) Find the child_2 node and update its status to "failed"
	child2Node := child1Run.Status.Nodes[0]
	child2Node.Status = scheduler.NodeStatusError
	updateStatus(child1Rec, child1Run)

	// (4) Find the history record for child_2
	child2Rec, err := th.HistoryStore.FindBySubRequestID(ctx, child2Node.SubRuns[0].RequestID, rootDAG)
	require.NoError(t, err)

	child2Run, err := child2Rec.ReadRun(ctx)
	require.NoError(t, err)

	require.Equal(t, child2Run.Status.Status.String(), scheduler.NodeStatusSuccess.String())

	// (5) Update the step in child_2 to "failed" to simulate a retry
	child2Run.Status.Nodes[0].Status = scheduler.NodeStatusError
	updateStatus(child2Rec, child2Run)

	// (6) Check if the child_2 status is now "failed"
	child2Run, err = child2Rec.ReadRun(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Run.Status.Nodes[0].Status.String(), scheduler.NodeStatusError.String())

	// Retry the DAG

	args = []string{"retry", "--request-id", requestID, "parent"}
	th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Check if the child_2 status is now "success"
	child2Rec, err = th.HistoryStore.FindBySubRequestID(ctx, child2Node.SubRuns[0].RequestID, rootDAG)
	require.NoError(t, err)
	child2Run, err = child2Rec.ReadRun(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Run.Status.Nodes[0].Status.String(), scheduler.NodeStatusSuccess.String())
}

// verifyLogs checks if the expected log directory and files exist
func verifyLogs(t *testing.T, tempDir string) {
	// Check if the logs directory was created
	_, err := os.Stat(tempDir + "/logs/basic")
	require.NoError(t, err)

	// Check if the log file was created with the expected pattern
	files, err := os.ReadDir(tempDir + "/logs/basic")
	require.NoError(t, err)

	// Look for a log file that matches the expected pattern
	logFileFound := false
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "scheduler_basic.") && strings.HasSuffix(file.Name(), ".log") {
			logFileFound = true
			break
		}
	}
	require.True(t, logFileFound, "No log file found with expected naming pattern")
}
