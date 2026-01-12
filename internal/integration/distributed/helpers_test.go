package distributed_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Worker Mode Constants
// =============================================================================

type WorkerMode int

const (
	// SharedNothingMode - Worker has NO filesystem access to coordinator.
	// Uses RemoteTaskHandler that pushes status and streams logs to coordinator via gRPC.
	SharedNothingMode WorkerMode = iota
)

// =============================================================================
// Worker Setup Helpers
// =============================================================================

// setupSharedNothingWorker creates and starts a worker with RemoteTaskHandler.
// This worker runs tasks in-process and pushes status/logs to the coordinator via gRPC.
// Use this for shared-nothing mode where worker has no filesystem access.
func setupSharedNothingWorker(t *testing.T, coord *test.Coordinator, workerID string, labels map[string]string) *worker.Worker {
	t.Helper()
	coordinatorClient := coord.GetCoordinatorClient(t)

	handlerCfg := worker.RemoteTaskHandlerConfig{
		WorkerID:          workerID,
		CoordinatorClient: coordinatorClient,
		DAGRunStore:       nil, // No local store in shared-nothing mode
		DAGStore:          coord.DAGStore,
		DAGRunMgr:         coord.DAGRunMgr,
		ServiceRegistry:   coord.ServiceRegistry,
		PeerConfig:        coord.Config.Core.Peer,
		Config:            coord.Config,
	}

	workerInst := worker.NewWorker(workerID, 10, coordinatorClient, labels, coord.Config)
	workerInst.SetHandler(worker.NewRemoteTaskHandler(handlerCfg))

	return startAndCleanupWorkerWithID(t, coord, workerInst, workerID)
}

// setupWorker creates a worker with the specified mode.
func setupWorker(t *testing.T, coord *test.Coordinator, workerID string, mode WorkerMode, labels map[string]string) *worker.Worker {
	t.Helper()
	switch mode {
	case SharedNothingMode:
		return setupSharedNothingWorker(t, coord, workerID, labels)
	default:
		t.Fatalf("unknown worker mode: %d", mode)
		return nil
	}
}

// setupWorkers creates and starts multiple workers with the specified mode.
func setupWorkers(t *testing.T, coord *test.Coordinator, count int, mode WorkerMode, labels map[string]string) []*worker.Worker {
	t.Helper()
	workers := make([]*worker.Worker, count)
	for i := range count {
		workers[i] = setupWorker(t, coord, fmt.Sprintf("test-worker-%d", i+1), mode, labels)
	}
	return workers
}

// startAndCleanupWorkerWithID starts a worker and registers cleanup.
// It waits for the worker to register with the coordinator before returning.
func startAndCleanupWorkerWithID(t *testing.T, coord *test.Coordinator, workerInst *worker.Worker, workerID string) *worker.Worker {
	t.Helper()

	go func() {
		if err := workerInst.Start(coord.Context); err != nil {
			t.Logf("Worker stopped: %v", err)
		}
	}()

	// Wait for the worker to register with the coordinator
	waitForWorkerRegistration(t, coord, workerID, 5*time.Second)

	t.Cleanup(func() {
		if err := workerInst.Stop(coord.Context); err != nil {
			t.Logf("Error stopping worker: %v", err)
		}
	})

	return workerInst
}

// waitForWorkerRegistration waits for a worker to register with the coordinator.
func waitForWorkerRegistration(t *testing.T, coord *test.Coordinator, workerID string, timeout time.Duration) {
	t.Helper()
	coordinatorClient := coord.GetCoordinatorClient(t)
	require.Eventually(t, func() bool {
		workers, err := coordinatorClient.GetWorkers(coord.Context)
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.WorkerId == workerID {
				return true
			}
		}
		return false
	}, timeout, 50*time.Millisecond, "worker %s should register with coordinator", workerID)
}

// =============================================================================
// Scheduler Setup Helpers
// =============================================================================

// setupScheduler creates and configures a scheduler instance for distributed execution.
func setupScheduler(t *testing.T, coord *test.Coordinator, coordinatorClient execution.Dispatcher) *scheduler.Scheduler {
	t.Helper()

	de := scheduler.NewDAGExecutor(coordinatorClient, runtime.NewSubCmdBuilder(coord.Config))
	em := scheduler.NewEntryReader(coord.Config.Paths.DAGsDir, coord.DAGStore, coord.DAGRunMgr, de, "")

	schedulerInst, err := scheduler.New(
		coord.Config,
		em,
		coord.DAGRunMgr,
		coord.DAGRunStore,
		coord.QueueStore,
		coord.ProcStore,
		coord.ServiceRegistry,
		coordinatorClient,
	)
	require.NoError(t, err, "failed to create scheduler")

	return schedulerInst
}

// startScheduler starts the scheduler in a goroutine and returns a done channel.
func startScheduler(t *testing.T, ctx context.Context, schedulerInst *scheduler.Scheduler) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- schedulerInst.Start(ctx)
	}()
	return done
}

// =============================================================================
// Status Wait Helpers
// =============================================================================

// waitForStatus waits for DAG to reach expected status within timeout.
func waitForStatus(t *testing.T, coord *test.Coordinator, dag *core.DAG, expected core.Status, timeout time.Duration) execution.DAGRunStatus {
	t.Helper()
	var status execution.DAGRunStatus
	require.Eventually(t, func() bool {
		var err error
		status, err = coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
		if err != nil {
			return false
		}
		t.Logf("Current status: %s (waiting for %s)", status.Status, expected)
		return status.Status == expected
	}, timeout, 100*time.Millisecond, "timeout waiting for status %s", expected)
	return status
}

// waitForStatusIn waits for DAG to reach any of the expected statuses within timeout.
func waitForStatusIn(t *testing.T, coord *test.Coordinator, dag *core.DAG, expected []core.Status, timeout time.Duration) execution.DAGRunStatus {
	t.Helper()
	var status execution.DAGRunStatus
	require.Eventually(t, func() bool {
		var err error
		status, err = coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
		if err != nil {
			return false
		}
		for _, exp := range expected {
			if status.Status == exp {
				return true
			}
		}
		return false
	}, timeout, 100*time.Millisecond, "timeout waiting for status in %v", expected)
	return status
}

// waitForRunning waits for DAG to reach Running status.
func waitForRunning(t *testing.T, coord *test.Coordinator, dag *core.DAG, timeout time.Duration) execution.DAGRunStatus {
	t.Helper()
	return waitForStatus(t, coord, dag, core.Running, timeout)
}

// waitForSucceeded waits for DAG to reach Succeeded status.
func waitForSucceeded(t *testing.T, coord *test.Coordinator, dag *core.DAG, timeout time.Duration) execution.DAGRunStatus {
	t.Helper()
	return waitForStatus(t, coord, dag, core.Succeeded, timeout)
}

// waitForFailed waits for DAG to reach Failed status.
func waitForFailed(t *testing.T, coord *test.Coordinator, dag *core.DAG, timeout time.Duration) execution.DAGRunStatus {
	t.Helper()
	return waitForStatus(t, coord, dag, core.Failed, timeout)
}

// waitForQueueEmpty waits for the queue to be empty.
func waitForQueueEmpty(t *testing.T, coord *test.Coordinator, dag *test.DAG, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		items, err := coord.QueueStore.ListByDAGName(coord.Context, dag.ProcGroup(), dag.Name)
		return err == nil && len(items) == 0
	}, timeout, 100*time.Millisecond, "timeout waiting for queue to be empty")
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// assertFinalStatus asserts that the DAG has the expected final status.
func assertFinalStatus(t *testing.T, coord *test.Coordinator, dag *core.DAG, expected core.Status) {
	t.Helper()
	status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
	require.NoError(t, err)
	require.Equal(t, expected, status.Status, "unexpected final status")
}

// assertNodeStatus asserts that a specific node has the expected status.
func assertNodeStatus(t *testing.T, status execution.DAGRunStatus, stepName string, expected core.NodeStatus) {
	t.Helper()
	for _, node := range status.Nodes {
		if node.Step.Name == stepName {
			require.Equal(t, expected, node.Status, "unexpected status for step %s", stepName)
			return
		}
	}
	t.Fatalf("step %s not found in status", stepName)
}

// assertAllNodesSucceeded asserts that all nodes in the DAG succeeded.
func assertAllNodesSucceeded(t *testing.T, status execution.DAGRunStatus) {
	t.Helper()
	for _, node := range status.Nodes {
		require.Equal(t, core.NodeSucceeded, node.Status, "step %s should have succeeded", node.Step.Name)
	}
}

// assertWorkerID asserts that the status has the expected worker ID.
func assertWorkerID(t *testing.T, status execution.DAGRunStatus, expected string) {
	t.Helper()
	require.Equal(t, expected, status.WorkerID, "unexpected worker ID")
}

// =============================================================================
// Log File Helpers
// =============================================================================

// findLogFiles finds all log files matching the pattern in logDir.
func findLogFiles(t *testing.T, logDir, dagName, dagRunID, stepName, suffix string) []string {
	t.Helper()

	baseDir := filepath.Join(logDir, dagName, dagRunID)
	var matches []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			filename := fmt.Sprintf("%s.%s.log", stepName, suffix)
			if filepath.Base(path) == filename {
				matches = append(matches, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Logf("Error walking log directory: %v", err)
	}

	return matches
}

// assertLogContains verifies log file exists and contains expected content.
func assertLogContains(t *testing.T, logDir, dagName, dagRunID, stepName, expected string) {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)

	content, err := os.ReadFile(matches[0])
	require.NoError(t, err, "failed to read log file %s", matches[0])
	assert.Contains(t, string(content), expected, "log file should contain expected content")
}

// assertLogExists verifies that a log file exists for the given step.
func assertLogExists(t *testing.T, logDir, dagName, dagRunID, stepName string) string {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)
	return matches[0]
}

// getLogContent reads and returns the content of a log file.
func getLogContent(t *testing.T, logPath string) string {
	t.Helper()
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "failed to read log file")
	return string(content)
}

// =============================================================================
// Command Execution Helpers
// =============================================================================

// executeStartCommand runs the dagu start command for a DAG.
func executeStartCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	startSpec := subCmdBuilder.Start(dag, runtime.StartOptions{Quiet: true})
	return runtime.Start(coord.Context, startSpec)
}

// executeRetryCommand runs the dagu retry command for a DAG run.
func executeRetryCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG, dagRunID string) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	retrySpec := subCmdBuilder.Retry(dag, dagRunID, "")
	return runtime.Start(coord.Context, retrySpec)
}

// executeEnqueueCommand runs the dagu enqueue command for a DAG.
func executeEnqueueCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	enqueueSpec := subCmdBuilder.Enqueue(dag, runtime.EnqueueOptions{Quiet: true})
	return runtime.Start(coord.Context, enqueueSpec)
}
