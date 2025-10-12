package coordinator_test

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func parsePort(addr string) int {
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		return 0
	}
	port, _ := strconv.Atoi(parts[1])
	return port
}

func TestClientNew(t *testing.T) {
	config := coordinator.DefaultConfig()
	monitor := &mockServiceMonitor{}

	client := coordinator.New(monitor, config)
	require.NotNil(t, client)

	// Check initial metrics
	metrics := client.Metrics()
	assert.True(t, metrics.IsConnected)
	assert.Equal(t, 0, metrics.ConsecutiveFails)
	assert.Equal(t, 0, metrics.FailCount)
	assert.Nil(t, metrics.LastError)
}

func TestClientDispatch(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := coordinator.DefaultConfig()
		config.MaxRetries = 0
		config.RequestTimeout = 100 * time.Millisecond

		mockCoord := &mockCoordinatorService{
			dispatchFunc: func(_ context.Context, req *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
				assert.Equal(t, "test-dag-run", req.Task.DagRunId)
				assert.Equal(t, "test.yaml", req.Task.Target)
				return &coordinatorv1.DispatchResponse{}, nil
			},
		}

		server, addr := startMockServer(t, mockCoord)
		defer server.Stop()

		monitor := &mockServiceMonitor{
			members: []execution.HostInfo{
				{ID: "coord-1", Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive},
			},
		}

		client := coordinator.New(monitor, config)

		task := &coordinatorv1.Task{
			DagRunId: "test-dag-run",
			Target:   "test.yaml",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := client.Dispatch(ctx, task)
		require.NoError(t, err)
	})

	t.Run("NoCoordinators", func(t *testing.T) {
		config := coordinator.DefaultConfig()
		config.MaxRetries = 0
		config.RequestTimeout = 100 * time.Millisecond

		monitor := &mockServiceMonitor{
			members: []execution.HostInfo{}, // No coordinators
		}

		client := coordinator.New(monitor, config)

		task := &coordinatorv1.Task{
			DagRunId: "test-dag-run",
			Target:   "test.yaml",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := client.Dispatch(ctx, task)
		require.Error(t, err)
		// Could be either error depending on timing
		assert.True(t, strings.Contains(err.Error(), "no coordinator instances available") ||
			strings.Contains(err.Error(), "context deadline exceeded"))
	})
}

func TestClientPoll(t *testing.T) {
	config := coordinator.DefaultConfig()
	config.RequestTimeout = 100 * time.Millisecond

	expectedTask := &coordinatorv1.Task{
		DagRunId:  "test-dag-run",
		Target:    "test.yaml",
		Operation: coordinatorv1.Operation_OPERATION_START,
	}

	mockCoord := &mockCoordinatorService{
		pollFunc: func(_ context.Context, req *coordinatorv1.PollRequest) (*coordinatorv1.PollResponse, error) {
			assert.Equal(t, "test-worker", req.WorkerId)
			return &coordinatorv1.PollResponse{Task: expectedTask}, nil
		},
	}

	server, addr := startMockServer(t, mockCoord)
	defer server.Stop()

	monitor := &mockServiceMonitor{
		members: []execution.HostInfo{{Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive}},
	}

	client := coordinator.New(monitor, config)

	req := &coordinatorv1.PollRequest{
		WorkerId: "test-worker",
		Labels:   map[string]string{"type": "test"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	policy := backoff.NewConstantBackoffPolicy(10 * time.Millisecond)
	task, err := client.Poll(ctx, policy, req)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, expectedTask.DagRunId, task.DagRunId)
}

func TestClientGetWorkers(t *testing.T) {
	config := coordinator.DefaultConfig()
	config.RequestTimeout = 100 * time.Millisecond

	expectedWorkers := []*coordinatorv1.WorkerInfo{
		{
			WorkerId:     "worker-1",
			TotalPollers: 5,
		},
		{
			WorkerId:     "worker-2",
			TotalPollers: 3,
		},
	}

	mockCoord := &mockCoordinatorService{
		getWorkersFunc: func(_ context.Context, _ *coordinatorv1.GetWorkersRequest) (*coordinatorv1.GetWorkersResponse, error) {
			return &coordinatorv1.GetWorkersResponse{Workers: expectedWorkers}, nil
		},
	}

	server, addr := startMockServer(t, mockCoord)
	defer server.Stop()

	monitor := &mockServiceMonitor{
		members: []execution.HostInfo{{Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive}},
	}

	client := coordinator.New(monitor, config)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	workers, err := client.GetWorkers(ctx)
	require.NoError(t, err)
	assert.Len(t, workers, 2)
	assert.Equal(t, "worker-1", workers[0].WorkerId)
	assert.Equal(t, "worker-2", workers[1].WorkerId)
}

func TestClientHeartbeat(t *testing.T) {
	config := coordinator.DefaultConfig()
	config.RequestTimeout = 100 * time.Millisecond

	var receivedReq *coordinatorv1.HeartbeatRequest
	mockCoord := &mockCoordinatorService{
		heartbeatFunc: func(_ context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
			receivedReq = req
			return &coordinatorv1.HeartbeatResponse{}, nil
		},
	}

	server, addr := startMockServer(t, mockCoord)
	defer server.Stop()

	monitor := &mockServiceMonitor{
		members: []execution.HostInfo{{Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive}},
	}

	client := coordinator.New(monitor, config)

	req := &coordinatorv1.HeartbeatRequest{
		WorkerId: "test-worker",
		Stats: &coordinatorv1.WorkerStats{
			TotalPollers: 5,
			BusyPollers:  2,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := client.Heartbeat(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, receivedReq)
	assert.Equal(t, "test-worker", receivedReq.WorkerId)
	assert.NotNil(t, receivedReq.Stats)
	assert.Equal(t, int32(5), receivedReq.Stats.TotalPollers)
	assert.Equal(t, int32(2), receivedReq.Stats.BusyPollers)
}

func TestClientMetrics(t *testing.T) {
	// Test metrics tracking during failures
	config := coordinator.DefaultConfig()
	config.MaxRetries = 0 // No retries
	config.RequestTimeout = 100 * time.Millisecond

	// Create a failing coordinator
	mockCoord := &mockCoordinatorService{
		dispatchFunc: func(_ context.Context, _ *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
			return nil, status.Error(codes.Unavailable, "service unavailable")
		},
	}

	server, addr := startMockServer(t, mockCoord)
	defer server.Stop()

	monitor := &mockServiceMonitor{
		members: []execution.HostInfo{{Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive}},
	}

	client := coordinator.New(monitor, config)

	// Initial state
	metrics := client.Metrics()
	assert.True(t, metrics.IsConnected)
	assert.Equal(t, 0, metrics.ConsecutiveFails)

	task := &coordinatorv1.Task{DagRunId: "test"}

	// Attempt dispatch - should fail
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := client.Dispatch(ctx, task)
	require.Error(t, err)

	// Check failure metrics
	metrics = client.Metrics()
	assert.False(t, metrics.IsConnected)
	assert.Greater(t, metrics.ConsecutiveFails, 0)
	assert.Greater(t, metrics.FailCount, 0)
}

func TestClientCleanup(t *testing.T) {
	config := coordinator.DefaultConfig()
	config.RequestTimeout = 100 * time.Millisecond

	mockCoord := &mockCoordinatorService{
		dispatchFunc: func(_ context.Context, _ *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
			return &coordinatorv1.DispatchResponse{}, nil
		},
	}

	server, addr := startMockServer(t, mockCoord)
	defer server.Stop()

	monitor := &mockServiceMonitor{
		members: []execution.HostInfo{{Host: strings.Split(addr, ":")[0], Port: parsePort(addr), Status: execution.ServiceStatusActive}},
	}

	client := coordinator.New(monitor, config)

	// Make a call to establish connection
	task := &coordinatorv1.Task{DagRunId: "test"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := client.Dispatch(ctx, task)
	require.NoError(t, err)

	// Cleanup should close all connections
	err = client.Cleanup(ctx)
	require.NoError(t, err)

	// Future calls should still work (will create new connections)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	err = client.Dispatch(ctx2, task)
	require.NoError(t, err)
}

func TestClientDispatcherInterface(t *testing.T) {
	// Verify that clientImpl implements the core.Dispatcher interface
	config := coordinator.DefaultConfig()
	monitor := &mockServiceMonitor{}

	client := coordinator.New(monitor, config)

	// This should compile if the interface is implemented correctly
	var _ core.Dispatcher = client

	// Test the Dispatch method from the interface
	task := &coordinatorv1.Task{
		DagRunId: "test-dag-run",
		Target:   "test.yaml",
	}

	// Should fail gracefully with no coordinators
	monitor.members = []execution.HostInfo{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := client.Dispatch(ctx, task)
	require.Error(t, err)
	// Could be either error depending on timing
	assert.True(t, strings.Contains(err.Error(), "no coordinator instances available") ||
		strings.Contains(err.Error(), "context deadline exceeded"))
}

// Mock implementations

type mockServiceMonitor struct {
	members   []execution.HostInfo
	err       error
	onMembers func()
}

func (m *mockServiceMonitor) Register(_ context.Context, _ execution.ServiceName, _ execution.HostInfo) error {
	return nil
}

func (m *mockServiceMonitor) GetServiceMembers(_ context.Context, _ execution.ServiceName) ([]execution.HostInfo, error) {
	if m.onMembers != nil {
		m.onMembers()
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.members, nil
}

func (m *mockServiceMonitor) Unregister(_ context.Context) {
	// No-op
}

func (m *mockServiceMonitor) UpdateStatus(_ context.Context, _ execution.ServiceName, _ execution.ServiceStatus) error {
	return nil
}

type mockCoordinatorService struct {
	coordinatorv1.UnimplementedCoordinatorServiceServer

	dispatchFunc   func(context.Context, *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error)
	pollFunc       func(context.Context, *coordinatorv1.PollRequest) (*coordinatorv1.PollResponse, error)
	getWorkersFunc func(context.Context, *coordinatorv1.GetWorkersRequest) (*coordinatorv1.GetWorkersResponse, error)
	heartbeatFunc  func(context.Context, *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error)
}

func (m *mockCoordinatorService) Dispatch(ctx context.Context, req *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
	if m.dispatchFunc != nil {
		return m.dispatchFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCoordinatorService) Poll(ctx context.Context, req *coordinatorv1.PollRequest) (*coordinatorv1.PollResponse, error) {
	if m.pollFunc != nil {
		return m.pollFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCoordinatorService) GetWorkers(ctx context.Context, req *coordinatorv1.GetWorkersRequest) (*coordinatorv1.GetWorkersResponse, error) {
	if m.getWorkersFunc != nil {
		return m.getWorkersFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCoordinatorService) Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	if m.heartbeatFunc != nil {
		return m.heartbeatFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Helper to start a mock gRPC server
func startMockServer(t *testing.T, service coordinatorv1.CoordinatorServiceServer) (*grpc.Server, string) {
	t.Helper()

	server := grpc.NewServer()
	coordinatorv1.RegisterCoordinatorServiceServer(server, service)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		_ = server.Serve(listener)
	}()

	return server, listener.Addr().String()
}
