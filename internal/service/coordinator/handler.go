package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/proto/convert"
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

// staleHeartbeatThreshold is the duration after which a worker's heartbeat is considered stale.
const staleHeartbeatThreshold = 30 * time.Second

type Handler struct {
	coordinatorv1.UnimplementedCoordinatorServiceServer

	mu             sync.Mutex
	waitingPollers map[string]*workerInfo    // pollerID -> worker info
	heartbeats     map[string]*heartbeatInfo // workerID -> heartbeat info

	// Optional: for shared-nothing worker architecture
	dagRunStore execution.DAGRunStore // For status persistence
	logDir      string                // For log storage

	// Open attempts cache for status persistence
	attemptsMu   sync.RWMutex
	openAttempts map[string]execution.DAGRunAttempt // dagRunID -> open attempt

	// Zombie detector shutdown synchronization
	zombieDetectorMu      sync.Mutex
	zombieDetectorStarted bool
	zombieDetectorDone    chan struct{}
}

// HandlerOption configures the Handler
type HandlerOption func(*Handler)

// WithDAGRunStore sets the DAGRunStore for status persistence
func WithDAGRunStore(store execution.DAGRunStore) HandlerOption {
	return func(h *Handler) {
		h.dagRunStore = store
	}
}

// WithLogDir sets the log directory for log storage
func WithLogDir(dir string) HandlerOption {
	return func(h *Handler) {
		h.logDir = dir
	}
}

func NewHandler(opts ...HandlerOption) *Handler {
	h := &Handler{
		waitingPollers: make(map[string]*workerInfo),
		heartbeats:     make(map[string]*heartbeatInfo),
		openAttempts:   make(map[string]execution.DAGRunAttempt),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Close cleans up all resources held by the handler.
// This should be called during coordinator shutdown.
func (h *Handler) Close(ctx context.Context) {
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	for dagRunID, attempt := range h.openAttempts {
		if err := attempt.Close(ctx); err != nil {
			logger.Warn(ctx, "Failed to close attempt during handler shutdown",
				tag.RunID(dagRunID), tag.Error(err))
		}
		delete(h.openAttempts, dagRunID)
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

		// Inject the worker ID into the task so it can be tracked
		if task != nil {
			task.WorkerId = req.WorkerId
		}

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

	workers := make([]*coordinatorv1.WorkerInfo, 0, len(h.heartbeats))
	now := time.Now()

	for _, hb := range h.heartbeats {
		workerInfo := &coordinatorv1.WorkerInfo{
			WorkerId:        hb.workerID,
			Labels:          hb.labels,
			LastHeartbeatAt: hb.lastHeartbeatAt.Unix(),
			HealthStatus:    calculateHealthStatus(now.Sub(hb.lastHeartbeatAt)),
		}

		if hb.stats != nil {
			workerInfo.TotalPollers = hb.stats.TotalPollers
			workerInfo.BusyPollers = hb.stats.BusyPollers
			workerInfo.RunningTasks = hb.stats.RunningTasks
		}

		workers = append(workers, workerInfo)
	}

	return &coordinatorv1.GetWorkersResponse{Workers: workers}, nil
}

// calculateHealthStatus determines worker health based on time since last heartbeat.
func calculateHealthStatus(sinceLastHeartbeat time.Duration) coordinatorv1.WorkerHealthStatus {
	switch {
	case sinceLastHeartbeat < 5*time.Second:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY
	case sinceLastHeartbeat < 15*time.Second:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING
	default:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY
	}
}

// Heartbeat receives periodic status updates from workers
func (h *Handler) Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	if req.WorkerId == "" {
		return nil, status.Error(codes.InvalidArgument, "worker_id is required")
	}

	// Update heartbeat under lock
	h.mu.Lock()
	h.heartbeats[req.WorkerId] = &heartbeatInfo{
		workerID:        req.WorkerId,
		labels:          req.Labels,
		stats:           req.Stats,
		lastHeartbeatAt: time.Now(),
	}
	h.mu.Unlock()

	// Note: Zombie detection is handled by StartZombieDetector's periodic ticker,
	// not on every heartbeat, to avoid O(n²) complexity with many workers.

	// Check for cancelled runs among the worker's running tasks
	cancelledRuns := h.getCancelledRunsForWorker(ctx, req.Stats)

	return &coordinatorv1.HeartbeatResponse{
		CancelledRuns: cancelledRuns,
	}, nil
}

// getCancelledRunsForWorker checks which of the worker's running tasks have been cancelled
func (h *Handler) getCancelledRunsForWorker(ctx context.Context, stats *coordinatorv1.WorkerStats) []string {
	if h.dagRunStore == nil || stats == nil || len(stats.RunningTasks) == 0 {
		return nil
	}

	var cancelledRuns []string
	for _, task := range stats.RunningTasks {
		ref := execution.DAGRunRef{Name: task.DagName, ID: task.DagRunId}
		attempt, err := h.dagRunStore.FindAttempt(ctx, ref)
		if err != nil {
			// Attempt not found, skip
			continue
		}

		// Check if the attempt has been marked for cancellation
		aborting, err := attempt.IsAborting(ctx)
		if err != nil {
			continue
		}

		if aborting {
			cancelledRuns = append(cancelledRuns, task.DagRunId)
		}
	}

	return cancelledRuns
}

// ReportStatus receives status updates from workers and persists them.
// This is used in shared-nothing architecture where workers don't have filesystem access.
func (h *Handler) ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
	if h.dagRunStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "status reporting not configured: dagRunStore is nil")
	}

	if req.Status == nil {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	// Convert proto to execution.DAGRunStatus
	dagRunStatus := convert.ProtoToDAGRunStatus(req.Status)

	// Get or create an open attempt for this dag run
	// Check if this is a sub-DAG (has root that differs from self)
	var attempt execution.DAGRunAttempt
	var err error

	isSubDAG := dagRunStatus.Root.ID != "" && dagRunStatus.Root.ID != dagRunStatus.DAGRunID
	if isSubDAG {
		attempt, err = h.getOrOpenSubAttempt(ctx, dagRunStatus.Root, dagRunStatus.DAGRunID)
	} else {
		attempt, err = h.getOrOpenAttempt(ctx, dagRunStatus.Name, dagRunStatus.DAGRunID)
	}

	if err != nil {
		return &coordinatorv1.ReportStatusResponse{
			Accepted: false,
			Error:    err.Error(),
		}, nil
	}

	// Write the status
	if err := attempt.Write(ctx, *dagRunStatus); err != nil {
		return &coordinatorv1.ReportStatusResponse{
			Accepted: false,
			Error:    "failed to write status: " + err.Error(),
		}, nil
	}

	// Note: We don't close the attempt immediately on terminal status because
	// the agent may push the same terminal status multiple times from different
	// code paths. Attempts are cleaned up during coordinator shutdown.

	return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
}

// getOrOpenAttempt retrieves an open attempt from cache or opens a new one.
// Uses double-check locking to avoid holding the mutex during blocking I/O.
func (h *Handler) getOrOpenAttempt(ctx context.Context, dagName, dagRunID string) (execution.DAGRunAttempt, error) {
	ref := execution.DAGRunRef{Name: dagName, ID: dagRunID}
	return h.getOrOpenAttemptWithFinder(ctx, dagRunID, func() (execution.DAGRunAttempt, error) {
		return h.dagRunStore.FindAttempt(ctx, ref)
	})
}

// getOrOpenSubAttempt retrieves an open sub-attempt from cache or opens a new one.
// This is used for sub-DAG status reporting in distributed execution.
func (h *Handler) getOrOpenSubAttempt(ctx context.Context, rootRef execution.DAGRunRef, subDAGRunID string) (execution.DAGRunAttempt, error) {
	return h.getOrOpenAttemptWithFinder(ctx, subDAGRunID, func() (execution.DAGRunAttempt, error) {
		return h.dagRunStore.FindSubAttempt(ctx, rootRef, subDAGRunID)
	})
}

// getOrOpenAttemptWithFinder is a generic helper that retrieves an open attempt from cache
// or uses the provided finder function to locate and open a new one.
// Uses double-check locking to avoid holding the mutex during blocking I/O.
func (h *Handler) getOrOpenAttemptWithFinder(ctx context.Context, cacheKey string, finder func() (execution.DAGRunAttempt, error)) (execution.DAGRunAttempt, error) {
	// First check: fast path with read lock
	h.attemptsMu.RLock()
	if attempt, ok := h.openAttempts[cacheKey]; ok {
		h.attemptsMu.RUnlock()
		return attempt, nil
	}
	h.attemptsMu.RUnlock()

	// Perform I/O without lock to avoid blocking other goroutines
	attempt, err := finder()
	if err != nil {
		return nil, err
	}

	if err := attempt.Open(ctx); err != nil {
		return nil, err
	}

	// Second check: acquire write lock and verify no race
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	if existing, ok := h.openAttempts[cacheKey]; ok {
		// Another goroutine opened it first, close ours and use theirs
		_ = attempt.Close(ctx)
		return existing, nil
	}

	h.openAttempts[cacheKey] = attempt
	return attempt, nil
}

// StreamLogs receives log streams from workers and writes them to local filesystem.
// This is used in shared-nothing architecture where workers don't have filesystem access.
func (h *Handler) StreamLogs(stream coordinatorv1.CoordinatorService_StreamLogsServer) error {
	if h.logDir == "" {
		return status.Error(codes.FailedPrecondition, "log streaming not configured: logDir is empty")
	}

	// Delegate to the log handler
	logHandler := newLogHandler(h.logDir)
	defer logHandler.Close() // Ensure file handles are closed on stream end or error
	return logHandler.handleStream(stream)
}

// GetDAGRunStatus retrieves the status of a DAG run.
// This is used by parent DAGs to poll the status of remote sub-DAGs in shared-nothing mode.
func (h *Handler) GetDAGRunStatus(ctx context.Context, req *coordinatorv1.GetDAGRunStatusRequest) (*coordinatorv1.GetDAGRunStatusResponse, error) {
	if h.dagRunStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "DAG run status query not configured: dagRunStore is nil")
	}

	if req.DagName == "" || req.DagRunId == "" {
		return nil, status.Error(codes.InvalidArgument, "dag_name and dag_run_id are required")
	}

	var attempt execution.DAGRunAttempt
	var err error

	// Check if this is a sub-DAG query (root info provided)
	if req.RootDagRunName != "" && req.RootDagRunId != "" {
		// Look up as a sub-DAG
		rootRef := execution.DAGRunRef{Name: req.RootDagRunName, ID: req.RootDagRunId}
		attempt, err = h.dagRunStore.FindSubAttempt(ctx, rootRef, req.DagRunId)
	} else {
		// Look up as a top-level DAG run
		ref := execution.DAGRunRef{Name: req.DagName, ID: req.DagRunId}
		attempt, err = h.dagRunStore.FindAttempt(ctx, ref)
	}

	if err != nil {
		// Not found is not an error, just return found=false
		return &coordinatorv1.GetDAGRunStatusResponse{
			Found: false,
		}, nil
	}

	runStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return &coordinatorv1.GetDAGRunStatusResponse{
			Found: false,
			Error: fmt.Sprintf("failed to read status: %v", err),
		}, nil
	}

	return &coordinatorv1.GetDAGRunStatusResponse{
		Found:  true,
		Status: convert.DAGRunStatusToProto(runStatus),
	}, nil
}

// StartZombieDetector starts a background goroutine that periodically checks for zombie runs.
// It detects workers that have stopped sending heartbeats and marks their running tasks as failed.
// The interval parameter controls how often the detector runs (recommended: 45 seconds).
// Call WaitZombieDetector after canceling the context to ensure clean shutdown.
// This method is safe to call multiple times; subsequent calls are no-ops.
func (h *Handler) StartZombieDetector(ctx context.Context, interval time.Duration) {
	h.zombieDetectorMu.Lock()
	defer h.zombieDetectorMu.Unlock()

	if h.zombieDetectorStarted {
		return // Already started
	}
	h.zombieDetectorStarted = true
	h.zombieDetectorDone = make(chan struct{})

	go func() {
		defer close(h.zombieDetectorDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.detectAndCleanupZombies(ctx)
			}
		}
	}()
}

// WaitZombieDetector waits for the zombie detector goroutine to finish.
// This should be called after the context passed to StartZombieDetector is canceled.
func (h *Handler) WaitZombieDetector() {
	h.zombieDetectorMu.Lock()
	done := h.zombieDetectorDone
	h.zombieDetectorMu.Unlock()

	if done != nil {
		<-done
	}
}

// detectAndCleanupZombies checks for stale workers and marks their tasks as failed.
func (h *Handler) detectAndCleanupZombies(ctx context.Context) {
	// Collect stale workers under lock, then process outside lock to avoid
	// holding the mutex during blocking I/O operations.
	staleWorkers := h.collectAndRemoveStaleHeartbeats()

	// Process stale workers outside the lock
	for _, info := range staleWorkers {
		h.markWorkerTasksFailed(ctx, info)
	}
}

// collectAndRemoveStaleHeartbeats finds and removes stale heartbeats from the map.
// Returns the removed heartbeat infos for processing.
func (h *Handler) collectAndRemoveStaleHeartbeats() []*heartbeatInfo {
	h.mu.Lock()
	defer h.mu.Unlock()

	staleThreshold := time.Now().Add(-staleHeartbeatThreshold)
	var stale []*heartbeatInfo

	for workerID, info := range h.heartbeats {
		if info.lastHeartbeatAt.Before(staleThreshold) {
			stale = append(stale, info)
			delete(h.heartbeats, workerID)
		}
	}
	return stale
}

// markWorkerTasksFailed marks all running tasks from a failed worker as FAILED.
// This is called when a worker's heartbeat becomes stale (worker crashed or disconnected).
func (h *Handler) markWorkerTasksFailed(ctx context.Context, info *heartbeatInfo) {
	if h.dagRunStore == nil || info.stats == nil {
		return
	}

	for _, task := range info.stats.RunningTasks {
		reason := fmt.Sprintf("worker %s became unresponsive", info.workerID)
		h.markRunFailed(ctx, task.DagName, task.DagRunId, reason)
	}
}

// markRunFailed marks a single DAG run as FAILED.
// This is used to clean up zombie runs when their worker becomes unresponsive.
// Uses context.WithoutCancel to ensure cleanup I/O completes even if the
// zombie detector context is canceled mid-flight.
func (h *Handler) markRunFailed(ctx context.Context, dagName, dagRunID, reason string) {
	// Use non-cancelable context for store operations to ensure cleanup completes
	storeCtx := context.WithoutCancel(ctx)
	ref := execution.DAGRunRef{Name: dagName, ID: dagRunID}
	attempt, err := h.dagRunStore.FindAttempt(storeCtx, ref)
	if err != nil {
		logger.Error(ctx, "Failed to find attempt for zombie cleanup",
			tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
		return
	}

	dagRunStatus, err := attempt.ReadStatus(storeCtx)
	if err != nil {
		logger.Error(ctx, "Failed to read status for zombie cleanup",
			tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
		return
	}

	// Only mark as failed if still active (running, queued, or waiting)
	if !dagRunStatus.Status.IsActive() {
		return
	}

	// Update status to FAILED with consistent timestamp
	finishedAt := stringutil.FormatTime(time.Now())
	dagRunStatus.Status = core.Failed
	dagRunStatus.FinishedAt = finishedAt
	dagRunStatus.Error = reason

	// Mark all running nodes as failed
	for i, node := range dagRunStatus.Nodes {
		if node.Status == core.NodeRunning {
			dagRunStatus.Nodes[i].Status = core.NodeFailed
			dagRunStatus.Nodes[i].FinishedAt = finishedAt
			dagRunStatus.Nodes[i].Error = reason
		}
	}

	// Persist the failed status
	if err := attempt.Open(storeCtx); err != nil {
		logger.Error(ctx, "Failed to open attempt for zombie cleanup",
			tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
		return
	}
	defer func() { _ = attempt.Close(storeCtx) }()

	if err := attempt.Write(storeCtx, *dagRunStatus); err != nil {
		logger.Error(ctx, "Failed to write failed status for zombie cleanup",
			tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
		return
	}

	logger.Warn(ctx, "Marked zombie run as FAILED",
		tag.DAG(dagName), tag.RunID(dagRunID), slog.String("reason", reason))
}
