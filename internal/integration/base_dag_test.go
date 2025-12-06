package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestSkipBaseHandlers_SubDAGDoesNotInheritHandlers(t *testing.T) {
	t.Parallel()

	// Create a temp directory to store base config
	tmpDir := t.TempDir()
	baseConfigPath := filepath.Join(tmpDir, "base.yaml")
	markerFile := filepath.Join(tmpDir, "marker.txt")

	// Create base DAG with handlerOn: failure that writes a marker file
	baseConfig := `handlerOn:
  failure:
    command: echo "BASE_FAILURE_HANDLER_RAN" >> ` + markerFile + `
`
	require.NoError(t, os.WriteFile(baseConfigPath, []byte(baseConfig), 0600))

	// Load a DAG WITHOUT skip base handlers - should have handler
	th := test.Setup(t)
	dagContent := `steps:
  - name: failing-step
    command: exit 1
`
	dagFile := th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "test-no-skip", []byte(dagContent))

	// Load without skip - should have failure handler from base config
	dagWithHandler, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath))
	require.NoError(t, err)
	require.NotNil(t, dagWithHandler.HandlerOn.Failure, "failure handler from base config should be set")

	// Load WITH skip base handlers - should NOT have handler
	dagWithoutHandler, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath), spec.WithSkipBaseHandlers())
	require.NoError(t, err)
	require.Nil(t, dagWithoutHandler.HandlerOn.Failure, "failure handler should NOT be inherited when skip flag is set")
}

func TestSkipBaseHandlers_ExplicitHandlersStillWork(t *testing.T) {
	t.Parallel()

	// Create a temp directory to store base config
	tmpDir := t.TempDir()
	baseConfigPath := filepath.Join(tmpDir, "base.yaml")
	baseMarkerFile := filepath.Join(tmpDir, "base_marker.txt")
	dagMarkerFile := filepath.Join(tmpDir, "dag_marker.txt")

	// Create base DAG with handlerOn: failure
	baseConfig := `handlerOn:
  failure:
    command: echo "BASE" >> ` + baseMarkerFile + `
`
	require.NoError(t, os.WriteFile(baseConfigPath, []byte(baseConfig), 0600))

	// Setup test helper
	th := test.Setup(t)

	// Create a DAG file with its own failure handler
	dagContent := `handlerOn:
  failure:
    command: echo "DAG" >> ` + dagMarkerFile + `

steps:
  - name: failing-step
    command: exit 1
`
	dagFile := th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "test-explicit-handler", []byte(dagContent))

	// Load WITH skip base handlers - should have DAG's own handler
	dag, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath), spec.WithSkipBaseHandlers())
	require.NoError(t, err)
	require.NotNil(t, dag.HandlerOn.Failure, "DAG's own failure handler should be present")

	// Run the DAG
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

	// Wait a bit for the handler file to be written
	time.Sleep(100 * time.Millisecond)

	// Verify DAG's own handler ran
	dagOutput, err := os.ReadFile(dagMarkerFile)
	require.NoError(t, err, "DAG's failure handler should have written marker file")
	require.Contains(t, string(dagOutput), "DAG", "DAG's own failure handler should have run")

	// Verify base handler did NOT run
	_, err = os.ReadFile(baseMarkerFile)
	require.True(t, os.IsNotExist(err), "base failure handler should NOT have run")
}

func TestSkipBaseHandlers_AllHandlerTypesSkipped(t *testing.T) {
	t.Parallel()

	// Create a temp directory to store base config
	tmpDir := t.TempDir()
	baseConfigPath := filepath.Join(tmpDir, "base.yaml")

	// Create base DAG with all handler types
	baseConfig := `handlerOn:
  init:
    command: "true"
  success:
    command: "true"
  failure:
    command: "true"
  abort:
    command: "true"
  exit:
    command: "true"
`
	require.NoError(t, os.WriteFile(baseConfigPath, []byte(baseConfig), 0600))

	// Setup test helper
	th := test.Setup(t)

	// Create a DAG file
	dagContent := `steps:
  - name: step1
    command: "true"
`
	dagFile := th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "test-all-handlers", []byte(dagContent))

	// Load without skip - all handlers should be set
	dagWithHandlers, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath))
	require.NoError(t, err)
	require.NotNil(t, dagWithHandlers.HandlerOn.Init, "init handler should be set")
	require.NotNil(t, dagWithHandlers.HandlerOn.Success, "success handler should be set")
	require.NotNil(t, dagWithHandlers.HandlerOn.Failure, "failure handler should be set")
	require.NotNil(t, dagWithHandlers.HandlerOn.Cancel, "abort/cancel handler should be set")
	require.NotNil(t, dagWithHandlers.HandlerOn.Exit, "exit handler should be set")

	// Load WITH skip - no handlers should be set
	dagWithoutHandlers, err := spec.Load(th.Context, dagFile, spec.WithBaseConfig(baseConfigPath), spec.WithSkipBaseHandlers())
	require.NoError(t, err)
	require.Nil(t, dagWithoutHandlers.HandlerOn.Init, "init handler should NOT be set")
	require.Nil(t, dagWithoutHandlers.HandlerOn.Success, "success handler should NOT be set")
	require.Nil(t, dagWithoutHandlers.HandlerOn.Failure, "failure handler should NOT be set")
	require.Nil(t, dagWithoutHandlers.HandlerOn.Cancel, "abort/cancel handler should NOT be set")
	require.Nil(t, dagWithoutHandlers.HandlerOn.Exit, "exit handler should NOT be set")
}
