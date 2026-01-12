package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
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

// defaultStaleHeartbeatThreshold is the default duration after which a worker's heartbeat is considered stale.
const defaultStaleHeartbeatThreshold = 30 * time.Second

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

	// Per-run mutexes to prevent concurrent access to the same DAG run
	// This prevents races between ReportStatus and markRunFailed
	runMutexesMu sync.Mutex
	runMutexes   map[string]*sync.Mutex // dagRunID -> per-run mutex

	// Stale heartbeat threshold - configurable
	staleHeartbeatThreshold time.Duration

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

// WithStaleHeartbeatThreshold sets the threshold for considering a worker's heartbeat stale.
// Workers that haven't sent a heartbeat within this duration are considered dead.
func WithStaleHeartbeatThreshold(d time.Duration) HandlerOption {
	return func(h *Handler) {
		h.staleHeartbeatThreshold = d
	}
}

func NewHandler(opts ...HandlerOption) *Handler {
	h := &Handler{
		waitingPollers:          make(map[string]*workerInfo),
		heartbeats:              make(map[string]*heartbeatInfo),
		openAttempts:            make(map[string]execution.DAGRunAttempt),
		runMutexes:              make(map[string]*sync.Mutex),
		staleHeartbeatThreshold: defaultStaleHeartbeatThreshold,
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

// getRunMutex returns a mutex for the given DAG run ID.
// This ensures serialized access to operations on the same DAG run
// across ReportStatus and zombie cleanup to prevent data races.
func (h *Handler) getRunMutex(dagRunID string) *sync.Mutex {
	h.runMutexesMu.Lock()
	defer h.runMutexesMu.Unlock()

	if mu, ok := h.runMutexes[dagRunID]; ok {
		return mu
	}

	mu := &sync.Mutex{}
	h.runMutexes[dagRunID] = mu
	return mu
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
func (h *Handler) Dispatch(ctx context.Context, req *coordinatorv1.DispatchRequest) (*coordinatorv1.DispatchResponse, error) {
	if req.Task == nil {
		return nil, status.Error(codes.InvalidArgument, "task is required")
	}

	logger.Info(ctx, "Handler Dispatch called",
		tag.RunID(req.Task.DagRunId),
		tag.Target(req.Task.Target),
		slog.String("operation", req.Task.Operation.String()),
	)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Create attempt before dispatching to ensure coordinator has a place to store status updates.
	// This is done after acquiring the lock to prevent race conditions where multiple concurrent
	// Dispatch() calls could create duplicate attempts for the same DagRunId.
	isRootRun := req.Task.ParentDagRunId == "" &&
		(req.Task.RootDagRunId == "" || req.Task.RootDagRunId == req.Task.DagRunId)

	if h.dagRunStore != nil && req.Task.Definition != "" {
		if isRootRun {
			// For root-level DAG runs (no parent), create the attempt
			if err := h.createAttemptForTask(ctx, req.Task); err != nil {
				logger.Warn(ctx, "Failed to create attempt for task", tag.Error(err), tag.RunID(req.Task.DagRunId))
				// Don't fail dispatch - the task can still run, status just won't be stored
			}
		} else if req.Task.ParentDagRunId != "" {
			// For sub-DAG runs, create the attempt under the root DAG run
			if err := h.createSubAttemptForTask(ctx, req.Task); err != nil {
				logger.Warn(ctx, "Failed to create sub-attempt for task", tag.Error(err), tag.RunID(req.Task.DagRunId))
				// Don't fail dispatch - the task can still run, status just won't be stored
			}
		}
	}

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

// createAttemptForTask creates a DAGRun attempt for a root-level task.
// This is called when the coordinator receives a dispatch for a root-level DAG run
// (not a sub-DAG), so it has a place to store status updates from the worker.
func (h *Handler) createAttemptForTask(ctx context.Context, task *coordinatorv1.Task) error {
	// Parse the full DAG from the definition to preserve all metadata
	// (including workerSelector, steps, etc.) for retry operations
	dag, err := spec.LoadYAML(ctx, []byte(task.Definition), spec.WithName(task.Target))
	if err != nil {
		return fmt.Errorf("failed to parse DAG definition: %w", err)
	}

	// Determine if this is a retry operation
	isRetry := task.Operation == coordinatorv1.Operation_OPERATION_RETRY
	opts := execution.NewDAGRunAttemptOptions{Retry: isRetry}

	// Create the attempt
	attempt, err := h.dagRunStore.CreateAttempt(ctx, dag, time.Now(), task.DagRunId, opts)
	if err != nil {
		return fmt.Errorf("failed to create attempt: %w", err)
	}

	// Set the attempt ID in the task so workers can use the same ID
	task.AttemptId = attempt.ID()

	// Open the attempt for writing
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open attempt: %w", err)
	}

	// Cache the open attempt so ReportStatus can find it
	h.attemptsMu.Lock()
	h.openAttempts[task.DagRunId] = attempt
	h.attemptsMu.Unlock()

	logger.Info(ctx, "Created DAGRun attempt for dispatched task",
		tag.RunID(task.DagRunId),
		tag.Target(task.Target),
		slog.String("attempt-id", task.AttemptId),
	)

	return nil
}

// createSubAttemptForTask creates a sub-DAG attempt under the root DAG run.
// This is called when the coordinator receives a dispatch for a sub-DAG
// (dispatched from a parent DAG), so it has a place to store status updates from the worker.
func (h *Handler) createSubAttemptForTask(ctx context.Context, task *coordinatorv1.Task) error {
	rootRef := execution.DAGRunRef{
		Name: task.RootDagRunName,
		ID:   task.RootDagRunId,
	}

	attempt, err := h.dagRunStore.CreateSubAttempt(ctx, rootRef, task.DagRunId)
	if err != nil {
		return fmt.Errorf("failed to create sub-attempt: %w", err)
	}

	// Parse and set the DAG from the definition to preserve all metadata
	// (including workerSelector, steps, etc.) for retry operations
	if task.Definition != "" {
		dag, err := spec.LoadYAML(ctx, []byte(task.Definition), spec.WithName(task.Target))
		if err != nil {
			return fmt.Errorf("failed to parse DAG definition: %w", err)
		}
		attempt.SetDAG(dag)
	}

	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open sub-attempt: %w", err)
	}

	// Cache the open attempt for ReportStatus calls
	h.attemptsMu.Lock()
	h.openAttempts[task.DagRunId] = attempt
	h.attemptsMu.Unlock()

	logger.Info(ctx, "Created sub-DAG attempt for distributed execution",
		tag.RunID(task.DagRunId),
		tag.DAG(task.Target),
		slog.String("root-dag-run-id", task.RootDagRunId),
	)

	return nil
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
	const (
		healthyThreshold = 5 * time.Second
		warningThreshold = 15 * time.Second
	)

	switch {
	case sinceLastHeartbeat < healthyThreshold:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY
	case sinceLastHeartbeat < warningThreshold:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING
	default:
		return coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY
	}
}

// Heartbeat receives periodic status updates from workers.
func (h *Handler) Heartbeat(ctx context.Context, req *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	if req.WorkerId == "" {
		return nil, status.Error(codes.InvalidArgument, "worker_id is required")
	}

	h.mu.Lock()
	h.heartbeats[req.WorkerId] = &heartbeatInfo{
		workerID:        req.WorkerId,
		labels:          req.Labels,
		stats:           req.Stats,
		lastHeartbeatAt: time.Now(),
	}
	h.mu.Unlock()

	cancelledRuns := h.getCancelledRunsForWorker(ctx, req.Stats)

	return &coordinatorv1.HeartbeatResponse{
		CancelledRuns: cancelledRuns,
	}, nil
}

// getCancelledRunsForWorker checks which of the worker's running tasks have been cancelled.
func (h *Handler) getCancelledRunsForWorker(ctx context.Context, stats *coordinatorv1.WorkerStats) []string {
	if h.dagRunStore == nil || stats == nil || len(stats.RunningTasks) == 0 {
		return nil
	}

	var cancelledRuns []string
	for _, task := range stats.RunningTasks {
		if h.isTaskCancelled(ctx, task) {
			cancelledRuns = append(cancelledRuns, task.DagRunId)
		}
	}
	return cancelledRuns
}

// isTaskCancelled checks if a task has been marked for cancellation.
func (h *Handler) isTaskCancelled(ctx context.Context, task *coordinatorv1.RunningTask) bool {
	h.attemptsMu.RLock()
	cachedAttempt, ok := h.openAttempts[task.DagRunId]
	h.attemptsMu.RUnlock()

	if ok {
		aborting, err := cachedAttempt.IsAborting(ctx)
		return err == nil && aborting
	}

	ref := execution.DAGRunRef{Name: task.DagName, ID: task.DagRunId}
	attempt, err := h.dagRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return false
	}

	aborting, err := attempt.IsAborting(ctx)
	return err == nil && aborting
}

// ReportStatus receives status updates from workers and persists them.
// This is used in shared-nothing architecture where workers don't have filesystem access.
func (h *Handler) ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
	if h.dagRunStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "status reporting not configured: DAG run storage not available")
	}

	if req.Status == nil {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	// Convert proto to execution.DAGRunStatus
	dagRunStatus := convert.ProtoToDAGRunStatus(req.Status)

	// Transform worker-local log paths to coordinator paths (shared-nothing mode)
	h.transformLogPaths(dagRunStatus)

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
		return nil, status.Error(codes.Internal, "failed to get/open attempt: "+err.Error())
	}

	// Write the status
	if err := attempt.Write(ctx, *dagRunStatus); err != nil {
		return nil, status.Error(codes.Internal, "failed to write status: "+err.Error())
	}

	// Note: We don't close the attempt immediately on terminal status because
	// the agent may push the same terminal status multiple times from different
	// code paths. Attempts are cleaned up during coordinator shutdown.

	return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
}

// transformLogPaths rewrites worker-local log paths to coordinator paths.
// This is called when logDir is configured (shared-nothing mode).
func (h *Handler) transformLogPaths(status *execution.DAGRunStatus) {
	if h.logDir == "" {
		return // Not in shared-nothing mode, keep original paths
	}

	dagName := status.Name
	dagRunID := status.DAGRunID
	attemptID := status.AttemptID

	// For sub-DAGs, logs are stored under root DAG's directory
	if status.Root.ID != "" && status.Root.ID != dagRunID {
		dagName = status.Root.Name
		dagRunID = status.Root.ID
	}

	// Use dagRunID as attemptID if not set (matches log_handler.go logic)
	if attemptID == "" {
		attemptID = status.DAGRunID
	}

	// Helper to compute coordinator log path
	computePath := func(stepName string, streamType coordinatorv1.LogStreamType) string {
		ext := StreamTypeToExtension(streamType)
		filename := fmt.Sprintf("%s.%s", fileutil.SafeName(stepName), ext)
		return filepath.Join(
			h.logDir,
			fileutil.SafeName(dagName),
			fileutil.SafeName(dagRunID),
			fileutil.SafeName(attemptID),
			filename,
		)
	}

	// Transform node log paths
	transformNode := func(node *execution.Node, fallbackName string) {
		if node == nil {
			return
		}
		// Use step name, or fallback for handler nodes with empty names
		stepName := node.Step.Name
		if stepName == "" {
			stepName = fallbackName
		}
		node.Stdout = computePath(stepName, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT)
		node.Stderr = computePath(stepName, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR)
	}

	// Transform all regular nodes
	for _, node := range status.Nodes {
		transformNode(node, "step")
	}

	// Transform handler nodes with explicit fallback names
	transformNode(status.OnInit, "on_init")
	transformNode(status.OnExit, "on_exit")
	transformNode(status.OnSuccess, "on_success")
	transformNode(status.OnFailure, "on_failure")
	transformNode(status.OnCancel, "on_cancel")
	transformNode(status.OnWait, "on_wait")

	// Transform scheduler log path
	status.Log = filepath.Join(
		h.logDir,
		fileutil.SafeName(dagName),
		fileutil.SafeName(dagRunID),
		fileutil.SafeName(attemptID),
		"scheduler.log",
	)
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
// Uses per-run mutex to prevent concurrent I/O operations on the same DAG run,
// avoiding races between ReportStatus and markRunFailed.
func (h *Handler) getOrOpenAttemptWithFinder(ctx context.Context, cacheKey string, finder func() (execution.DAGRunAttempt, error)) (execution.DAGRunAttempt, error) {
	// First check: fast path with read lock (no per-run mutex needed for cache hit)
	h.attemptsMu.RLock()
	if attempt, ok := h.openAttempts[cacheKey]; ok {
		h.attemptsMu.RUnlock()
		return attempt, nil
	}
	h.attemptsMu.RUnlock()

	// Acquire per-run mutex to serialize I/O operations for this DAG run
	runMu := h.getRunMutex(cacheKey)
	runMu.Lock()
	defer runMu.Unlock()

	// Re-check cache after acquiring per-run mutex (another goroutine may have opened it)
	h.attemptsMu.RLock()
	if attempt, ok := h.openAttempts[cacheKey]; ok {
		h.attemptsMu.RUnlock()
		return attempt, nil
	}
	h.attemptsMu.RUnlock()

	// Perform I/O with per-run mutex held
	attempt, err := finder()
	if err != nil {
		return nil, err
	}

	if err := attempt.Open(ctx); err != nil {
		return nil, err
	}

	// Add to cache under write lock
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	// Final check: if somehow another goroutine got through, use theirs
	if existing, ok := h.openAttempts[cacheKey]; ok {
		if err := attempt.Close(ctx); err != nil {
			logger.Warn(ctx, "Failed to close duplicate opened attempt", tag.Error(err))
		}
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
	defer logHandler.Close(stream.Context()) // Ensure file handles are closed on stream end or error
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

	// First check the cache for open attempts. This is critical for shared-nothing mode
	// where ReportStatus writes to a cached attempt object. The write is buffered, so
	// we need to read from the same cached attempt to see the latest status.
	// The cached attempt's ReadStatus method will flush the buffer before reading.
	h.attemptsMu.RLock()
	cachedAttempt, found := h.openAttempts[req.DagRunId]
	h.attemptsMu.RUnlock()

	if found {
		attempt = cachedAttempt
	} else {
		// Fall back to finding the attempt on disk
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

	staleThreshold := time.Now().Add(-h.staleHeartbeatThreshold)
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
// Uses per-run mutex to prevent races with concurrent ReportStatus calls.
func (h *Handler) markRunFailed(ctx context.Context, dagName, dagRunID, reason string) {
	// Use non-cancelable context for store operations to ensure cleanup completes
	storeCtx := context.WithoutCancel(ctx)

	// Acquire per-run mutex to prevent races with ReportStatus
	runMu := h.getRunMutex(dagRunID)
	runMu.Lock()
	defer runMu.Unlock()

	// Try to use cached attempt first (prevents race with ReportStatus)
	var attempt execution.DAGRunAttempt
	var needsOpen bool

	h.attemptsMu.RLock()
	cachedAttempt, ok := h.openAttempts[dagRunID]
	h.attemptsMu.RUnlock()

	if ok {
		attempt = cachedAttempt
		needsOpen = false
	} else {
		// Not in cache, find it
		ref := execution.DAGRunRef{Name: dagName, ID: dagRunID}
		foundAttempt, err := h.dagRunStore.FindAttempt(storeCtx, ref)
		if err != nil {
			logger.Error(ctx, "Failed to find attempt for zombie cleanup",
				tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
			return
		}
		attempt = foundAttempt
		needsOpen = true
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

	// Only open if we didn't get from cache
	if needsOpen {
		if err := attempt.Open(storeCtx); err != nil {
			logger.Error(ctx, "Failed to open attempt for zombie cleanup",
				tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
			return
		}
		defer func() {
			if err := attempt.Close(storeCtx); err != nil {
				logger.Warn(ctx, "Failed to close attempt in markRunFailed",
					tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
			}
		}()
	}

	if err := attempt.Write(storeCtx, *dagRunStatus); err != nil {
		logger.Error(ctx, "Failed to write failed status for zombie cleanup",
			tag.DAG(dagName), tag.RunID(dagRunID), tag.Error(err))
		return
	}

	logger.Warn(ctx, "Marked zombie run as FAILED",
		tag.DAG(dagName), tag.RunID(dagRunID), slog.String("reason", reason))
}

// RequestCancel handles requests to cancel a DAG run.
// This is used in shared-nothing mode for sub-DAG cancellation where the parent
// worker cannot directly access the sub-DAG's attempt.
func (h *Handler) RequestCancel(ctx context.Context, req *coordinatorv1.RequestCancelRequest) (*coordinatorv1.RequestCancelResponse, error) {
	if h.dagRunStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "cancellation not available: DAG run storage not configured")
	}
	if req.DagName == "" {
		return nil, status.Error(codes.InvalidArgument, "dag_name is required")
	}
	if req.DagRunId == "" {
		return nil, status.Error(codes.InvalidArgument, "dag_run_id is required")
	}

	ctx = logger.WithValues(ctx,
		tag.DAG(req.DagName),
		tag.RunID(req.DagRunId),
	)

	// Find the attempt (either root or sub-DAG)
	var attempt execution.DAGRunAttempt
	var err error

	isSubDAG := req.RootDagRunId != "" && req.RootDagRunId != req.DagRunId
	if isSubDAG {
		rootRef := execution.DAGRunRef{Name: req.RootDagRunName, ID: req.RootDagRunId}
		attempt, err = h.dagRunStore.FindSubAttempt(ctx, rootRef, req.DagRunId)
		logger.Info(ctx, "Looking up sub-DAG attempt for cancellation",
			slog.String("root-dag-run-id", req.RootDagRunId),
		)
	} else {
		ref := execution.DAGRunRef{Name: req.DagName, ID: req.DagRunId}
		attempt, err = h.dagRunStore.FindAttempt(ctx, ref)
		logger.Info(ctx, "Looking up DAG attempt for cancellation")
	}

	if err != nil {
		logger.Warn(ctx, "Failed to find DAG run for cancellation", tag.Error(err))
		return &coordinatorv1.RequestCancelResponse{
			Accepted: false,
			Error:    fmt.Sprintf("failed to find DAG run: %v", err),
		}, nil
	}

	// Set the abort flag
	if err := attempt.Abort(ctx); err != nil {
		logger.Warn(ctx, "Failed to abort DAG run", tag.Error(err))
		return &coordinatorv1.RequestCancelResponse{
			Accepted: false,
			Error:    fmt.Sprintf("failed to abort: %v", err),
		}, nil
	}

	logger.Info(ctx, "DAG run cancellation requested successfully")
	return &coordinatorv1.RequestCancelResponse{Accepted: true}, nil
}
