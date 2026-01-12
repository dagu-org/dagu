package distr_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/frontend"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Queue Integration Tests
// =============================================================================
// These tests verify queue functionality including REST API enqueue and
// scheduler processing.

func TestQueue_EnqueueViaAPI(t *testing.T) {
	t.Run("enqueueViaRESTAPIAndProcess", func(t *testing.T) {
		th := test.Setup(t)
		th.Config.Queues.Enabled = true

		port := findAvailablePort(t)
		th.Config.Server = config.Server{
			Host: "localhost",
			Port: port,
			Permissions: map[config.Permission]bool{
				config.PermissionWriteDAGs: true,
				config.PermissionRunDAGs:   true,
			},
		}

		server, err := frontend.NewServer(
			th.Context,
			th.Config,
			th.DAGStore,
			th.DAGRunStore,
			th.QueueStore,
			th.ProcStore,
			th.DAGRunMgr,
			nil,
			th.ServiceRegistry,
			nil,
			nil,
			nil,
		)
		require.NoError(t, err, "failed to create server")

		go func() {
			_ = server.Serve(th.Context)
		}()

		waitForServer(t, fmt.Sprintf("localhost:%d", port))

		client := resty.New()
		baseURL := fmt.Sprintf("http://localhost:%d", port)

		dagYAML := `
name: api-enqueue-test
steps:
  - name: "task1"
    command: "echo hello"
`
		reqBody := map[string]any{
			"spec": dagYAML,
		}

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(reqBody).
			Post(baseURL + "/api/v2/dag-runs/enqueue")

		require.NoError(t, err, "Enqueue API request should succeed")
		require.Equal(t, 200, resp.StatusCode(), "Enqueue API should return 200, got: %s", resp.String())

		require.Eventually(t, func() bool {
			items, err := th.QueueStore.ListByDAGName(th.Context, "api-enqueue-test", "api-enqueue-test")
			return err == nil && len(items) == 1
		}, 3*time.Second, 100*time.Millisecond, "DAG should be enqueued")

		err = os.MkdirAll(th.Config.Paths.DAGsDir, 0750)
		require.NoError(t, err, "failed to create DAGs directory")

		de := scheduler.NewDAGExecutor(nil, runtime.NewSubCmdBuilder(th.Config))
		entryReader := scheduler.NewEntryReader(th.Config.Paths.DAGsDir, th.DAGStore, th.DAGRunMgr, de, "")
		schedulerInst, err := scheduler.New(
			th.Config,
			entryReader,
			th.DAGRunMgr,
			th.DAGRunStore,
			th.QueueStore,
			th.ProcStore,
			th.ServiceRegistry,
			nil,
		)
		require.NoError(t, err, "failed to create scheduler")

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(th.Context)
		}()

		require.Eventually(t, func() bool {
			statuses, err := th.DAGRunStore.ListStatuses(th.Context)
			if err != nil || len(statuses) == 0 {
				return false
			}

			for _, status := range statuses {
				if status.Name == "api-enqueue-test" {
					return status.Status == core.Succeeded
				}
			}
			return false
		}, 30*time.Second, 500*time.Millisecond, "Timeout waiting for DAG execution")

		schedulerInst.Stop(th.Context)

		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		att, err := th.DAGRunStore.LatestAttempt(th.Context, "api-enqueue-test")
		require.NoError(t, err)

		status, err := att.ReadStatus(th.Context)
		require.NoError(t, err)

		require.Equal(t, core.Succeeded, status.Status)
	})
}

func TestQueue_EnqueueViaCommand(t *testing.T) {
	t.Run("enqueueViaCommandAndProcess", func(t *testing.T) {
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, `
name: cmd-enqueue-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: echo "enqueued via command"
`)

		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "enqueue should succeed")

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
	})
}

func TestQueue_CleanupOnSuccess(t *testing.T) {
	t.Run("queueItemRemovedAfterSuccess", func(t *testing.T) {
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, `
name: queue-cleanup-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: echo "done"
`)

		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for both success and queue cleanup
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil || status.Status != core.Succeeded {
				return false
			}

			items, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(items) == 0
		}, 25*time.Second, 200*time.Millisecond, "Queue should be empty after success")

		schedulerCancel()
	})
}

func TestQueue_SchedulerProcessing(t *testing.T) {
	t.Run("schedulerPicksUpQueuedDAG", func(t *testing.T) {
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"env": "prod"})

		dagWrapper := coord.DAG(t, `
name: scheduler-process-test
workerSelector:
  env: prod
steps:
  - name: step1
    command: echo "step1"
  - name: step2
    command: echo "step2"
    depends: [step1]
`)

		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		latest, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Queued, latest.Status, "DAG should be in queued state before scheduler starts")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		assertAllNodesSucceeded(t, status)
	})
}

// findAvailablePort finds an available TCP port
func findAvailablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// waitForServer waits for the server to be ready
func waitForServer(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 20; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Server at %s failed to start", addr)
}
