package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/runtime/remote"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var _ TaskHandler = (*remoteTaskHandler)(nil)

// RemoteTaskHandlerConfig contains configuration for the remote task handler
type RemoteTaskHandlerConfig struct {
	// WorkerID is the identifier of this worker
	WorkerID string
	// CoordinatorClient is the gRPC client to communicate with the coordinator
	CoordinatorClient coordinatorv1.CoordinatorServiceClient
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
	coordinatorClient coordinatorv1.CoordinatorServiceClient
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
		return h.handleStart(ctx, task)
	case coordinatorv1.Operation_OPERATION_RETRY:
		return h.handleRetry(ctx, task)
	default:
		return fmt.Errorf("unsupported operation: %v", task.Operation)
	}
}

func (h *remoteTaskHandler) handleStart(ctx context.Context, task *coordinatorv1.Task) error {
	// Load the DAG
	dag, err := h.loadDAG(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to load DAG: %w", err)
	}

	// Parse references
	root := execution.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}
	parent := execution.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}

	// Create status pusher
	statusPusher := remote.NewStatusPusher(h.coordinatorClient, h.workerID)

	// Create log streamer
	logStreamer := remote.NewLogStreamer(
		h.coordinatorClient,
		h.workerID,
		task.DagRunId,
		dag.Name,
		"", // attemptID will be set later
		root,
	)

	// Create and run the agent
	return h.executeDAGRun(ctx, dag, task.DagRunId, root, parent, statusPusher, logStreamer)
}

func (h *remoteTaskHandler) handleRetry(ctx context.Context, task *coordinatorv1.Task) error {
	// For retry, we need to find the previous run and retry it
	root := execution.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}

	// Find the previous attempt
	attempt, err := h.dagRunStore.FindAttempt(ctx, execution.NewDAGRunRef(task.RootDagRunName, task.DagRunId))
	if err != nil {
		return fmt.Errorf("failed to find previous run: %w", err)
	}

	// Read the previous status
	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous status: %w", err)
	}

	// Read the DAG snapshot from the previous run
	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from previous run: %w", err)
	}

	// Create status pusher
	statusPusher := remote.NewStatusPusher(h.coordinatorClient, h.workerID)

	// Create log streamer
	logStreamer := remote.NewLogStreamer(
		h.coordinatorClient,
		h.workerID,
		task.DagRunId,
		dag.Name,
		"", // attemptID will be set later
		root,
	)

	parent := execution.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}

	// Create and run the agent with retry target
	return h.executeRetry(ctx, dag, status, root, parent, task.Step, statusPusher, logStreamer)
}

func (h *remoteTaskHandler) loadDAG(ctx context.Context, task *coordinatorv1.Task) (*core.DAG, error) {
	var target string
	var cleanupFunc func()

	// If definition is provided, create a temporary DAG file
	if task.Definition != "" {
		logger.Info(ctx, "Creating temporary DAG file from definition",
			tag.DAG(task.Target),
			tag.Size(len(task.Definition)))

		tempFile, err := createTempDAGFile(task.Target, []byte(task.Definition))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp DAG file: %w", err)
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
		return nil, fmt.Errorf("failed to load DAG from %s: %w", target, err)
	}

	// Schedule cleanup after DAG is loaded (actual cleanup happens when task completes)
	if cleanupFunc != nil {
		defer cleanupFunc()
	}

	return dag, nil
}

func (h *remoteTaskHandler) executeDAGRun(
	ctx context.Context,
	dag *core.DAG,
	dagRunID string,
	root execution.DAGRunRef,
	parent execution.DAGRunRef,
	statusPusher *remote.StatusPusher,
	logStreamer *remote.LogStreamer,
) error {
	// For remote mode, we don't write logs locally
	// Create a temporary directory for any local operations
	logDir := filepath.Join(os.TempDir(), "dagu", "worker-logs", dagRunID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logFile := filepath.Join(logDir, "scheduler.log")

	// Create the agent with remote status pusher and log streamer
	agentInstance := agent.New(
		dagRunID,
		dag,
		logDir,
		logFile,
		h.dagRunMgr,
		h.dagStore,
		h.dagRunStore,
		h.serviceRegistry,
		root,
		h.peerConfig,
		agent.Options{
			ParentDAGRun:     parent,
			WorkerID:         h.workerID,
			StatusPusher:     statusPusher,
			LogWriterFactory: logStreamer,
		},
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

func (h *remoteTaskHandler) executeRetry(
	ctx context.Context,
	dag *core.DAG,
	retryTarget *execution.DAGRunStatus,
	root execution.DAGRunRef,
	parent execution.DAGRunRef,
	stepName string,
	statusPusher *remote.StatusPusher,
	logStreamer *remote.LogStreamer,
) error {
	dagRunID := retryTarget.DAGRunID

	// For remote mode, we don't write logs locally
	logDir := filepath.Join(os.TempDir(), "dagu", "worker-logs", dagRunID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logFile := filepath.Join(logDir, "scheduler.log")

	// Create the agent with retry target and remote status pusher
	agentInstance := agent.New(
		dagRunID,
		dag,
		logDir,
		logFile,
		h.dagRunMgr,
		h.dagStore,
		h.dagRunStore,
		h.serviceRegistry,
		root,
		h.peerConfig,
		agent.Options{
			ParentDAGRun:     parent,
			RetryTarget:      retryTarget,
			StepRetry:        stepName,
			WorkerID:         h.workerID,
			StatusPusher:     statusPusher,
			LogWriterFactory: logStreamer,
		},
	)

	// Run the agent
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "DAG retry execution failed",
			tag.RunID(dagRunID),
			tag.Error(err))
		return err
	}

	logger.Info(ctx, "DAG retry execution completed",
		tag.RunID(dagRunID))

	return nil
}
