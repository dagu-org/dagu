package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/stringutil"
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
		err = th.QueueStore.Enqueue(th.Context, dag.ProcGroup(), execution.QueuePriorityLow, execution.NewDAGRunRef(dag.Name, dagRunID))
		require.NoError(t, err)
	}

	// Verify queue has correct number of items
	queuedItems, err := th.QueueStore.List(th.Context, dag.ProcGroup())
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

	// Wait until queue is empty
	startTime := time.Now()

	require.Eventually(t, func() bool {
		remaining, err := th.QueueStore.List(th.Context, dag.Name)
		if err != nil {
			t.Logf("Error checking queue: %v", err)
			return false
		}

		t.Logf("Queue: %d/%d items remaining", len(remaining), numItems)

		return len(remaining) == 0
	}, 25*time.Second, 200*time.Millisecond, "Queue items should be processed")

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
}

// TestGlobalQueueMaxConcurrency verifies that a global queue with maxConcurrency > 1
// processes multiple DAGs concurrently, and that the DAG's maxActiveRuns doesn't
// override the global queue's maxConcurrency setting.
func TestGlobalQueueMaxConcurrency(t *testing.T) {
	t.Parallel()

	// Configure a global queue with maxConcurrency = 3 BEFORE setup
	// so it gets written to the config file
	th := test.SetupCommand(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "global-queue", MaxActiveRuns: 3},
		}
	}))

	// Create a DAG with maxActiveRuns = 1 that uses the global queue
	// Each DAG sleeps for 1 second to ensure we can detect concurrent vs sequential execution
	dagContent := `name: concurrent-test
queue: global-queue
maxActiveRuns: 1
steps:
  - name: sleep-step
    command: sleep 1
`
	require.NoError(t, os.MkdirAll(th.Config.Paths.DAGsDir, 0755))
	dagFile := filepath.Join(th.Config.Paths.DAGsDir, "concurrent-test.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0644))

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	// Enqueue 3 items
	runIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dagRunID := uuid.New().String()
		runIDs[i] = dagRunID

		att, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		logFile := filepath.Join(th.Config.Paths.LogDir, dag.Name, dagRunID+".log")
		require.NoError(t, os.MkdirAll(filepath.Dir(logFile), 0755))

		dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{},
			transform.WithLogFilePath(logFile),
			transform.WithAttemptID(att.ID()),
			transform.WithHierarchyRefs(
				execution.NewDAGRunRef(dag.Name, dagRunID),
				execution.DAGRunRef{},
			),
		)

		require.NoError(t, att.Open(th.Context))
		require.NoError(t, att.Write(th.Context, dagStatus))
		require.NoError(t, att.Close(th.Context))

		err = th.QueueStore.Enqueue(th.Context, "global-queue", execution.QueuePriorityLow, execution.NewDAGRunRef(dag.Name, dagRunID))
		require.NoError(t, err)
	}

	// Verify queue has 3 items
	queuedItems, err := th.QueueStore.List(th.Context, "global-queue")
	require.NoError(t, err)
	require.Len(t, queuedItems, 3)

	// Start scheduler
	schedulerDone := make(chan error, 1)
	daguHome := filepath.Dir(th.Config.Paths.DAGsDir)
	go func() {
		ctx, cancel := context.WithTimeout(th.Context, 30*time.Second)
		defer cancel()

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

	// Wait until all DAGs complete
	require.Eventually(t, func() bool {
		remaining, err := th.QueueStore.List(th.Context, "global-queue")
		if err != nil {
			return false
		}
		t.Logf("Queue: %d/3 items remaining", len(remaining))
		return len(remaining) == 0
	}, 15*time.Second, 200*time.Millisecond, "Queue items should be processed")

	th.Cancel()

	select {
	case <-schedulerDone:
	case <-time.After(5 * time.Second):
	}

	// Collect start times from all DAG runs
	var startTimes []time.Time
	for _, runID := range runIDs {
		attempt, err := th.DAGRunStore.FindAttempt(th.Context, execution.NewDAGRunRef(dag.Name, runID))
		require.NoError(t, err)

		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)

		startedAt, err := stringutil.ParseTime(status.StartedAt)
		require.NoError(t, err, "Failed to parse start time for run %s", runID)
		require.False(t, startedAt.IsZero(), "Start time is zero for run %s", runID)
		startTimes = append(startTimes, startedAt)
	}

	// All 3 DAGs should have started
	require.Len(t, startTimes, 3, "All 3 DAGs should have started")

	// Find the max difference between start times
	// If they ran concurrently, all should start within ~500ms
	// If they ran sequentially (maxConcurrency=1), they'd be ~1s apart
	var maxDiff time.Duration
	for i := 0; i < len(startTimes); i++ {
		for j := i + 1; j < len(startTimes); j++ {
			diff := startTimes[i].Sub(startTimes[j]).Abs()
			if diff > maxDiff {
				maxDiff = diff
			}
		}
	}

	t.Logf("Start times: %v", startTimes)
	t.Logf("Max difference between start times: %v", maxDiff)

	// All DAGs should start within 2 seconds of each other (concurrent execution)
	// If maxConcurrency was incorrectly set to 1, they would start ~3+ seconds apart
	// (due to 1 second sleep in each DAG + processing overhead)
	require.Less(t, maxDiff, 2*time.Second,
		"All 3 DAGs should start concurrently (within 2s), but max diff was %v", maxDiff)
}

// TestDAGQueueMaxActiveRunsFirstBatch verifies that when a DAG-based (non-global)
// queue is first encountered, all items up to maxActiveRuns are processed in the
// first batch, not just 1.
//
// This test covers the bug where dynamically created queues were initialized with
// maxConcurrency=1, causing only 1 DAG to start initially even when maxActiveRuns > 1.
// The fix reads the DAG's maxActiveRuns before selecting items for the first batch.
func TestDAGQueueMaxActiveRunsFirstBatch(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		// No global queues configured - we want to test DAG-based queues
	}))

	// Create a DAG with maxActiveRuns = 3 (no queue: field, so it uses DAG-based queue)
	// Each DAG sleeps for 2 seconds to ensure we can detect concurrent vs sequential execution
	dagContent := `name: dag-queue-test
maxActiveRuns: 3
steps:
  - name: sleep-step
    command: sleep 2
`
	require.NoError(t, os.MkdirAll(th.Config.Paths.DAGsDir, 0755))
	dagFile := filepath.Join(th.Config.Paths.DAGsDir, "dag-queue-test.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0644))

	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	// Verify queue name is the DAG name (DAG-based queue)
	queueName := dag.ProcGroup()
	require.Equal(t, "dag-queue-test", queueName, "DAG should use its name as queue")

	// Enqueue 3 items
	runIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dagRunID := uuid.New().String()
		runIDs[i] = dagRunID

		att, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		logFile := filepath.Join(th.Config.Paths.LogDir, dag.Name, dagRunID+".log")
		require.NoError(t, os.MkdirAll(filepath.Dir(logFile), 0755))

		dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{},
			transform.WithLogFilePath(logFile),
			transform.WithAttemptID(att.ID()),
			transform.WithHierarchyRefs(
				execution.NewDAGRunRef(dag.Name, dagRunID),
				execution.DAGRunRef{},
			),
		)

		require.NoError(t, att.Open(th.Context))
		require.NoError(t, att.Write(th.Context, dagStatus))
		require.NoError(t, att.Close(th.Context))

		err = th.QueueStore.Enqueue(th.Context, queueName, execution.QueuePriorityLow, execution.NewDAGRunRef(dag.Name, dagRunID))
		require.NoError(t, err)
	}

	// Verify queue has 3 items
	queuedItems, err := th.QueueStore.List(th.Context, queueName)
	require.NoError(t, err)
	require.Len(t, queuedItems, 3)
	t.Logf("Enqueued 3 items to DAG-based queue %q", queueName)

	// Start scheduler
	schedulerDone := make(chan error, 1)
	daguHome := filepath.Dir(th.Config.Paths.DAGsDir)
	go func() {
		ctx, cancel := context.WithTimeout(th.Context, 30*time.Second)
		defer cancel()

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

	// Wait until all DAGs complete
	startTime := time.Now()
	require.Eventually(t, func() bool {
		remaining, err := th.QueueStore.List(th.Context, queueName)
		if err != nil {
			return false
		}
		t.Logf("Queue: %d/3 items remaining", len(remaining))
		return len(remaining) == 0
	}, 20*time.Second, 200*time.Millisecond, "Queue items should be processed")

	totalDuration := time.Since(startTime)
	t.Logf("All items processed in %v", totalDuration)

	th.Cancel()

	select {
	case <-schedulerDone:
	case <-time.After(5 * time.Second):
	}

	// Collect start times from all DAG runs
	var startTimes []time.Time
	for _, runID := range runIDs {
		attempt, err := th.DAGRunStore.FindAttempt(th.Context, execution.NewDAGRunRef(dag.Name, runID))
		require.NoError(t, err)

		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)

		startedAt, err := stringutil.ParseTime(status.StartedAt)
		require.NoError(t, err, "Failed to parse start time for run %s", runID)
		require.False(t, startedAt.IsZero(), "Start time is zero for run %s", runID)
		startTimes = append(startTimes, startedAt)
	}

	// All 3 DAGs should have started
	require.Len(t, startTimes, 3, "All 3 DAGs should have started")

	// Find the max difference between start times
	var maxDiff time.Duration
	for i := 0; i < len(startTimes); i++ {
		for j := i + 1; j < len(startTimes); j++ {
			diff := startTimes[i].Sub(startTimes[j]).Abs()
			if diff > maxDiff {
				maxDiff = diff
			}
		}
	}

	t.Logf("Start times: %v", startTimes)
	t.Logf("Max difference between start times: %v", maxDiff)

	// KEY ASSERTION: All 3 DAGs should start in the FIRST batch (concurrently)
	// If the bug exists (queue initialized with maxConcurrency=1), they would start
	// sequentially: first at 0s, second at ~2s, third at ~4s (total ~6s+)
	// With the fix, all 3 start within the first batch, so max diff <= 2s
	require.LessOrEqual(t, maxDiff, 2*time.Second,
		"All 3 DAGs should start in first batch (within 2s), but max diff was %v. "+
			"This suggests maxActiveRuns was not applied to the first batch.", maxDiff)

	// Also verify total time is reasonable
	// - Concurrent execution: ~2s sleep + scheduler overhead (~4-6s total)
	// - Sequential execution (bug): ~6s sleep + overhead (~8-10s total)
	require.Less(t, totalDuration, 8*time.Second,
		"Total processing time should be under 8s for concurrent execution, but was %v", totalDuration)
}

// TestCronScheduleRunsTwice verifies that a DAG with */1 * * * * schedule
// runs twice in two minutes.
func TestCronScheduleRunsTwice(t *testing.T) {
	t.Parallel()

	// Create a temp directory for test DAGs
	tmpDir, err := os.MkdirTemp("", "dagu-cron-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0755))

	// Create a DAG with a cron schedule that runs every minute
	dagContent := `name: cron-test
schedule: "*/1 * * * *"
steps:
  - name: test-step
    command: echo "hello"
`
	dagFile := filepath.Join(dagsDir, "cron-test.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0644))

	// Setup scheduler with the custom DAGs directory
	th := test.SetupScheduler(t, test.WithDAGsDir(dagsDir))

	// Create scheduler instance
	schedulerInstance, err := th.NewSchedulerInstance(t)
	require.NoError(t, err)

	// Start the scheduler
	ctx, cancel := context.WithCancel(th.Context)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- schedulerInstance.Start(ctx)
	}()

	// Load DAG for status checking
	dag, err := spec.Load(th.Context, dagFile)
	require.NoError(t, err)

	// Poll until we have at least 2 runs or timeout after 2.5 minutes
	// This is more reliable than sleeping a fixed duration since it doesn't
	// depend on when the test starts relative to the minute boundary.
	timeout := time.After(2*time.Minute + 30*time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var runCount int
waitLoop:
	for {
		select {
		case <-timeout:
			break waitLoop
		case <-ticker.C:
			runs := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
			runCount = len(runs)
			if runCount >= 2 {
				break waitLoop
			}
		}
	}

	// Stop the scheduler
	schedulerInstance.Stop(ctx)
	cancel()

	select {
	case err = <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		// Timeout is acceptable for cleanup
	}

	// Verify the DAG ran at least twice (re-check after cleanup in case more runs completed)
	runs := th.DAGRunMgr.ListRecentStatus(th.Context, dag.Name, 10)
	require.GreaterOrEqual(t, len(runs), 2, "expected at least 2 DAG runs, got %d", len(runs))
}
