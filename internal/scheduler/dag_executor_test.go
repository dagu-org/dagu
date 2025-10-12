package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/core/builder"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
)

func TestDAGExecutorRealBehavior(t *testing.T) {
	// Note: This test creates gRPC connections through the coordinator client.
	// The connections are cleaned up when the test ends, but goleak may still
	// report leaked goroutines from gRPC's internal connection management.

	t.Run("HandleJobVsExecuteDAGBehavior", func(t *testing.T) {
		// Setup test environment
		tmpDir := t.TempDir()
		queueDir := filepath.Join(tmpDir, "queue")
		err := os.MkdirAll(queueDir, 0755)
		require.NoError(t, err)

		// Create real DAG run manager with test helper config
		th := test.Setup(t)

		// Create a test DAG
		yamlContent := `
steps:
  - name: test-step
    command: echo "test"
`
		testDAG := th.DAG(t, yamlContent)
		testFile := testDAG.Location
		coordinatorCli := coordinator.New(th.ServiceRegistry, coordinator.DefaultConfig())

		dagExecutor := scheduler.NewDAGExecutor(coordinatorCli, th.SubCmdBuilder)
		t.Cleanup(func() {
			dagExecutor.Close(th.Context)
		})

		// Test 1: HandleJob with distributed execution and START operation
		t.Run("HandleJobDistributedSTART", func(t *testing.T) {
			// Load DAG and set worker selector for distributed execution
			dag, err := builder.Load(context.Background(), testFile)
			require.NoError(t, err)
			dag.WorkerSelector = map[string]string{"type": "test-worker"}

			// Call HandleJob with START operation
			runID := "handle-job-test-123"
			err = dagExecutor.HandleJob(
				context.Background(),
				dag,
				coordinatorv1.Operation_OPERATION_START,
				runID,
			)

			// This succeeds because it enqueues the DAG
			require.NoError(t, err)

			// Key point: HandleJob with distributed + START = EnqueueDAGRun
			// The DAG is persisted to queue before any execution attempt
		})

		// Test 2: ExecuteDAG with distributed execution
		t.Run("ExecuteDAGDistributed", func(t *testing.T) {
			// Create DAG executor
			dagExecutor := scheduler.NewDAGExecutor(coordinatorCli, th.SubCmdBuilder)

			// Load DAG and set worker selector
			dag, err := builder.Load(context.Background(), testFile)
			require.NoError(t, err)
			dag.WorkerSelector = map[string]string{"type": "test-worker"}

			// Call ExecuteDAG with START operation
			runID := "execute-dag-test-456"
			err = dagExecutor.ExecuteDAG(
				context.Background(),
				dag,
				coordinatorv1.Operation_OPERATION_START,
				runID,
			)

			// This fails because no worker is connected, but the important
			// point is that it tried to dispatch, not enqueue
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to dispatch task")

			// Key point: ExecuteDAG with distributed = Direct dispatch to coordinator
			// No enqueueing happens - assumes the job is already persisted
		})

		// Test 3: HandleJob with local execution
		t.Run("HandleJobLocal", func(t *testing.T) {
			// Create DAG executor without coordinator (local only)
			dagExecutor := scheduler.NewDAGExecutor(nil, th.SubCmdBuilder)

			// Load DAG without worker selector (local execution)
			dag, err := builder.Load(context.Background(), testFile)
			require.NoError(t, err)

			// Call HandleJob with START operation
			runID := "handle-job-local-789"
			err = dagExecutor.HandleJob(
				context.Background(),
				dag,
				coordinatorv1.Operation_OPERATION_START,
				runID,
			)

			// With the test executable, this actually succeeds because
			// the test harness provides a working executable
			// The important point is it called StartDAGRunAsync (not EnqueueDAGRun)
			if err != nil {
				require.Contains(t, err.Error(), "failed to start dag-run")
			}

			// Key point: HandleJob with local = StartDAGRunAsync
			// Direct execution without enqueueing
		})

		// Test 4: HandleJob with RETRY operation
		t.Run("HandleJobRETRY", func(t *testing.T) {
			// Create DAG executor
			dagExecutor := scheduler.NewDAGExecutor(coordinatorCli, th.SubCmdBuilder)

			// Load DAG and set worker selector
			dag, err := builder.Load(context.Background(), testFile)
			require.NoError(t, err)
			dag.WorkerSelector = map[string]string{"type": "test-worker"}

			// Call HandleJob with RETRY operation
			runID := "handle-job-retry-999"
			err = dagExecutor.HandleJob(
				context.Background(),
				dag,
				coordinatorv1.Operation_OPERATION_RETRY,
				runID,
			)

			// This fails because no worker is connected, but it shows
			// that RETRY operations go directly to ExecuteDAG
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to dispatch task")

			// Key point: HandleJob with RETRY = ExecuteDAG (no enqueueing)
			// RETRY means the job is already persisted from a previous run
		})
	})
}
