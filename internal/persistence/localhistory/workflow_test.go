package localhistory

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecution(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-id-1", models.NewUTC(time.Now()))

		ts1 := models.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := models.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := models.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))

		_ = exec.WriteStatus(t, ts1, scheduler.StatusRunning)
		_ = exec.WriteStatus(t, ts2, scheduler.StatusSuccess)
		_ = exec.WriteStatus(t, ts3, scheduler.StatusError)

		latestRun, err := exec.LatestRun(exec.Context, nil)
		require.NoError(t, err)

		status, err := latestRun.ReadStatus(exec.Context)
		require.NoError(t, err)

		require.Equal(t, scheduler.StatusError.String(), status.Status.String())
	})
	t.Run("LastUpdated", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-id-1", models.NewUTC(time.Now()))

		ts1 := models.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := models.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))

		_ = exec.WriteStatus(t, ts1, scheduler.StatusRunning)
		run := exec.WriteStatus(t, ts2, scheduler.StatusSuccess)

		lastUpdate, err := exec.LastUpdated(exec.Context)
		require.NoError(t, err)

		info, err := os.Stat(run.file)
		require.NoError(t, err)

		require.Equal(t, info.ModTime(), lastUpdate)
	})
}

type ExecutionTest struct {
	DataRootTest
	*Workflow
	TB testing.TB
}

func (et ExecutionTest) WriteStatus(t *testing.T, ts models.TimeInUTC, s scheduler.Status) *Run {
	t.Helper()

	dag := &digraph.DAG{Name: "test-dag"}
	status := models.InitialStatus(dag)
	status.WorkflowID = "test-id-1"
	status.Status = s

	run, err := et.CreateRun(et.Context, ts, nil)
	require.NoError(t, err)
	err = run.Open(et.Context)
	require.NoError(t, err)

	defer func() {
		_ = run.Close(et.Context)
	}()

	err = run.Write(et.Context, status)
	require.NoError(t, err)

	return run
}

func TestWorkflowListChildWorkflows(t *testing.T) {
	t.Run("NoChildWorkflows", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		children, err := exec.ListChildWorkflows(exec.Context)
		require.NoError(t, err)
		assert.Empty(t, children, "should return empty list when no child workflows exist")
	})

	t.Run("WithChildWorkflows", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "parent-workflow", models.NewUTC(time.Now()))

		// Create child workflows directory and some child workflows
		childDir := filepath.Join(exec.baseDir, ChildWorkflowsDir)
		require.NoError(t, os.MkdirAll(childDir, 0755))

		// Create two child workflow directories
		child1Dir := filepath.Join(childDir, ChildWorkflowDirPrefix+"child1")
		child2Dir := filepath.Join(childDir, ChildWorkflowDirPrefix+"child2")
		require.NoError(t, os.MkdirAll(child1Dir, 0755))
		require.NoError(t, os.MkdirAll(child2Dir, 0755))

		// Create a non-directory file (should be ignored)
		nonDirFile := filepath.Join(childDir, "not-a-directory.txt")
		require.NoError(t, os.WriteFile(nonDirFile, []byte("test"), 0644))

		children, err := exec.ListChildWorkflows(exec.Context)
		require.NoError(t, err)
		assert.Len(t, children, 2, "should return two child workflows")

		// Verify child workflow IDs
		childIDs := make([]string, len(children))
		for i, child := range children {
			childIDs[i] = child.workflowID
		}
		assert.Contains(t, childIDs, "child1")
		assert.Contains(t, childIDs, "child2")
	})
}

func TestWorkflowListRuns(t *testing.T) {
	t.Run("NoRuns", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Remove the run that was created by CreateTestExecution
		require.NoError(t, os.RemoveAll(exec.baseDir))
		require.NoError(t, os.MkdirAll(exec.baseDir, 0755))

		runs, err := exec.ListRuns(exec.Context)
		require.NoError(t, err)
		assert.Empty(t, runs, "should return empty list when no runs exist")
	})

	t.Run("WithMultipleRuns", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Create additional runs
		ts1 := models.NewUTC(time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC))
		ts2 := models.NewUTC(time.Date(2021, 1, 2, 12, 0, 0, 0, time.UTC))
		exec.WriteStatus(t, ts1, scheduler.StatusSuccess)
		exec.WriteStatus(t, ts2, scheduler.StatusError)

		runs, err := exec.ListRuns(exec.Context)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(runs), 2, "should return at least the runs we created")

		// Verify runs are valid
		for _, run := range runs {
			assert.True(t, run.Exists(), "each run should exist")
		}
	})
}

func TestWorkflowListLogFiles(t *testing.T) {
	t.Run("WithLogFiles", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Create a run with log files
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "test-workflow"
		status.Status = scheduler.StatusSuccess
		status.Log = "/tmp/test.log"
		status.Nodes = []*models.Node{
			{
				Step: digraph.Step{Name: "step1"},
				Stdout: "/tmp/step1.out",
				Stderr: "/tmp/step1.err",
			},
			{
				Step: digraph.Step{Name: "step2"}, 
				Stdout: "/tmp/step2.out",
				Stderr: "/tmp/step2.err",
			},
		}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		logFiles, err := exec.listLogFiles(exec.Context)
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

func TestWorkflowRemoveLogFiles(t *testing.T) {
	t.Run("RemoveMainWorkflowLogFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files
		logFiles := []string{
			filepath.Join(tmpDir, "main.log"),
			filepath.Join(tmpDir, "step1.out"),
			filepath.Join(tmpDir, "step1.err"),
		}

		for _, logFile := range logFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0644))
		}

		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Create a run with log files pointing to our test files
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "test-workflow"
		status.Status = scheduler.StatusSuccess
		status.Log = logFiles[0]
		status.Nodes = []*models.Node{
			{
				Step: digraph.Step{Name: "step1"},
				Stdout: logFiles[1],
				Stderr: logFiles[2],
			},
		}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		// Verify files exist before removal
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove log files
		err = exec.removeLogFiles(exec.Context)
		require.NoError(t, err)

		// Verify files are removed
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveChildWorkflowLogFiles", func(t *testing.T) {
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
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0644))
		}

		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "parent-workflow", models.NewUTC(time.Now()))

		// Create parent workflow with log files
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "parent-workflow"
		status.Log = parentLogFiles[0]
		status.Nodes = []*models.Node{{
			Step: digraph.Step{Name: "parent-step"},
			Stdout: parentLogFiles[1],
		}}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		// Create child workflow
		childDir := filepath.Join(exec.baseDir, ChildWorkflowsDir)
		require.NoError(t, os.MkdirAll(childDir, 0755))
		
		childWorkflowDir := filepath.Join(childDir, ChildWorkflowDirPrefix+"child1")
		require.NoError(t, os.MkdirAll(childWorkflowDir, 0755))

		childWorkflow, err := NewWorkflow(childWorkflowDir)
		require.NoError(t, err)

		// Create child run with log files
		childStatus := models.InitialStatus(dag)
		childStatus.WorkflowID = "child1"
		childStatus.Log = childLogFiles[0]
		childStatus.Nodes = []*models.Node{{
			Step: digraph.Step{Name: "child-step"},
			Stdout: childLogFiles[1],
		}}

		childRun, err := childWorkflow.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, childRun.Open(exec.Context))
		require.NoError(t, childRun.Write(exec.Context, childStatus))
		require.NoError(t, childRun.Close(exec.Context))

		// Verify all files exist before removal
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove log files (should remove both parent and child log files)
		err = exec.removeLogFiles(exec.Context)
		require.NoError(t, err)

		// Verify all files are removed
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})
}

func TestWorkflowRemove(t *testing.T) {
	t.Run("RemoveWorkflowWithLogFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test log files
		logFiles := []string{
			filepath.Join(tmpDir, "workflow.log"),
			filepath.Join(tmpDir, "step1.out"),
			filepath.Join(tmpDir, "step1.err"),
		}

		for _, logFile := range logFiles {
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0644))
		}

		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Create a run with log files
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "test-workflow"
		status.Status = scheduler.StatusSuccess
		status.Log = logFiles[0]
		status.Nodes = []*models.Node{
			{
				Step: digraph.Step{Name: "step1"},
				Stdout: logFiles[1],
				Stderr: logFiles[2],
			},
		}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		// Verify workflow directory and log files exist
		_, err = os.Stat(exec.baseDir)
		require.NoError(t, err, "workflow directory should exist")

		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist: %s", logFile)
		}

		// Remove the workflow
		err = exec.Remove(exec.Context)
		require.NoError(t, err)

		// Verify workflow directory is removed
		_, err = os.Stat(exec.baseDir)
		assert.True(t, os.IsNotExist(err), "workflow directory should be removed")

		// Verify log files are removed
		for _, logFile := range logFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveWorkflowWithChildWorkflows", func(t *testing.T) {
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
			require.NoError(t, os.WriteFile(logFile, []byte("test log content"), 0644))
		}

		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "parent-workflow", models.NewUTC(time.Now()))

		// Create parent workflow with log files
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "parent-workflow"
		status.Log = parentLogFiles[0]
		status.Nodes = []*models.Node{{
			Step: digraph.Step{Name: "parent-step"},
			Stdout: parentLogFiles[1],
		}}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		// Create child workflows
		childDir := filepath.Join(exec.baseDir, ChildWorkflowsDir)
		require.NoError(t, os.MkdirAll(childDir, 0755))

		// Create two child workflows with their own log files
		childWorkflows := []struct {
			workflowID string
			logFiles   []string
		}{
			{"child1", child1LogFiles},
			{"child2", child2LogFiles},
		}

		for _, child := range childWorkflows {
			childWorkflowDir := filepath.Join(childDir, ChildWorkflowDirPrefix+child.workflowID)
			require.NoError(t, os.MkdirAll(childWorkflowDir, 0755))

			childWorkflow, err := NewWorkflow(childWorkflowDir)
			require.NoError(t, err)

			childStatus := models.InitialStatus(dag)
			childStatus.WorkflowID = child.workflowID
			childStatus.Log = child.logFiles[0]
			childStatus.Nodes = []*models.Node{{
				Step: digraph.Step{Name: fmt.Sprintf("%s-step", child.workflowID)},
				Stdout: child.logFiles[1],
			}}

			childRun, err := childWorkflow.CreateRun(exec.Context, ts, nil)
			require.NoError(t, err)
			require.NoError(t, childRun.Open(exec.Context))
			require.NoError(t, childRun.Write(exec.Context, childStatus))
			require.NoError(t, childRun.Close(exec.Context))
		}

		// Verify all files exist before removal
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			require.NoError(t, err, "log file should exist before removal: %s", logFile)
		}

		// Remove the parent workflow (should remove all log files including child workflows)
		err = exec.Remove(exec.Context)
		require.NoError(t, err)

		// Verify workflow directory is removed
		_, err = os.Stat(exec.baseDir)
		assert.True(t, os.IsNotExist(err), "workflow directory should be removed")

		// Verify all log files are removed (parent and children)
		for _, logFile := range allLogFiles {
			_, err := os.Stat(logFile)
			assert.True(t, os.IsNotExist(err), "log file should be removed: %s", logFile)
		}
	})

	t.Run("RemoveHandlesNonExistentLogFiles", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-workflow", models.NewUTC(time.Now()))

		// Create a run with log files that don't exist
		dag := &digraph.DAG{Name: "test-dag"}
		status := models.InitialStatus(dag)
		status.WorkflowID = "test-workflow"
		status.Log = "/non/existent/path/workflow.log"
		status.Nodes = []*models.Node{
			{
				Step: digraph.Step{Name: "step1"},
				Stdout: "/non/existent/path/step1.out",
				Stderr: "/non/existent/path/step1.err",
			},
		}

		ts := models.NewUTC(time.Now())
		run, err := exec.CreateRun(exec.Context, ts, nil)
		require.NoError(t, err)
		require.NoError(t, run.Open(exec.Context))
		require.NoError(t, run.Write(exec.Context, status))
		require.NoError(t, run.Close(exec.Context))

		// Remove should not fail even if log files don't exist
		err = exec.Remove(exec.Context)
		require.NoError(t, err)

		// Verify workflow directory is removed
		_, err = os.Stat(exec.baseDir)
		assert.True(t, os.IsNotExist(err), "workflow directory should be removed")
	})
}
