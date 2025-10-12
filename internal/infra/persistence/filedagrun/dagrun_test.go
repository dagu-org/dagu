package filedagrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGRun(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-id-1", execution.NewUTC(time.Now()))

		ts1 := execution.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := execution.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := execution.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))

		_ = run.WriteStatus(t, ts1, status.Running)
		_ = run.WriteStatus(t, ts2, status.Success)
		_ = run.WriteStatus(t, ts3, status.Error)

		latestRun, err := run.LatestAttempt(run.Context, nil)
		require.NoError(t, err)

		dagRunStatus, err := latestRun.ReadStatus(run.Context)
		require.NoError(t, err)

		require.Equal(t, status.Error.String(), dagRunStatus.Status.String())
	})
}

type DAGRunTest struct {
	DataRootTest
	*DAGRun
	TB testing.TB
}

func (dr DAGRunTest) WriteStatus(t *testing.T, ts execution.TimeInUTC, s status.Status) *Attempt {
	t.Helper()

	dag := &core.DAG{Name: "test-dag"}
	dagRunStatus := execution.InitialStatus(dag)
	dagRunStatus.DAGRunID = "test-id-1"
	dagRunStatus.Status = s

	run, err := dr.CreateAttempt(dr.Context, ts, nil)
	require.NoError(t, err)
	err = run.Open(dr.Context)
	require.NoError(t, err)

	defer func() {
		_ = run.Close(dr.Context)
	}()

	err = run.Write(dr.Context, dagRunStatus)
	require.NoError(t, err)

	return run
}

func TestListChildDAGRuns(t *testing.T) {
	t.Run("NoChildDAGRuns", func(t *testing.T) {
		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-dag-run", execution.NewUTC(time.Now()))

		children, err := run.ListChildDAGRuns(run.Context)
		require.NoError(t, err)
		assert.Empty(t, children, "should return empty list when no child dag-run exist")
	})

	t.Run("WithChildDAGRuns", func(t *testing.T) {
		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "parent-dag-run", execution.NewUTC(time.Now()))

		// Create child dag-run directory and some child dag-run directories
		childDir := filepath.Join(run.baseDir, ChildDAGRunsDir)
		require.NoError(t, os.MkdirAll(childDir, 0750))

		// Create two child dag-run directories
		child1Dir := filepath.Join(childDir, ChildDAGRunDirPrefix+"child1")
		child2Dir := filepath.Join(childDir, ChildDAGRunDirPrefix+"child2")
		require.NoError(t, os.MkdirAll(child1Dir, 0750))
		require.NoError(t, os.MkdirAll(child2Dir, 0750))

		// Create a non-directory file (should be ignored)
		nonDirFile := filepath.Join(childDir, "not-a-directory.txt")
		require.NoError(t, os.WriteFile(nonDirFile, []byte("test"), 0600))

		children, err := run.ListChildDAGRuns(run.Context)
		require.NoError(t, err)
		assert.Len(t, children, 2, "should return two child dag-runs")

		// Verify child dag-run directories
		childIDs := make([]string, len(children))
		for i, child := range children {
			childIDs[i] = child.dagRunID
		}
		assert.Contains(t, childIDs, "child1")
		assert.Contains(t, childIDs, "child2")
	})
}

func TestListLogFiles(t *testing.T) {
	t.Run("WithLogFiles", func(t *testing.T) {
		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-dag-run", execution.NewUTC(time.Now()))

		// Create a run with log files
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "test-dag-run"
		dagRunStatus.Status = status.Success
		dagRunStatus.Log = "/tmp/test.log"
		dagRunStatus.Nodes = []*execution.Node{
			{
				Step:   core.Step{Name: "step1"},
				Stdout: "/tmp/step1.out",
				Stderr: "/tmp/step1.err",
			},
			{
				Step:   core.Step{Name: "step2"},
				Stdout: "/tmp/step2.out",
				Stderr: "/tmp/step2.err",
			},
		}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		logFiles, err := run.listLogFiles(run.Context)
		require.NoError(t, err)

		expectedFiles := []string{
			"/tmp/test.log",
			"/tmp/step1.out", "/tmp/step1.err",
			"/tmp/step2.out", "/tmp/step2.err",
		}

		assert.Len(t, logFiles, len(expectedFiles), "should return all log files")
		for _, expectedFile := range expectedFiles {
			assert.Contains(t, logFiles, expectedFile, "should contain expected log file: %s", expectedFile)
		}
	})
}

func TestRemoveLogFiles(t *testing.T) {
	t.Run("RemoveMainDAGRunLogFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files
		logFiles := []string{
			filepath.Join(tmpDir, "main.log"),
			filepath.Join(tmpDir, "step1.out"),
			filepath.Join(tmpDir, "step1.err"),
		}

		for _, logFile := range logFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0600))
		}

		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-dag-run", execution.NewUTC(time.Now()))

		// Create a run with log files pointing to our test files
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "test-dag-run"
		dagRunStatus.Status = status.Success
		dagRunStatus.Log = logFiles[0]
		dagRunStatus.Nodes = []*execution.Node{
			{
				Step:   core.Step{Name: "step1"},
				Stdout: logFiles[1],
				Stderr: logFiles[2],
			},
		}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		// Verify files exist before removal
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove log files
		err = run.removeLogFiles(run.Context)
		require.NoError(t, err)

		// Verify files are removed
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveChildDAGRunLogFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files for parent and child
		parentLogFiles := []string{
			filepath.Join(tmpDir, "parent.log"),
			filepath.Join(tmpDir, "parent_step.out"),
		}
		childLogFiles := []string{
			filepath.Join(tmpDir, "child.log"),
			filepath.Join(tmpDir, "child_step.out"),
		}

		allLogFiles := append(parentLogFiles, childLogFiles...)
		for _, logFile := range allLogFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0600))
		}

		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "parent-dag-run", execution.NewUTC(time.Now()))

		// Create parent dag-run with log files
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "parent-dag-run"
		dagRunStatus.Log = parentLogFiles[0]
		dagRunStatus.Nodes = []*execution.Node{{
			Step:   core.Step{Name: "parent-step"},
			Stdout: parentLogFiles[1],
		}}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		// Create child dag-run directory
		childDir := filepath.Join(run.baseDir, ChildDAGRunsDir)
		require.NoError(t, os.MkdirAll(childDir, 0750))

		childDAGRunDir := filepath.Join(childDir, ChildDAGRunDirPrefix+"child1")
		require.NoError(t, os.MkdirAll(childDAGRunDir, 0750))

		childDAGRun, err := NewDAGRun(childDAGRunDir)
		require.NoError(t, err)

		// Create child run with log files
		childStatus := execution.InitialStatus(dag)
		childStatus.DAGRunID = "child1"
		childStatus.Log = childLogFiles[0]
		childStatus.Nodes = []*execution.Node{{
			Step:   core.Step{Name: "child-step"},
			Stdout: childLogFiles[1],
		}}

		childRun, err := childDAGRun.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, childRun.Open(run.Context))
		require.NoError(t, childRun.Write(run.Context, childStatus))
		require.NoError(t, childRun.Close(run.Context))

		// Verify all files exist before removal
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove log files (should remove both parent and child log files)
		err = run.removeLogFiles(run.Context)
		require.NoError(t, err)

		// Verify all files are removed
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})
}

func TestDAGRunRemove(t *testing.T) {
	t.Run("RemoveDAGRunWithLogFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files
		logFiles := []string{
			filepath.Join(tmpDir, "dag-run.log"),
			filepath.Join(tmpDir, "step1.out"),
			filepath.Join(tmpDir, "step1.err"),
		}

		for _, logFile := range logFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0600))
		}

		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-dag-run", execution.NewUTC(time.Now()))

		// Create a run with log files
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "test-dag-run"
		dagRunStatus.Status = status.Success
		dagRunStatus.Log = logFiles[0]
		dagRunStatus.Nodes = []*execution.Node{
			{
				Step:   core.Step{Name: "step1"},
				Stdout: logFiles[1],
				Stderr: logFiles[2],
			},
		}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		// Verify dag-run directory and log files exist
		_, err = os.Stat(run.baseDir)
		require.NoError(t, err, "dag-run directory should exist")

		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist: %s", logFile)
		}

		// Remove the dag-run
		err = run.Remove(run.Context)
		require.NoError(t, err)

		// Verify the dag-run directory is removed
		_, err = os.Stat(run.baseDir)
		assert.True(t, os.IsNotExist(err), "dag-run directory should be removed")

		// Verify log files are removed
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveWithChildDAGRuns", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files for parent and children
		parentLogFiles := []string{
			filepath.Join(tmpDir, "parent.log"),
			filepath.Join(tmpDir, "parent_step.out"),
		}
		child1LogFiles := []string{
			filepath.Join(tmpDir, "child1.log"),
			filepath.Join(tmpDir, "child1_step.out"),
		}
		child2LogFiles := []string{
			filepath.Join(tmpDir, "child2.log"),
			filepath.Join(tmpDir, "child2_step.out"),
		}

		allLogFiles := append(append(parentLogFiles, child1LogFiles...), child2LogFiles...)
		for _, logFile := range allLogFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0600))
		}

		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "parent-dag-run", execution.NewUTC(time.Now()))

		// Create parent dag-run with log files
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "parent-dag-run"
		dagRunStatus.Log = parentLogFiles[0]
		dagRunStatus.Nodes = []*execution.Node{{
			Step:   core.Step{Name: "parent-step"},
			Stdout: parentLogFiles[1],
		}}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		// Create child dag-run directory
		childDir := filepath.Join(run.baseDir, ChildDAGRunsDir)
		require.NoError(t, os.MkdirAll(childDir, 0750))

		// Create two child dag-run with their own log files
		childDAGRuns := []struct {
			dagRunID string
			logFiles []string
		}{
			{"child1", child1LogFiles},
			{"child2", child2LogFiles},
		}

		for _, child := range childDAGRuns {
			childDAGRunDir := filepath.Join(childDir, ChildDAGRunDirPrefix+child.dagRunID)
			require.NoError(t, os.MkdirAll(childDAGRunDir, 0750))

			childDAGRun, err := NewDAGRun(childDAGRunDir)
			require.NoError(t, err)

			childStatus := execution.InitialStatus(dag)
			childStatus.DAGRunID = child.dagRunID
			childStatus.Log = child.logFiles[0]
			childStatus.Nodes = []*execution.Node{{
				Step:   core.Step{Name: fmt.Sprintf("%s-step", child.dagRunID)},
				Stdout: child.logFiles[1],
			}}

			childRun, err := childDAGRun.CreateAttempt(run.Context, ts, nil)
			require.NoError(t, err)
			require.NoError(t, childRun.Open(run.Context))
			require.NoError(t, childRun.Write(run.Context, childStatus))
			require.NoError(t, childRun.Close(run.Context))
		}

		// Verify all files exist before removal
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove the parent dag-run (should remove all log files including child dag-runs)
		err = run.Remove(run.Context)
		require.NoError(t, err)

		// Verify dag-run directory is removed
		_, err = os.Stat(run.baseDir)
		assert.True(t, os.IsNotExist(err), "dag-run directory should be removed")

		// Verify all log files are removed (parent and children)
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveHandlesNonExistentLogFiles", func(t *testing.T) {
		root := setupTestDataRoot(t)
		run := root.CreateTestDAGRun(t, "test-dag-run", execution.NewUTC(time.Now()))

		// Create a run with log files that don't exist
		dag := &core.DAG{Name: "test-dag"}
		dagRunStatus := execution.InitialStatus(dag)
		dagRunStatus.DAGRunID = "test-dag-run"
		dagRunStatus.Log = "/non/existent/path/dag-run.log"
		dagRunStatus.Nodes = []*execution.Node{
			{
				Step:   core.Step{Name: "step1"},
				Stdout: "/non/existent/path/step1.out",
				Stderr: "/non/existent/path/step1.err",
			},
		}

		ts := execution.NewUTC(time.Now())
		att, err := run.CreateAttempt(run.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, att.Open(run.Context))
		require.NoError(t, att.Write(run.Context, dagRunStatus))
		require.NoError(t, att.Close(run.Context))

		// Remove should not fail even if log files don't exist
		err = run.Remove(run.Context)
		require.NoError(t, err)

		// Verify dag-run directory is removed
		_, err = os.Stat(run.baseDir)
		assert.True(t, os.IsNotExist(err), "dag-run directory should be removed")
	})
}

func TestDAGRun_listAttemptDirs(t *testing.T) {
	ctx := context.Background()
	root := setupTestDataRoot(t)

	// Create DAG run directory manually without creating any attempts
	dagRunDir := filepath.Join(root.dagRunsDir, "2025", "07", "22", "dag-run_20250722_120000Z_test-run")
	require.NoError(t, os.MkdirAll(dagRunDir, 0755))

	run, err := NewDAGRun(dagRunDir)
	require.NoError(t, err)

	// Create some normal attempt directories with older timestamps
	normalAttempt1 := filepath.Join(run.baseDir, "attempt_20250722_120000_123Z_abc123")
	normalAttempt2 := filepath.Join(run.baseDir, "attempt_20250722_120100_456Z_def456")
	require.NoError(t, os.MkdirAll(normalAttempt1, 0755))
	require.NoError(t, os.MkdirAll(normalAttempt2, 0755))

	// Create a hidden attempt directory with the latest timestamp
	hiddenAttempt := filepath.Join(run.baseDir, ".attempt_20250722_120200_789Z_ghi789")
	require.NoError(t, os.MkdirAll(hiddenAttempt, 0755))

	// Create some non-attempt directories that should be ignored
	require.NoError(t, os.MkdirAll(filepath.Join(run.baseDir, "not-an-attempt"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(run.baseDir, ".hidden-but-not-attempt"), 0755))

	// Create a file that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(run.baseDir, "attempt_fake.txt"), []byte("fake"), 0644))

	// Get all attempt directories
	dirs, err := run.listAttemptDirs()
	require.NoError(t, err)

	// Should return 3 directories (2 normal + 1 hidden)
	assert.Len(t, dirs, 3, "should return all attempt directories including hidden ones")

	// Verify the directories are sorted in reverse order (newest first)
	// The hidden attempt with latest timestamp should be first
	expected := []string{
		".attempt_20250722_120200_789Z_ghi789", // Latest (hidden)
		"attempt_20250722_120100_456Z_def456",  // Second
		"attempt_20250722_120000_123Z_abc123",  // Oldest
	}
	assert.Equal(t, expected, dirs, "directories should be sorted newest first with hidden directory in correct position")

	// Create status files so attempts are considered valid
	for _, dir := range []string{normalAttempt1, normalAttempt2, hiddenAttempt} {
		statusFile := filepath.Join(dir, JSONLStatusFile)
		status := createTestStatus(status.Success)
		data, err := json.Marshal(status)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(statusFile, append(data, '\n'), 0600))
	}

	// Test that LatestAttempt skips hidden directories
	latestAttempt, err := run.LatestAttempt(ctx, nil)
	require.NoError(t, err)
	assert.False(t, latestAttempt.Hidden(), "LatestAttempt should skip hidden attempts")

	// The latest visible attempt should be normalAttempt2
	status, err := latestAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test", status.DAGRunID, "should return the latest visible attempt")
}
