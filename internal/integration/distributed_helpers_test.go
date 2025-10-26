package integration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// Helper functions shared across distributed integration tests

// setupWorker creates and starts a single worker with the given labels.
// It automatically stops the worker when the test completes.
func setupWorker(t *testing.T, coord *test.Coordinator, workerID string, maxActiveRuns int, labels map[string]string) *worker.Worker {
	t.Helper()
	coordinatorClient := coord.GetCoordinatorClient(t)

	workerInst := worker.NewWorker(workerID, maxActiveRuns, coordinatorClient, labels, coord.Config)

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

	// Give worker time to connect
	time.Sleep(100 * time.Millisecond)

	return workerInst
}

// setupWorkers creates and starts multiple workers with the given labels.
// All workers are automatically stopped when the test completes.
func setupWorkers(t *testing.T, coord *test.Coordinator, count int, labels map[string]string) []*worker.Worker {
	t.Helper()
	coordinatorClient := coord.GetCoordinatorClient(t)
	workers := make([]*worker.Worker, count)

	for i := range count {
		workerInst := worker.NewWorker(
			fmt.Sprintf("test-worker-%d", i+1),
			10,
			coordinatorClient,
			labels,
			coord.Config,
		)
		workers[i] = workerInst

		go func(w *worker.Worker) {
			if err := w.Start(coord.Context); err != nil {
				t.Logf("Worker stopped: %v", err)
			}
		}(workerInst)

		t.Cleanup(func() {
			if err := workerInst.Stop(coord.Context); err != nil {
				t.Logf("Error stopping worker: %v", err)
			}
		})
	}

	// Give workers time to connect
	time.Sleep(50 * time.Millisecond)

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
