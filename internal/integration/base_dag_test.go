package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	runtimepkg "github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBaseDAGSpecialEnvVarsInHandler(t *testing.T) {
	t.Parallel()

	// Create a temp directory to store base config and output files
	tmpDir := t.TempDir()
	baseConfigPath := filepath.Join(tmpDir, "base.yaml")
	outputFile := filepath.Join(tmpDir, "handler_output.txt")

	// Create base DAG with handlerOn: failure that captures special env vars
	baseConfig := `handlerOn:
  failure:
    command: |
      echo "DAG_NAME=${DAG_NAME}" >> ` + outputFile + `
      echo "DAG_RUN_ID=${DAG_RUN_ID}" >> ` + outputFile + `
      echo "DAG_RUN_LOG_FILE=${DAG_RUN_LOG_FILE}" >> ` + outputFile + `
`
	require.NoError(t, os.WriteFile(baseConfigPath, []byte(baseConfig), 0600))

	// Setup test helper
	th := test.Setup(t)

	// Create a DAG file that will fail
	dagContent := `steps:
  - name: failing-step
    command: exit 1
`
	dagFile := th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "test-base-env", []byte(dagContent))

	// Load the DAG with base config
	dag, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath))
	require.NoError(t, err)

	// Verify base config was applied - handlerOn should be set
	require.NotNil(t, dag.HandlerOn.Failure, "failure handler from base config should be set")

	// Create agent and run
	dagRunID := uuid.New().String()
	logDir := th.Config.Paths.LogDir
	logFile := filepath.Join(logDir, dagRunID+".log")
	root := execution.NewDAGRunRef(dag.Name, dagRunID)

	drm := runtimepkg.NewManager(th.DAGRunStore, th.ProcStore, th.Config)

	a := agent.New(
		dagRunID,
		dag,
		logDir,
		logFile,
		drm,
		th.DAGStore,
		th.DAGRunStore,
		th.ServiceRegistry,
		root,
		th.Config.Global.Peer,
		agent.Options{},
	)

	// Run the agent - expect failure
	err = a.Run(th.Context)
	require.Error(t, err)

	// Verify the DAG failed
	status := a.Status(th.Context)
	require.Equal(t, core.Failed, status.Status)

	// Read the output file and verify special env vars were available
	output, err := os.ReadFile(outputFile)
	require.NoError(t, err, "handler output file should exist")

	outputStr := string(output)
	require.Contains(t, outputStr, "DAG_NAME=", "DAG_NAME should be set")
	require.Contains(t, outputStr, "DAG_RUN_ID=", "DAG_RUN_ID should be set")
	require.Contains(t, outputStr, "DAG_RUN_LOG_FILE=", "DAG_RUN_LOG_FILE should be set")

	// Verify the values are not empty
	require.NotContains(t, outputStr, "DAG_NAME=\n", "DAG_NAME should not be empty")
	require.NotContains(t, outputStr, "DAG_RUN_ID=\n", "DAG_RUN_ID should not be empty")
	require.NotContains(t, outputStr, "DAG_RUN_LOG_FILE=\n", "DAG_RUN_LOG_FILE should not be empty")
}
