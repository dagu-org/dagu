package integration_test

import (
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

// TestQueueShellConfig tests that shell configuration is properly preserved
// when executing DAGs through the queue system via the REST API.
func TestQueueShellConfig(t *testing.T) {
	t.Run("EnqueueAPI_MultipleStepsChained", func(t *testing.T) {
		// Setup test helper with queues enabled
		th := test.Setup(t)
		th.Config.Queues.Enabled = true

		// Find available port for HTTP server
		port := findQueueTestPort(t)
		th.Config.Server = config.Server{
			Host: "localhost",
			Port: port,
			Permissions: map[config.Permission]bool{
				config.PermissionWriteDAGs: true,
				config.PermissionRunDAGs:   true,
			},
		}

		// Start the frontend server
		server := frontend.NewServer(
			th.Config,
			th.DAGStore,
			th.DAGRunStore,
			th.QueueStore,
			th.ProcStore,
			th.DAGRunMgr,
			nil, // no coordinator client for local execution
			th.ServiceRegistry,
			nil, // no metrics registry
		)

		go func() {
			_ = server.Serve(th.Context)
		}()

		// Wait for server to start
		waitForQueueServer(t, fmt.Sprintf("localhost:%d", port))

		// Create HTTP client
		client := resty.New()
		baseURL := fmt.Sprintf("http://localhost:%d", port)

		// Exact replication of user's DAG - NO workerSelector for local execution
		dagYAML := `
name: chained-steps-test
steps:
  - name: "1"
    command: "sleep 1"
    shell: "/bin/zsh"
`

		// Enqueue via REST API
		reqBody := map[string]interface{}{
			"spec": dagYAML,
		}

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(reqBody).
			Post(baseURL + "/api/v2/dag-runs/enqueue")

		require.NoError(t, err, "Enqueue API request should succeed")
		require.Equal(t, 200, resp.StatusCode(), "Enqueue API should return 200, got: %s", resp.String())

		t.Logf("Enqueue response: %s", resp.String())

		// Wait for DAG to be enqueued
		require.Eventually(t, func() bool {
			items, err := th.QueueStore.ListByDAGName(th.Context, "chained-steps-test", "chained-steps-test")
			return err == nil && len(items) == 1
		}, 3*time.Second, 100*time.Millisecond, "DAG should be enqueued")

		// Create DAGsDir for the scheduler's entry reader (even though we use REST API)
		err = os.MkdirAll(th.Config.Paths.DAGsDir, 0750)
		require.NoError(t, err, "failed to create DAGs directory")

		// Create and start scheduler for local execution
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
			nil, // no dispatcher for local execution
		)
		require.NoError(t, err, "failed to create scheduler")

		t.Log("Starting scheduler to process queue...")

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(th.Context)
		}()

		// Wait for execution to complete
		require.Eventually(t, func() bool {
			statuses, err := th.DAGRunStore.ListStatuses(th.Context)
			if err != nil || len(statuses) == 0 {
				return false
			}

			for _, status := range statuses {
				if status.Name == "chained-steps-test" {
					t.Logf("DAG status: %s", status.Status)

					if status.Status == core.Failed {
						for _, node := range status.Nodes {
							if node.Status == core.NodeFailed {
								t.Logf("Node %s failed: %s", node.Step.Name, node.Error)
							}
						}
						return true // Exit to check error
					}

					return status.Status == core.Succeeded
				}
			}
			return false
		}, 60*time.Second, 500*time.Millisecond, "Timeout waiting for DAG execution")

		// Stop scheduler
		schedulerInst.Stop(th.Context)

		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		// Verify final status
		att, err := th.DAGRunStore.LatestAttempt(th.Context, "chained-steps-test")
		require.NoError(t, err)

		status, err := att.ReadStatus(th.Context)
		require.NoError(t, err)

		require.Equal(t, core.Succeeded, status.Status,
			"DAG with 6 chained steps should succeed")

		t.Log("Test completed successfully!")
	})
}

// findQueueTestPort finds an available TCP port
func findQueueTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// waitForQueueServer waits for the server to be ready
func waitForQueueServer(t *testing.T, addr string) {
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
