package worker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	runtime1 "github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTaskHandler(t *testing.T) {
	th := test.Setup(t)

	t.Run("HandleTaskRetry", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: echo step1
  - name: "2"
    command: echo step2
`)
		ctx := th.Context

		// First, start a DAG run
		spec := th.SubCmdBuilder.Start(dag.DAG, runtime1.StartOptions{})
		err := runtime1.Start(th.Context, spec)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, core.Succeeded)

		// Get the st to get the dag-run ID
		st, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := st.DAGRunID

		// Create a retry task
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_RETRY,
			DagRunId:  dagRunID,
			Target:    dag.Name,
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := NewTaskHandler(th.Config)
		err = handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran again successfully
		dag.AssertLatestStatus(t, core.Succeeded)
	})

	t.Run("HandleTaskRetryWithStep", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: echo step1
  - name: "2"
    command: echo step2
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// First, start a DAG run
		spec := th.SubCmdBuilder.Start(dag.DAG, runtime1.StartOptions{})
		err := runtime1.Start(th.Context, spec)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, core.Succeeded)

		// Get the st to get the dag-run ID
		st, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := st.DAGRunID

		// Create a retry task with specific step
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_RETRY,
			DagRunId:  dagRunID,
			Target:    dag.Name,
			Step:      "1",
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := NewTaskHandler(th.Config)
		err = handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran again successfully
		dag.AssertLatestStatus(t, core.Succeeded)
	})

	t.Run("HandleTaskStart", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "process"
    command: echo processing $1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// Generate a new dag-run ID
		dagRunID := uuid.Must(uuid.NewV7()).String()

		// Create a start task
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  dagRunID,
			Target:    dag.Location,
			Params:    "param1=value1",
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := NewTaskHandler(th.Config)
		err := handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran successfully
		dag.AssertLatestStatus(t, core.Succeeded)

		// Verify the params were passed
		status, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, dagRunID, status.DAGRunID)
		require.Equal(t, "param1=value1", status.Params)
	})

	t.Run("HandleTaskInvalidOperation", func(t *testing.T) {
		ctx := th.Context

		// Create a task with invalid operation
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			DagRunId:  "test-id",
			Target:    "test-dag",
		}

		// Execute the task
		handler := NewTaskHandler(th.Config)
		err := handler.Handle(ctx, task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "operation not specified")
	})
}

func TestCreateTempDAGFile(t *testing.T) {
	path, err := createTempDAGFile("simple", []byte("steps:\n  - name: example\n"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = os.Remove(path) })

	require.FileExists(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "steps:\n  - name: example\n", string(data))

	expectedDir := filepath.Join(os.TempDir(), "dagu", "worker-dags") + string(os.PathSeparator)
	require.True(t, strings.HasPrefix(path, expectedDir), "expected %q to start with %q", path, expectedDir)
}

func TestTaskHandlerStartWithDefinition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required for fake executable script")
	}

	tmpDir := t.TempDir()
	argsPath := filepath.Join(tmpDir, "args.txt")
	fakeExec := filepath.Join(tmpDir, "fake-dagu.sh")

	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	err := os.WriteFile(fakeExec, []byte(script), 0o700)
	require.NoError(t, err)

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: fakeExec,
		},
		Global: config.Global{
			BaseEnv: config.NewBaseEnv(nil),
		},
	}

	handler := NewTaskHandler(cfg)

	originalTarget := "workflow.yaml"
	task := &coordinatorv1.Task{
		Operation:  coordinatorv1.Operation_OPERATION_START,
		DagRunId:   "run-123",
		Target:     originalTarget,
		Definition: "steps:\n  - name: example\n",
		Params:     "foo=bar",
	}

	err = handler.Handle(context.Background(), task)
	require.NoError(t, err)

	require.NotEqual(t, originalTarget, task.Target)

	argsData, err := os.ReadFile(argsPath)
	require.NoError(t, err)

	argsLines := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	require.Contains(t, argsLines, "start")
	require.Contains(t, argsLines, "--run-id=run-123")
	require.Contains(t, argsLines, "--no-queue")
	require.Contains(t, argsLines, task.Target)
	require.Contains(t, argsLines, "--")
	require.Contains(t, argsLines, "foo=bar")

	_, statErr := os.Stat(task.Target)
	require.True(t, os.IsNotExist(statErr), "temporary DAG file should be removed after execution")
}
