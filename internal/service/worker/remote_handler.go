package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/runtime/remote"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var _ TaskHandler = (*remoteTaskHandler)(nil)

// RemoteTaskHandlerConfig contains configuration for the remote task handler
type RemoteTaskHandlerConfig struct {
	// WorkerID is the identifier of this worker
	WorkerID string
	// CoordinatorClient is the coordinator client with load balancing support
	CoordinatorClient coordinator.Client
	// DAGRunStore is the store for DAG run status (may be nil for fully remote mode)
	DAGRunStore execution.DAGRunStore
	// DAGStore is the store for DAG definitions
	DAGStore execution.DAGStore
	// DAGRunMgr is the manager for DAG runs
	DAGRunMgr runtime.Manager
	// ServiceRegistry is the service registry
	ServiceRegistry execution.ServiceRegistry
	// PeerConfig is the peer configuration
	PeerConfig config.Peer
	// Config is the main application configuration
	Config *config.Config
}

// NewRemoteTaskHandler creates a new TaskHandler that runs tasks in-process
// with status pushing and log streaming to the coordinator.
func NewRemoteTaskHandler(cfg RemoteTaskHandlerConfig) TaskHandler {
	return &remoteTaskHandler{
		workerID:          cfg.WorkerID,
		coordinatorClient: cfg.CoordinatorClient,
		dagRunStore:       cfg.DAGRunStore,
		dagStore:          cfg.DAGStore,
		dagRunMgr:         cfg.DAGRunMgr,
		serviceRegistry:   cfg.ServiceRegistry,
		peerConfig:        cfg.PeerConfig,
		config:            cfg.Config,
	}
}

type remoteTaskHandler struct {
	workerID          string
	coordinatorClient coordinator.Client
	dagRunStore       execution.DAGRunStore
	dagStore          execution.DAGStore
	dagRunMgr         runtime.Manager
	serviceRegistry   execution.ServiceRegistry
	peerConfig        config.Peer
	config            *config.Config
}

// Handle executes a task in-process with remote status/log streaming
func (h *remoteTaskHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing remote task",
		slog.String("operation", task.Operation.String()),
		tag.Target(task.Target),
		tag.RunID(task.DagRunId),
		slog.String("root-dag-run-id", task.RootDagRunId),
		slog.String("parent-dag-run-id", task.ParentDagRunId))

	switch task.Operation {
	case coordinatorv1.Operation_OPERATION_START:
		return h.handleStart(ctx, task, false)
	case coordinatorv1.Operation_OPERATION_RETRY:
		// OPERATION_RETRY from the queue processor means "execute this queued item".
		// This is only an actual step retry if a specific Step is specified.
		// Without a Step, it's a fresh start from the queue (queuedRun = true).
		if task.Step == "" {
			return h.handleStart(ctx, task, true)
		}
		return h.handleRetry(ctx, task)
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return fmt.Errorf("unspecified operation")
	default:
		return fmt.Errorf("unsupported operation: %v", task.Operation)
	}
}

func (h *remoteTaskHandler) handleStart(ctx context.Context, task *coordinatorv1.Task, queuedRun bool) error {
	dag, cleanup, err := h.loadDAG(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to load DAG: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	root := execution.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}
	parent := execution.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}
	statusPusher, logStreamer := h.createRemoteHandlers(task.DagRunId, dag.Name, root)

	return h.executeDAGRun(ctx, dag, task.DagRunId, root, parent, statusPusher, logStreamer, queuedRun, nil)
}

func (h *remoteTaskHandler) handleRetry(ctx context.Context, task *coordinatorv1.Task) error {
	root := execution.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}

	// Get previous status - prefer from task (shared-nothing mode), fallback to local store
	var status *execution.DAGRunStatus
	if task.PreviousStatus != nil {
		// Shared-nothing mode: status is provided in the task
		status = convert.ProtoToDAGRunStatus(task.PreviousStatus)
		logger.Info(ctx, "Using previous status from task for retry",
			tag.RunID(task.DagRunId),
			slog.Int("nodes", len(status.Nodes)))
	} else if h.dagRunStore != nil {
		// Fallback: read from local store
		attempt, err := h.dagRunStore.FindAttempt(ctx, execution.NewDAGRunRef(task.RootDagRunName, task.DagRunId))
		if err != nil {
			return fmt.Errorf("failed to find previous run: %w", err)
		}

		var readErr error
		status, readErr = attempt.ReadStatus(ctx)
		if readErr != nil {
			return fmt.Errorf("failed to read previous status: %w", readErr)
		}
	} else {
		return fmt.Errorf("retry requires either previous_status in task or local dagRunStore")
	}

	// Load the DAG - use task definition if provided, otherwise load from store
	dag, cleanup, err := h.loadDAG(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to load DAG: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	parent := execution.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}
	statusPusher, logStreamer := h.createRemoteHandlers(task.DagRunId, dag.Name, root)

	return h.executeDAGRun(ctx, dag, task.DagRunId, root, parent, statusPusher, logStreamer, false, &retryConfig{
		target:   status,
		stepName: task.Step,
	})
}

// retryConfig holds retry-specific configuration
type retryConfig struct {
	target   *execution.DAGRunStatus
	stepName string
}

// createRemoteHandlers creates the status pusher and log streamer for remote execution.
func (h *remoteTaskHandler) createRemoteHandlers(dagRunID, dagName string, root execution.DAGRunRef) (*remote.StatusPusher, *remote.LogStreamer) {
	statusPusher := remote.NewStatusPusher(h.coordinatorClient, h.workerID)
	logStreamer := remote.NewLogStreamer(
		h.coordinatorClient,
		h.workerID,
		dagRunID,
		dagName,
		"", // attemptID will be set by agent after attempt creation
		root,
	)
	return statusPusher, logStreamer
}

// loadDAG loads the DAG from task definition or target path.
// Returns the loaded DAG and a cleanup function that should be called after task execution.
func (h *remoteTaskHandler) loadDAG(ctx context.Context, task *coordinatorv1.Task) (*core.DAG, func(), error) {
	var target string
	var cleanupFunc func()

	// If definition is provided, create a temporary DAG file
	if task.Definition != "" {
		logger.Info(ctx, "Creating temporary DAG file from definition",
			tag.DAG(task.Target),
			tag.Size(len(task.Definition)))

		tempFile, err := fileutil.CreateTempDAGFile("worker-dags", task.Target, []byte(task.Definition))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temp DAG file: %w", err)
		}
		target = tempFile
		cleanupFunc = func() {
			if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
				logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
			}
		}
	} else {
		target = task.Target
	}

	// Prepare load options
	loadOpts := []spec.LoadOption{
		spec.WithBaseConfig(h.config.Paths.BaseConfig),
		spec.WithDAGsDir(h.config.Paths.DAGsDir),
	}

	// Load the DAG
	dag, err := spec.Load(ctx, target, loadOpts...)
	if err != nil {
		if cleanupFunc != nil {
			cleanupFunc()
		}
		return nil, nil, fmt.Errorf("failed to load DAG from %s: %w", target, err)
	}

	return dag, cleanupFunc, nil
}

// agentEnv holds temporary directories and cleanup function for agent execution.
type agentEnv struct {
	logDir  string
	logFile string
	cleanup func()
}

// createAgentEnv creates temporary directories for agent execution.
// The cleanup function must be called after execution completes.
func (h *remoteTaskHandler) createAgentEnv(ctx context.Context, dagRunID string) (*agentEnv, error) {
	logDir := filepath.Join(os.TempDir(), "dagu", "worker-logs", dagRunID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &agentEnv{
		logDir:  logDir,
		logFile: filepath.Join(logDir, "scheduler.log"),
		cleanup: func() {
			if err := os.RemoveAll(logDir); err != nil {
				logger.Warn(ctx, "Failed to cleanup temp log directory",
					slog.String("path", logDir),
					tag.Error(err))
			}
		},
	}, nil
}

func (h *remoteTaskHandler) executeDAGRun(
	ctx context.Context,
	dag *core.DAG,
	dagRunID string,
	root execution.DAGRunRef,
	parent execution.DAGRunRef,
	statusPusher *remote.StatusPusher,
	logStreamer *remote.LogStreamer,
	queuedRun bool,
	retry *retryConfig,
) error {
	// Create temporary directory for local operations
	env, err := h.createAgentEnv(ctx, dagRunID)
	if err != nil {
		return err
	}
	defer env.cleanup()

	// Build agent options
	opts := agent.Options{
		ParentDAGRun:     parent,
		WorkerID:         h.workerID,
		StatusPusher:     statusPusher,
		LogWriterFactory: logStreamer,
		QueuedRun:        queuedRun,
	}

	// Add retry configuration if present
	if retry != nil {
		opts.RetryTarget = retry.target
		opts.StepRetry = retry.stepName
	}

	// Create the agent
	agentInstance := agent.New(
		dagRunID,
		dag,
		env.logDir,
		env.logFile,
		h.dagRunMgr,
		h.dagStore,
		h.dagRunStore,
		h.serviceRegistry,
		root,
		h.peerConfig,
		opts,
	)

	// Run the agent
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "DAG execution failed",
			tag.RunID(dagRunID),
			tag.Error(err))
		return err
	}

	logger.Info(ctx, "DAG execution completed",
		tag.RunID(dagRunID))

	return nil
}
