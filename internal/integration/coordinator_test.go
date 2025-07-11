package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCoordinatorGetWorkers(t *testing.T) {
	t.Run("GetWorkersE2E", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create gRPC client to coordinator
		conn, err := grpc.NewClient(
			coord.Address(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				t.Logf("Failed to close connection: %v", err)
			}
		}()

		client := coordinatorv1.NewCoordinatorServiceClient(conn)

		// Initially should have no workers
		resp, err := client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Empty(t, resp.Workers)

		// Send heartbeats from two workers
		_, err = client.Heartbeat(context.Background(), &coordinatorv1.HeartbeatRequest{
			WorkerId: "test-worker-1",
			Labels:   map[string]string{"type": "compute", "region": "us-east"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 2,
				BusyPollers:  0,
			},
		})
		require.NoError(t, err)

		_, err = client.Heartbeat(context.Background(), &coordinatorv1.HeartbeatRequest{
			WorkerId: "test-worker-2",
			Labels:   map[string]string{"type": "storage", "region": "us-west"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 3,
				BusyPollers:  1,
				RunningTasks: []*coordinatorv1.RunningTask{
					{
						DagRunId:  "run-456",
						DagName:   "backup.yaml",
						StartedAt: time.Now().Unix(),
					},
				},
			},
		})
		require.NoError(t, err)

		// Get workers list
		resp, err = client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 2) // 2 workers

		// Verify worker details
		workerMap := make(map[string]*coordinatorv1.WorkerInfo)
		for _, w := range resp.Workers {
			workerMap[w.WorkerId] = w
		}

		// Check worker 1
		w1, ok := workerMap["test-worker-1"]
		require.True(t, ok)
		require.Equal(t, map[string]string{"type": "compute", "region": "us-east"}, w1.Labels)
		require.Equal(t, int32(2), w1.TotalPollers)
		require.Equal(t, int32(0), w1.BusyPollers)
		require.Empty(t, w1.RunningTasks)

		// Check worker 2
		w2, ok := workerMap["test-worker-2"]
		require.True(t, ok)
		require.Equal(t, map[string]string{"type": "storage", "region": "us-west"}, w2.Labels)
		require.Equal(t, int32(3), w2.TotalPollers)
		require.Equal(t, int32(1), w2.BusyPollers)
		require.Len(t, w2.RunningTasks, 1)
		require.Equal(t, "run-456", w2.RunningTasks[0].DagRunId)

		// Wait for heartbeats to become stale (>30 seconds)
		// In a real test we wouldn't wait this long, but for now we'll
		// just verify that the workers are still there
		resp, err = client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 2)
	})
}

func TestCoordinatorHeartbeat(t *testing.T) {
	t.Run("HeartbeatE2E", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create gRPC client to coordinator
		conn, err := grpc.NewClient(
			coord.Address(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				t.Logf("Failed to close connection: %v", err)
			}
		}()

		client := coordinatorv1.NewCoordinatorServiceClient(conn)

		// Initially no workers
		resp, err := client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Empty(t, resp.Workers)

		// Send heartbeat from worker
		_, err = client.Heartbeat(context.Background(), &coordinatorv1.HeartbeatRequest{
			WorkerId: "test-worker-1",
			Labels:   map[string]string{"type": "compute", "region": "us-east"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 10,
				BusyPollers:  3,
				RunningTasks: []*coordinatorv1.RunningTask{
					{
						DagRunId:  "run-123",
						DagName:   "etl-pipeline.yaml",
						StartedAt: time.Now().Unix(),
					},
				},
			},
		})
		require.NoError(t, err)

		// Get workers should now show the worker
		resp, err = client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)

		worker := resp.Workers[0]
		require.Equal(t, "test-worker-1", worker.WorkerId)
		require.Equal(t, map[string]string{"type": "compute", "region": "us-east"}, worker.Labels)
		require.Equal(t, int32(10), worker.TotalPollers)
		require.Equal(t, int32(3), worker.BusyPollers)
		require.Len(t, worker.RunningTasks, 1)
		require.Equal(t, "run-123", worker.RunningTasks[0].DagRunId)
		require.Greater(t, worker.LastHeartbeatAt, int64(0))

		// Send another heartbeat with updated stats
		_, err = client.Heartbeat(context.Background(), &coordinatorv1.HeartbeatRequest{
			WorkerId: "test-worker-1",
			Labels:   map[string]string{"type": "compute", "region": "us-east"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 10,
				BusyPollers:  0, // All tasks completed
				RunningTasks: []*coordinatorv1.RunningTask{},
			},
		})
		require.NoError(t, err)

		// Verify updated stats
		resp, err = client.GetWorkers(context.Background(), &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)
		require.Equal(t, int32(0), resp.Workers[0].BusyPollers)
		require.Empty(t, resp.Workers[0].RunningTasks)
	})
}
