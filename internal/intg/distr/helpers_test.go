package distr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type WorkerMode int

const (
	SharedNothingMode WorkerMode = iota
)

func setupSharedNothingWorker(t *testing.T, coord *test.Coordinator, workerID string, labels map[string]string) *worker.Worker {
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

	workerInst := worker.NewWorker(workerID, 10, coordinatorClient, labels, coord.Config)
	workerInst.SetHandler(worker.NewRemoteTaskHandler(handlerCfg))

	return startAndCleanupWorkerWithID(t, coord, workerInst, workerID)
}

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

func setupWorkers(t *testing.T, coord *test.Coordinator, count int, mode WorkerMode, labels map[string]string) []*worker.Worker {
	t.Helper()
	workers := make([]*worker.Worker, count)
	for i := range count {
		workers[i] = setupWorker(t, coord, fmt.Sprintf("test-worker-%d", i+1), mode, labels)
	}
	return workers
}

func startAndCleanupWorkerWithID(t *testing.T, coord *test.Coordinator, workerInst *worker.Worker, workerID string) *worker.Worker {
	t.Helper()

	go func() {
		if err := workerInst.Start(coord.Context); err != nil {
			t.Logf("Worker stopped: %v", err)
		}
	}()

	waitForWorkerRegistration(t, coord, workerID, 5*time.Second)

	t.Cleanup(func() {
		if err := workerInst.Stop(coord.Context); err != nil {
			t.Logf("Error stopping worker: %v", err)
		}
	})

	return workerInst
}

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

func setupScheduler(t *testing.T, coord *test.Coordinator, coordinatorClient exec.Dispatcher) *scheduler.Scheduler {
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

func waitForStatus(t *testing.T, coord *test.Coordinator, dag *core.DAG, expected core.Status, timeout time.Duration) exec.DAGRunStatus {
	t.Helper()
	var status exec.DAGRunStatus
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

func waitForStatusIn(t *testing.T, coord *test.Coordinator, dag *core.DAG, expected []core.Status, timeout time.Duration) exec.DAGRunStatus {
	t.Helper()
	var status exec.DAGRunStatus
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

func waitForRunning(t *testing.T, coord *test.Coordinator, dag *core.DAG, timeout time.Duration) exec.DAGRunStatus {
	t.Helper()
	return waitForStatus(t, coord, dag, core.Running, timeout)
}

func waitForSucceeded(t *testing.T, coord *test.Coordinator, dag *core.DAG, timeout time.Duration) exec.DAGRunStatus {
	t.Helper()
	return waitForStatus(t, coord, dag, core.Succeeded, timeout)
}

func assertAllNodesSucceeded(t *testing.T, status exec.DAGRunStatus) {
	t.Helper()
	for _, node := range status.Nodes {
		require.Equal(t, core.NodeSucceeded, node.Status, "step %s should have succeeded", node.Step.Name)
	}
}

func assertWorkerID(t *testing.T, status exec.DAGRunStatus, expected string) {
	t.Helper()
	require.Equal(t, expected, status.WorkerID, "unexpected worker ID")
}

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

func assertLogContains(t *testing.T, logDir, dagName, dagRunID, stepName, expected string) {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)

	content, err := os.ReadFile(matches[0])
	require.NoError(t, err, "failed to read log file %s", matches[0])
	assert.Contains(t, string(content), expected, "log file should contain expected content")
}

func assertLogExists(t *testing.T, logDir, dagName, dagRunID, stepName string) string {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)
	return matches[0]
}

func getLogContent(t *testing.T, logPath string) string {
	t.Helper()
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "failed to read log file")
	return string(content)
}

func executeStartCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	startSpec := subCmdBuilder.Start(dag, runtime.StartOptions{Quiet: true})
	return runtime.Start(coord.Context, startSpec)
}

func executeRetryCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG, dagRunID string) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	retrySpec := subCmdBuilder.Retry(dag, dagRunID, "")
	return runtime.Start(coord.Context, retrySpec)
}

func executeEnqueueCommand(t *testing.T, coord *test.Coordinator, dag *core.DAG) error {
	t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	enqueueSpec := subCmdBuilder.Enqueue(dag, runtime.EnqueueOptions{Quiet: true})
	return runtime.Start(coord.Context, enqueueSpec)
}
