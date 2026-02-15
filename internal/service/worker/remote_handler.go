package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/persis/fileagentconfig"
	"github.com/dagu-org/dagu/internal/persis/fileagentmodel"
	"github.com/dagu-org/dagu/internal/persis/filememory"
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
	DAGRunStore exec.DAGRunStore
	// DAGStore is the store for DAG definitions
	DAGStore exec.DAGStore
	// DAGRunMgr is the manager for DAG runs
	DAGRunMgr runtime.Manager
	// ServiceRegistry is the service registry
	ServiceRegistry exec.ServiceRegistry
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
	dagRunStore       exec.DAGRunStore
	dagStore          exec.DAGStore
	dagRunMgr         runtime.Manager
	serviceRegistry   exec.ServiceRegistry
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
		return h.handleRetry(ctx, task)

	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return fmt.Errorf("unsupported operation: unspecified")

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

	root := exec.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}
	parent := exec.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}
	statusPusher, logStreamer := h.createRemoteHandlers(task.DagRunId, dag.Name, root)

	return h.executeDAGRun(ctx, dag, task.DagRunId, task.AttemptId, root, parent, statusPusher, logStreamer, queuedRun, nil)
}

func (h *remoteTaskHandler) handleRetry(ctx context.Context, task *coordinatorv1.Task) error {
	root := exec.DAGRunRef{Name: task.RootDagRunName, ID: task.RootDagRunId}

	if task.PreviousStatus == nil {
		return fmt.Errorf("retry requires previous_status in task for shared-nothing mode")
	}

	status, convErr := convert.ProtoToDAGRunStatus(task.PreviousStatus)
	if convErr != nil {
		return fmt.Errorf("failed to convert previous status: %w", convErr)
	}
	logger.Info(ctx, "Using previous status from task for retry",
		tag.RunID(task.DagRunId),
		slog.Int("nodes", len(status.Nodes)))

	dag, cleanup, err := h.loadDAG(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to load DAG: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	parent := exec.DAGRunRef{Name: task.ParentDagRunName, ID: task.ParentDagRunId}
	statusPusher, logStreamer := h.createRemoteHandlers(task.DagRunId, dag.Name, root)

	return h.executeDAGRun(ctx, dag, task.DagRunId, task.AttemptId, root, parent, statusPusher, logStreamer, false, &retryConfig{
		target:   status,
		stepName: task.Step,
	})
}

// retryConfig holds retry-specific configuration
type retryConfig struct {
	target   *exec.DAGRunStatus
	stepName string
}

// createRemoteHandlers creates the status pusher and log streamer for remote execution.
func (h *remoteTaskHandler) createRemoteHandlers(dagRunID, dagName string, root exec.DAGRunRef) (*remote.StatusPusher, *remote.LogStreamer) {
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

// agentStores creates the agent config, model, and memory stores from the config paths.
func (h *remoteTaskHandler) agentStores(ctx context.Context) (configStore iface.ConfigStore, modelStore iface.ModelStore, memoryStore iface.MemoryStore) {
	acs, err := fileagentconfig.New(h.config.Paths.DataDir)
	if err != nil {
		logger.Warn(ctx, "Failed to create agent config store", tag.Error(err))
		return nil, nil, nil
	}
	if acs == nil {
		return nil, nil, nil
	}

	ams, err := fileagentmodel.New(filepath.Join(h.config.Paths.DataDir, "agent", "models"))
	if err != nil {
		logger.Warn(ctx, "Failed to create agent model store", tag.Error(err))
		return acs, nil, nil
	}

	ms, err := filememory.New(h.config.Paths.DAGsDir)
	if err != nil {
		logger.Warn(ctx, "Failed to create agent memory store", tag.Error(err))
		return acs, ams, nil
	}

	return acs, ams, ms
}

// loadDAG loads the DAG from task definition.
// Returns the loaded DAG and a cleanup function that should be called after task execution.
func (h *remoteTaskHandler) loadDAG(ctx context.Context, task *coordinatorv1.Task) (*core.DAG, func(), error) {
	logger.Info(ctx, "Creating temporary DAG file from definition",
		tag.DAG(task.Target),
		tag.Size(len(task.Definition)))

	tempFile, err := fileutil.CreateTempDAGFile("worker-dags", task.Target, []byte(task.Definition))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp DAG file: %w", err)
	}
	cleanupFunc := func() {
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
		}
	}

	// Prepare load options
	// Note: DAGsDir is intentionally NOT included because:
	// 1. Remote handlers always receive DAG definitions from the coordinator
	// 2. Shared-nothing workers should not access local DAG directories
	loadOpts := []spec.LoadOption{
		spec.WithBaseConfig(h.config.Paths.BaseConfig),
		spec.WithName(task.Target), // Use original DAG name, not temp file path
	}

	// Pass task params to the DAG (e.g., from parallel execution items)
	if task.Params != "" {
		loadOpts = append(loadOpts, spec.WithParams(task.Params))
	}

	dag, err := spec.Load(ctx, tempFile, loadOpts...)
	if err != nil {
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to load DAG from %s: %w", tempFile, err)
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
// Includes workerID in path to prevent collisions with concurrent workers on the same host.
func (h *remoteTaskHandler) createAgentEnv(ctx context.Context, dagRunID string) (*agentEnv, error) {
	logDir := filepath.Join(os.TempDir(), "dagu", "worker-logs", h.workerID, dagRunID)
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
	attemptID string,
	root exec.DAGRunRef,
	parent exec.DAGRunRef,
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

	// Open scheduler log file for writing
	logFile, err := os.OpenFile(env.logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create scheduler log file: %w", err)
	}
	defer func() {
		if closeErr := logFile.Close(); closeErr != nil {
			logger.Warn(ctx, "Failed to close scheduler log file", tag.Error(closeErr))
		}
	}()

	// Create a writer that writes to both local file AND streams to coordinator in real-time.
	// This enables viewing scheduler logs while the DAG is still running.
	var logWriter io.Writer = logFile
	if logStreamer != nil {
		streamingWriter := logStreamer.NewSchedulerLogWriter(ctx, logFile)
		defer func() {
			if closeErr := streamingWriter.Close(); closeErr != nil {
				logger.Warn(ctx, "Failed to close scheduler log streamer", tag.Error(closeErr))
			}
		}()
		logWriter = streamingWriter
	}

	// Configure logger to use the streaming writer
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.WithWriter(logWriter)))

	// Create agent stores for agent step execution
	agentConfigStore, agentModelStore, agentMemoryStore := h.agentStores(ctx)

	// Build agent options
	opts := agent.Options{
		ParentDAGRun:     parent,
		WorkerID:         h.workerID,
		StatusPusher:     statusPusher,
		LogWriterFactory: logStreamer,
		QueuedRun:        queuedRun,
		AttemptID:        attemptID,
		DAGRunStore:      h.dagRunStore,
		ServiceRegistry:  h.serviceRegistry,
		RootDAGRun:       root,
		PeerConfig:       h.peerConfig,
		DefaultExecMode:  h.config.DefaultExecMode,
		AgentConfigStore: agentConfigStore,
		AgentModelStore:  agentModelStore,
		AgentMemoryStore: agentMemoryStore,
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
