package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupWithNameField verifies that the history cleanup mechanism works correctly
// when a DAG has a 'name:' field that differs from the filename.
//
// This test ensures that:
// 1. DAG runs are stored using the name: field value (not the filename)
// 2. The cleanup targets the correct directory based on the DAG name
func TestCleanupWithNameField(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Create a DAG file where:
	// - filename: "my-workflow.yaml"
	// - name: field: "CustomWorkflowName"
	// This tests that cleanup works when name differs from filename
	dagContent := `name: CustomWorkflowName
histRetentionDays: 1
steps:
  - name: test-step
    command: echo "hello"
`
	dagFile := th.CreateDAGFile(t, "my-workflow.yaml", dagContent)

	// Load the DAG to verify the name is set correctly
	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)
	require.Equal(t, "CustomWorkflowName", dag.Name, "DAG name should come from name: field, not filename")

	// Run the DAG multiple times to create history
	for i := 0; i < 3; i++ {
		dagRunID := uuid.Must(uuid.NewV7()).String()
		args := []string{"start", "--run-id", dagRunID, "my-workflow"}
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})
	}

	// Verify we have 3 runs stored under the correct name (CustomWorkflowName)
	runs := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.Len(t, runs, 3, "should have 3 DAG runs stored under 'CustomWorkflowName'")

	// Verify runs are NOT stored under the filename (my-workflow)
	runsUnderFilename := th.DAGRunMgr.ListRecentStatus(th.Context, "my-workflow", 10)
	assert.Empty(t, runsUnderFilename, "should have no runs stored under filename 'my-workflow'")
}

// TestCleanupRemovesOldRuns verifies that old DAG runs are cleaned up
// when histRetentionDays is set and a new run is executed.
//
// This test uses a DAG with:
// - filename: cleanup-test.yaml
// - name: CleanupTestDAG (different from filename)
// - histRetentionDays: 1
//
// It creates an old run with a backdated file modification time
// and verifies that running the DAG again triggers cleanup.
//
// Note: This test uses os.Chtimes which may have issues on Windows.
func TestCleanupRemovesOldRuns(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Create a DAG where name differs from filename
	// histRetentionDays: 1 means only keep runs from the last 1 day
	dagContent := `name: CleanupTestDAG
histRetentionDays: 1
steps:
  - name: test-step
    command: echo "hello"
`
	dagFile := th.CreateDAGFile(t, "cleanup-test.yaml", dagContent)

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)
	require.Equal(t, "CleanupTestDAG", dag.Name, "DAG name should come from name: field")

	// Create an old DAG run manually (simulating a run from 5 days ago)
	oldTimestamp := time.Now().AddDate(0, 0, -5)
	oldRunID := uuid.Must(uuid.NewV7()).String()

	oldAttempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, oldTimestamp, oldRunID, execution.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	// Write a completed status so it can be cleaned up
	require.NoError(t, oldAttempt.Open(th.Context))
	oldStatus := createCompletedStatus(dag, oldRunID, oldTimestamp)
	require.NoError(t, oldAttempt.Write(th.Context, *oldStatus))
	require.NoError(t, oldAttempt.Close(th.Context))

	// Backdate the status file to simulate an old run
	// The cleanup uses file modification time, not the timestamp in the status
	statusFile := findStatusFile(t, th.Config.Paths.DAGRunsDir, dag.Name, oldRunID)
	require.NotEmpty(t, statusFile, "should find status file for the old run")
	require.NoError(t, os.Chtimes(statusFile, oldTimestamp, oldTimestamp))

	// Verify the old run exists under the DAG name (not filename)
	runsBeforeCleanup := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.Len(t, runsBeforeCleanup, 1, "should have 1 old run before cleanup")

	// Now run the DAG - this should trigger cleanup of old runs
	newRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", newRunID, "cleanup-test"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// After cleanup, only runs from the last 1 day should remain
	// The old run (5 days ago) should be removed, only the new run should remain
	runsAfterCleanup := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.Len(t, runsAfterCleanup, 1, "should have only 1 run after cleanup (the new one)")
	assert.Equal(t, newRunID, runsAfterCleanup[0].DAGRunID, "remaining run should be the new run")
}

// TestCleanupWithDifferentNameAndFilename specifically tests the scenario
// where the DAG name field differs from the filename, ensuring cleanup
// correctly identifies and removes old runs.
//
// This test uses:
// - filename: different-filename.yaml
// - name: ActualDAGName
// - histRetentionDays: 1
//
// It creates an old run with a backdated file modification time
// and verifies cleanup removes it.
func TestCleanupWithDifferentNameAndFilename(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Create a DAG where name differs from filename
	dagContent := `name: ActualDAGName
histRetentionDays: 1
steps:
  - name: step1
    command: echo "test"
`
	dagFile := th.CreateDAGFile(t, "different-filename.yaml", dagContent)

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)
	require.Equal(t, "ActualDAGName", dag.Name)

	// Create an old run from 10 days ago
	oldTimestamp := time.Now().AddDate(0, 0, -10)
	oldRunID := uuid.Must(uuid.NewV7()).String()

	oldAttempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, oldTimestamp, oldRunID, execution.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	require.NoError(t, oldAttempt.Open(th.Context))
	oldStatus := createCompletedStatus(dag, oldRunID, oldTimestamp)
	require.NoError(t, oldAttempt.Write(th.Context, *oldStatus))
	require.NoError(t, oldAttempt.Close(th.Context))

	// Backdate the status file to simulate an old run
	statusFile := findStatusFile(t, th.Config.Paths.DAGRunsDir, dag.Name, oldRunID)
	require.NotEmpty(t, statusFile, "should find status file for the old run")
	require.NoError(t, os.Chtimes(statusFile, oldTimestamp, oldTimestamp))

	// Verify old run exists under the correct name (ActualDAGName, not filename)
	runs := th.DAGRunMgr.ListRecentStatus(th.Context, "ActualDAGName", 10)
	require.Len(t, runs, 1, "should have 1 old run under 'ActualDAGName'")

	// Verify no runs under the filename
	runsUnderFilename := th.DAGRunMgr.ListRecentStatus(th.Context, "different-filename", 10)
	assert.Empty(t, runsUnderFilename, "should have no runs under filename")

	// Execute new run - should trigger cleanup
	newRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", newRunID, "different-filename"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Verify cleanup worked - old run should be removed
	runsAfter := th.DAGRunMgr.ListRecentStatus(th.Context, "ActualDAGName", 10)
	require.Len(t, runsAfter, 1, "should have only 1 run after cleanup")
	assert.Equal(t, newRunID, runsAfter[0].DAGRunID)
}

// TestCleanupDataDirectoryStructure verifies that DAG runs are stored
// in the correct directory structure based on the DAG name field.
func TestCleanupDataDirectoryStructure(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Create a DAG with a specific name
	dagContent := `name: MySpecialDAG
histRetentionDays: 30
steps:
  - name: test
    command: echo "hello"
`
	dagFile := th.CreateDAGFile(t, "special-dag-file.yaml", dagContent)

	_, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	// Run the DAG
	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "special-dag-file"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Verify the data is stored under the DAG name directory, not the filename
	dagRunsBaseDir := th.Config.Paths.DAGRunsDir

	// Check that directory exists for the DAG name
	dagNameDir := filepath.Join(dagRunsBaseDir, "MySpecialDAG")
	_, err = os.Stat(dagNameDir)
	assert.NoError(t, err, "directory for DAG name 'MySpecialDAG' should exist")

	// Check that directory does NOT exist for the filename
	filenameDir := filepath.Join(dagRunsBaseDir, "special-dag-file")
	_, err = os.Stat(filenameDir)
	assert.True(t, os.IsNotExist(err), "directory for filename 'special-dag-file' should NOT exist")
}

// TestCleanupWorksWhenNameMatchesFilename verifies that cleanup works
// when the DAG name matches the filename (baseline test).
func TestCleanupWorksWhenNameMatchesFilename(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Create a DAG where name matches filename (no name: field)
	// This should work correctly as a baseline
	dagContent := `histRetentionDays: 1
steps:
  - name: test-step
    command: echo "hello"
`
	dagFile := th.CreateDAGFile(t, "baseline-test.yaml", dagContent)

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)
	// When no name: field, the name should come from filename
	require.Equal(t, "baseline-test", dag.Name, "DAG name should come from filename when no name: field")

	// Create an old run from 5 days ago
	oldTimestamp := time.Now().AddDate(0, 0, -5)
	oldRunID := uuid.Must(uuid.NewV7()).String()

	oldAttempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, oldTimestamp, oldRunID, execution.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	require.NoError(t, oldAttempt.Open(th.Context))
	oldStatus := createCompletedStatus(dag, oldRunID, oldTimestamp)
	require.NoError(t, oldAttempt.Write(th.Context, *oldStatus))
	require.NoError(t, oldAttempt.Close(th.Context))

	// Backdate the status file to simulate an old run
	statusFile := findStatusFile(t, th.Config.Paths.DAGRunsDir, dag.Name, oldRunID)
	require.NotEmpty(t, statusFile, "should find status file for the old run")
	require.NoError(t, os.Chtimes(statusFile, oldTimestamp, oldTimestamp))

	// Verify the old run exists
	runsBeforeCleanup := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.Len(t, runsBeforeCleanup, 1, "should have 1 old run before cleanup")

	// Execute new run - should trigger cleanup
	newRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", newRunID, "baseline-test"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Verify cleanup worked - old run should be removed
	runsAfter := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.Len(t, runsAfter, 1, "should have only 1 run after cleanup")
	assert.Equal(t, newRunID, runsAfter[0].DAGRunID, "remaining run should be the new run")
}

// findStatusFile finds the status.jsonl file for a given DAG run
func findStatusFile(t *testing.T, dagRunsDir, dagName, dagRunID string) string {
	t.Helper()

	var statusFile string
	dagDir := filepath.Join(dagRunsDir, dagName, "dag-runs")

	err := filepath.Walk(dagDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "status.jsonl" {
			// The directory structure is: dag-runs/YYYY/MM/DD/dag-run_..._RUNID/attempt_.../status.jsonl
			// Check if the dag-run directory contains the runID
			dagRunDir := filepath.Dir(filepath.Dir(path))
			dirBase := filepath.Base(dagRunDir)
			if len(dirBase) > 8 && dirBase[:8] == "dag-run_" {
				if containsRunID(dirBase, dagRunID) {
					statusFile = path
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return statusFile
}

// containsRunID checks if a directory name contains the run ID
func containsRunID(dirName, runID string) bool {
	// Directory format: dag-run_YYYYMMDD_HHMMSSZ_RUNID
	return len(dirName) > len(runID) && dirName[len(dirName)-len(runID):] == runID
}

// createCompletedStatus creates a minimal completed DAGRunStatus for testing
func createCompletedStatus(dag *core.DAG, dagRunID string, timestamp time.Time) *execution.DAGRunStatus {
	return &execution.DAGRunStatus{
		Name:      dag.Name,
		Status:    core.Succeeded,
		DAGRunID:  dagRunID,
		StartedAt: timestamp.Format(time.RFC3339),
		Log:       "",
	}
}
