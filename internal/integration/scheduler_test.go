package integration_test

// Queue Processing Bug Integration Test
//
// This file contains integration tests for the scheduler's queue processing functionality.
// These tests reproduce bugs in internal/service/scheduler/scheduler.go where items
// get stuck in the queue due to synchronous processing in handleQueue().
//
// Key Files:
// - internal/service/scheduler/scheduler.go: handleQueue function
// - internal/service/scheduler/scheduler.go: queue channel definition (buffer=1)
// - internal/persistence/filequeue/reader.go: queue reader implementation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestQueueProcessing_TenItems ensures queued DAG runs drain promptly.
//
// This integration test covers the fixed behaviour by pushing several quick DAGs
// through the queue and asserting that they all complete within the timeout.
func TestQueueProcessing(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	// Enable queues
	th.Config.Queues.Enabled = true

	// Create simple DAG
	dagContent := `name: simple-echo
steps:
  - name: echo-hello
    command: echo "hello"
`
	require.NoError(t, os.MkdirAll(th.Config.Paths.DAGsDir, 0755))
	dagFile := filepath.Join(th.Config.Paths.DAGsDir, "simple-echo.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0644))

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	// Enqueue items directly
	numItems := 3
	for i := 0; i < numItems; i++ {
		dagRunID := uuid.New().String()

		// Create DAG run attempt (like cmd/enqueue.go does)
		att, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		// Create log file path
		logFile := filepath.Join(th.Config.Paths.LogDir, dag.Name, dagRunID+".log")
		require.NoError(t, os.MkdirAll(filepath.Dir(logFile), 0755))

		// Create initial queued status
		dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{},
			transform.WithLogFilePath(logFile),
			transform.WithAttemptID(att.ID()),
			transform.WithHierarchyRefs(
				execution.NewDAGRunRef(dag.Name, dagRunID),
				execution.DAGRunRef{},
			),
		)

		// Write status to attempt
		require.NoError(t, att.Open(th.Context))
		require.NoError(t, att.Write(th.Context, dagStatus))
		require.NoError(t, att.Close(th.Context))

		// Enqueue to queue
		err = th.QueueStore.Enqueue(th.Context, dag.Name, execution.QueuePriorityLow, execution.NewDAGRunRef(dag.Name, dagRunID))
		require.NoError(t, err)
	}

	// Verify queue has correct number of items
	queuedItems, err := th.QueueStore.List(th.Context, dag.Name)
	require.NoError(t, err)
	require.Len(t, queuedItems, numItems)
	t.Logf("Enqueued %d items", numItems)

	// Start scheduler
	schedulerDone := make(chan error, 1)
	daguHome := filepath.Dir(th.Config.Paths.DAGsDir)
	go func() {
		// Use timeout context for scheduler without modifying shared th.Context
		ctx, cancel := context.WithTimeout(th.Context, 30*time.Second)
		defer cancel()

		// Create a copy of test helper with timeout context to avoid race
		thCopy := th
		thCopy.Context = ctx

		schedulerDone <- thCopy.RunCommandWithError(t, cmd.Scheduler(), test.CmdTest{
			Args: []string{
				"scheduler",
				"--dagu-home", daguHome,
			},
			ExpectedOut: []string{"Scheduler started"},
		})
	}()

	time.Sleep(500 * time.Millisecond)

	// Poll until queue is empty
	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	processed := false
	timeout := time.After(25 * time.Second)

	for {
		select {
		case <-ticker.C:
			remaining, err := th.QueueStore.List(th.Context, dag.Name)
			if err != nil {
				t.Logf("Error checking queue: %v", err)
				continue
			}

			t.Logf("Queue: %d/%d items remaining", len(remaining), numItems)

			if len(remaining) == 0 {
				processed = true
				goto DONE
			}

		case <-timeout:
			remaining, _ := th.QueueStore.List(th.Context, dag.Name)
			t.Fatalf("Timeout: %d items still in queue", len(remaining))
		}
	}

DONE:
	th.Cancel()

	select {
	case err := <-schedulerDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
	}

	duration := time.Since(startTime)
	t.Logf("Processed %d items in %v", numItems, duration)

	// Verify queue is empty
	finalQueue, err := th.QueueStore.List(th.Context, dag.Name)
	require.NoError(t, err)
	require.Empty(t, finalQueue, "queue should be empty")

	// Verify processing time is reasonable
	require.Less(t, duration, 20*time.Second, "took too long: %v", duration)

	require.True(t, processed, "items should be processed")
}
