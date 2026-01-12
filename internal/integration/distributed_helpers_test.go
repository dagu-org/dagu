package integration_test

import (
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

// Helper functions shared across distributed integration tests

// setupRemoteWorker creates and starts a worker with remoteTaskHandler for shared-nothing mode.
// This worker runs tasks in-process with status pushing and log streaming to the coordinator.
func setupRemoteWorker(t *testing.T, coord *test.Coordinator, workerID string, maxActiveRuns int, labels map[string]string) *worker.Worker {
	t.Helper()
	coordinatorClient := coord.GetCoordinatorClient(t)

	handlerCfg := worker.RemoteTaskHandlerConfig{
		WorkerID:          workerID,
		CoordinatorClient: coordinatorClient,
		DAGRunStore:       nil,
		DAGStore:          coord.DAGStore,
		DAGRunMgr:         coord.DAGRunMgr,
		ServiceRegistry:   coord.ServiceRegistry,
		PeerConfig:        coord.Config.Core.Peer,
		Config:            coord.Config,
	}

	workerInst := worker.NewWorker(workerID, maxActiveRuns, coordinatorClient, labels, coord.Config)
	workerInst.SetHandler(worker.NewRemoteTaskHandler(handlerCfg))

	return startAndCleanupWorker(t, coord, workerInst)
}

// startAndCleanupWorker starts a worker and registers cleanup.
func startAndCleanupWorker(t *testing.T, coord *test.Coordinator, workerInst *worker.Worker) *worker.Worker {
	t.Helper()

	go func() {
		if err := workerInst.Start(coord.Context); err != nil {
			t.Logf("Worker stopped: %v", err)
		}
	}()

	t.Cleanup(func() {
		if err := workerInst.Stop(coord.Context); err != nil {
			t.Logf("Error stopping worker: %v", err)
		}
	})

	return workerInst
}

// setupRemoteWorkers creates and starts multiple workers with remoteTaskHandler.
// This is the correct choice for shared-nothing mode tests where workers need to
// push status and stream logs to the coordinator.
func setupRemoteWorkers(t *testing.T, coord *test.Coordinator, count int, labels map[string]string) []*worker.Worker {
	t.Helper()
	workers := make([]*worker.Worker, count)
	for i := range count {
		workers[i] = setupRemoteWorker(t, coord, fmt.Sprintf("test-worker-%d", i+1), 10, labels)
	}
	return workers
}

// setupSchedulerWithCoordinator creates and configures a scheduler instance
// that works with the coordinator for distributed execution.
func setupSchedulerWithCoordinator(t *testing.T, coord *test.Coordinator, coordinatorClient execution.Dispatcher) *scheduler.Scheduler {
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

// waitForStatus waits for DAG to reach expected status within timeout.
// Returns the final status when reached, or fails the test on timeout.
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

// findLogFiles finds all log files matching the pattern in logDir.
// Pattern: {logDir}/{dagName}/{dagRunID}/**/{stepName}.{suffix}.log
func findLogFiles(t *testing.T, logDir, dagName, dagRunID, stepName, suffix string) []string {
	t.Helper()

	// Build search pattern - need to handle nested attemptID directories
	baseDir := filepath.Join(logDir, dagName, dagRunID)
	var matches []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors, directory might not exist yet
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

// assertLogFileContains verifies log file exists and contains expected content.
func assertLogFileContains(t *testing.T, logDir, dagName, dagRunID, stepName, expected string) {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)

	content, err := os.ReadFile(matches[0])
	require.NoError(t, err, "failed to read log file %s", matches[0])
	assert.Contains(t, string(content), expected, "log file should contain expected content")
}

// assertLogFileExists verifies that a log file exists for the given step.
func assertLogFileExists(t *testing.T, logDir, dagName, dagRunID, stepName string) string {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)
	return matches[0]
}

// getLogFileContent reads and returns the content of a log file.
func getLogFileContent(t *testing.T, logPath string) string {
	t.Helper()
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "failed to read log file")
	return string(content)
}
