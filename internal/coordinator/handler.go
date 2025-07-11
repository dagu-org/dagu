package coordinator

import (
	"context"
	"sync"
	"time"

	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type workerInfo struct {
	workerID    string
	pollerID    string
	taskChan    chan *coordinatorv1.Task
	labels      map[string]string
	connectedAt time.Time
}

type heartbeatInfo struct {
	workerID        string
	labels          map[string]string
	stats           *coordinatorv1.WorkerStats
	lastHeartbeatAt time.Time
}

type Handler struct {
	coordinatorv1.UnimplementedCoordinatorServiceServer

	mu             sync.Mutex
	waitingPollers map[string]*workerInfo    // pollerID -> worker info
	heartbeats     map[string]*heartbeatInfo // workerID -> heartbeat info
}

func NewHandler() *Handler {
	return &Handler{
		waitingPollers: make(map[string]*workerInfo),
		heartbeats:     make(map[string]*heartbeatInfo),
	}
}

// Poll implements long polling - workers wait until a task is available
func (h *Handler) Poll(ctx context.Context, req *coordinatorv1.PollRequest) (*coordinatorv1.PollResponse, error) {
	if req.PollerId == "" {
		return nil, status.Error(codes.InvalidArgument, "poller_id is required")
	}

	// Register this poller to wait for a task
	h.mu.Lock()
	taskChan := make(chan *coordinatorv1.Task, 1)
	h.waitingPollers[req.PollerId] = &workerInfo{
		workerID:    req.WorkerId,
		pollerID:    req.PollerId,
		taskChan:    taskChan,
		labels:      req.Labels,
		connectedAt: time.Now(),
	}
	h.mu.Unlock()

	// Wait for a task or context cancellation
	select {
	case task := <-taskChan:
		h.mu.Lock()
		delete(h.waitingPollers, req.PollerId)
		h.mu.Unlock()

		return &coordinatorv1.PollResponse{Task: task}, nil

	case <-ctx.Done():
		h.mu.Lock()
		delete(h.waitingPollers, req.PollerId)
		h.mu.Unlock()

		return nil, ctx.Err()
	}
}

// Dispatch tries to send a task to a waiting poller
// It fails if no pollers are available or no workers match the selector
func (h *Handler) Dispatch(_ context.Context, req *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
	if req.Task == nil {
		return nil, status.Error(codes.InvalidArgument, "task is required")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Try to find a waiting poller that matches the worker selector
	for pollerID, worker := range h.waitingPollers {
		// Check if worker matches the selector
		if !matchesSelector(worker.labels, req.Task.WorkerSelector) {
			continue
		}

		select {
		case worker.taskChan <- req.Task:
			// Successfully dispatched to a waiting poller
			delete(h.waitingPollers, pollerID)
			return &coordinatorv1.DispatchResponse{}, nil
		default:
			// Channel might be closed/full, clean it up
			delete(h.waitingPollers, pollerID)
		}
	}

	// No available matching pollers - dispatch fails
	if len(req.Task.WorkerSelector) > 0 {
		return nil, status.Error(codes.FailedPrecondition, "no workers match the required selector")
	}
	return nil, status.Error(codes.FailedPrecondition, "no available workers")
}

// matchesSelector checks if worker labels match all required selector labels
func matchesSelector(workerLabels, selector map[string]string) bool {
	// Empty selector matches any worker
	if len(selector) == 0 {
		return true
	}

	// Check all selector requirements are met
	for key, value := range selector {
		if workerLabels[key] != value {
			return false
		}
	}
	return true
}

// GetWorkers returns the list of currently connected workers
func (h *Handler) GetWorkers(_ context.Context, _ *coordinatorv1.GetWorkersRequest) (*coordinatorv1.GetWorkersResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return aggregated worker info from heartbeats
	workers := make([]*coordinatorv1.WorkerInfo, 0, len(h.heartbeats))
	now := time.Now()

	for _, hb := range h.heartbeats {
		// Calculate health status based on heartbeat recency
		secondsSinceHeartbeat := now.Sub(hb.lastHeartbeatAt).Seconds()
		var healthStatus coordinatorv1.WorkerHealthStatus

		switch {
		case secondsSinceHeartbeat < 5:
			healthStatus = coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY
		case secondsSinceHeartbeat < 15:
			healthStatus = coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING
		default:
			healthStatus = coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY
		}

		workerInfo := &coordinatorv1.WorkerInfo{
			WorkerId:        hb.workerID,
			Labels:          hb.labels,
			LastHeartbeatAt: hb.lastHeartbeatAt.Unix(),
			HealthStatus:    healthStatus,
		}

		// Add stats if available
		if hb.stats != nil {
			workerInfo.TotalPollers = hb.stats.TotalPollers
			workerInfo.BusyPollers = hb.stats.BusyPollers
			workerInfo.RunningTasks = hb.stats.RunningTasks
		}

		workers = append(workers, workerInfo)
	}

	return &coordinatorv1.GetWorkersResponse{
		Workers: workers,
	}, nil
}

// Heartbeat receives periodic status updates from workers
func (h *Handler) Heartbeat(_ context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	if req.WorkerId == "" {
		return nil, status.Error(codes.InvalidArgument, "worker_id is required")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update or create heartbeat info
	h.heartbeats[req.WorkerId] = &heartbeatInfo{
		workerID:        req.WorkerId,
		labels:          req.Labels,
		stats:           req.Stats,
		lastHeartbeatAt: time.Now(),
	}

	// Clean up stale heartbeats (older than 30 seconds)
	staleThreshold := time.Now().Add(-30 * time.Second)
	for workerID, info := range h.heartbeats {
		if info.lastHeartbeatAt.Before(staleThreshold) {
			delete(h.heartbeats, workerID)
		}
	}

	return &coordinatorv1.HeartbeatResponse{}, nil
}
