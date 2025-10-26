package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/mailer"
	"github.com/dagu-org/dagu/internal/common/secrets"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/common/sock"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	runtime1 "github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/builtin/docker"
	"github.com/dagu-org/dagu/internal/runtime/builtin/ssh"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	_ "github.com/dagu-org/dagu/internal/runtime/builtin"
)

// Agent is responsible for running the DAG and handling communication
// via the unix socket. The agent performs the following tasks:
// 1. Start the DAG.
// 2. Propagate a signal to the running processes.
// 3. Handle the HTTP request via the unix socket.
// 4. Write the log and status to the data store.
type Agent struct {
	lock sync.RWMutex

	// dry indicates if the agent is running in dry-run mode.
	dry bool

	// retryTarget is the target status to retry the DAG.
	// It is nil if it's not a retry execution.
	retryTarget *execution.DAGRunStatus

	// dagStore is the database to store the DAG definitions.
	dagStore execution.DAGStore

	// dagRunStore is the database to store the run history.
	dagRunStore execution.DAGRunStore

	// registry is the service registry to find the coordinator service.
	registry execution.ServiceRegistry

	// peerConfig is the configuration for the peer connections.
	peerConfig config.Peer

	// dagRunMgr is the runstore dagRunMgr to communicate with the history.
	dagRunMgr runtime1.Manager

	// scheduler is the scheduler instance to run the DAG.
	scheduler *runtime.Scheduler

	// graph is the execution graph for the DAG.
	graph *runtime.ExecutionGraph

	// reporter is responsible for sending the report to the user.
	reporter *reporter

	// socketServer is the unix socket server to handle HTTP requests.
	// It listens to the requests from the local client (e.g., frontend server).
	socketServer *sock.Server

	// logDir is the directory to store the log files for each node in the DAG.
	logDir string

	// logFile is the file to write the scheduler log.
	logFile string

	// dag is the DAG to run.
	dag *core.DAG

	// rootDAGRun indicates the root dag-run of the current dag-run.
	// If the current dag-run is the root dag-run, it is the same as the current
	// DAG name and dag-run ID.
	rootDAGRun execution.DAGRunRef

	// parentDAGRun is the execution reference of the parent dag-run.
	parentDAGRun execution.DAGRunRef

	// dagRunID is the ID for the current dag-run.
	dagRunID string

	// dagRunAttemptID is the ID for the current dag-run attempt.
	dagRunAttemptID string

	// finished is true if the dag-run is finished.
	finished atomic.Bool

	// lastErr is the last error occurred during the dag-run.
	lastErr error

	// isSubDAGRun is true if the current dag-run is not the root dag-run,
	// meaning that it is a sub dag-run of another dag-run.
	isSubDAGRun atomic.Bool

	// progressDisplay is the progress display for showing real-time execution progress.
	progressDisplay ProgressReporter

	// stepRetry is the name of the step to retry, if specified.
	stepRetry string

	// tracer is the OpenTelemetry tracer for the agent.
	tracer *telemetry.Tracer
}

// Options is the configuration for the Agent.
type Options struct {
	// Dry is a dry-run mode. It does not execute the actual command.
	// Dry run does not create runstore data.
	Dry bool
	// RetryTarget is the target status (runstore of execution) to retry.
	// If it's specified the agent will execute the DAG with the same
	// configuration as the specified history.
	RetryTarget *execution.DAGRunStatus
	// ParentDAGRun is the dag-run reference of the parent dag-run.
	// It is required for sub dag-runs to identify the parent dag-run.
	ParentDAGRun execution.DAGRunRef
	// ProgressDisplay indicates if the progress display should be shown.
	// This is typically enabled for CLI execution in a TTY environment.
	ProgressDisplay bool
	// StepRetry is the name of the step to retry, if specified.
	StepRetry string
}

// New creates a new Agent.
func New(
	dagRunID string,
	dag *core.DAG,
	logDir string,
	logFile string,
	drm runtime1.Manager,
	ds execution.DAGStore,
	drs execution.DAGRunStore,
	reg execution.ServiceRegistry,
	root execution.DAGRunRef,
	peerConfig config.Peer,
	opts Options,
) *Agent {
	a := &Agent{
		rootDAGRun:   root,
		parentDAGRun: opts.ParentDAGRun,
		dagRunID:     dagRunID,
		dag:          dag,
		dry:          opts.Dry,
		retryTarget:  opts.RetryTarget,
		logDir:       logDir,
		logFile:      logFile,
		dagRunMgr:    drm,
		dagStore:     ds,
		dagRunStore:  drs,
		registry:     reg,
		stepRetry:    opts.StepRetry,
		peerConfig:   peerConfig,
	}

	// Initialize progress display if enabled
	if opts.ProgressDisplay {
		a.progressDisplay = createProgressReporter(dag, dagRunID, dag.Params)
	}

	return a
}

// Run setups the scheduler and runs the DAG.
func (a *Agent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize propagators for W3C trace context before anything else
	telemetry.InitializePropagators()

	// Extract trace context from environment variables if present
	// This must be done BEFORE initializing the tracer so sub DAGs
	// can continue the parent's trace
	if a.dag.OTel != nil && a.dag.OTel.Enabled {
		ctx = telemetry.ExtractTraceContext(ctx)
	}

	// Initialize OpenTelemetry tracer
	tracer, err := telemetry.NewTracer(ctx, a.dag)
	if err != nil {
		logger.Warn(ctx, "Failed to initialize OpenTelemetry tracer", "err", err)
		// Continue without tracing
	} else {
		a.tracer = tracer
		defer func() {
			if err := tracer.Shutdown(ctx); err != nil {
				logger.Warn(ctx, "Failed to shutdown OpenTelemetry tracer", "err", err)
			}
		}()
	}

	// Start root span for DAG execution
	var span trace.Span
	if a.tracer != nil && a.tracer.IsEnabled() {
		spanAttrs := []attribute.KeyValue{
			attribute.String("dag.name", a.dag.Name),
			attribute.String("dag.run_id", a.dagRunID),
		}
		if a.parentDAGRun.Name != "" {
			spanAttrs = append(spanAttrs, attribute.String("dag.parent_run_id", a.parentDAGRun.ID))
			spanAttrs = append(spanAttrs, attribute.String("dag.parent_name", a.parentDAGRun.Name))
		}

		// For sub DAGs, ensure we're creating the span as a child of the parent context
		spanName := fmt.Sprintf("DAG: %s", a.dag.Name)
		ctx, span = a.tracer.Start(ctx, spanName, trace.WithAttributes(spanAttrs...))
		defer func() {
			// Set final status
			status := a.Status(ctx)
			span.SetAttributes(attribute.String("dag.status", status.Status.String()))
			span.End()
		}()
	}

	if a.rootDAGRun.ID != a.dagRunID {
		logger.Debug(ctx, "Initiating a sub dag-run", "root-run", a.rootDAGRun.String(), "parent-run", a.parentDAGRun.String())
		a.isSubDAGRun.Store(true)
		if a.parentDAGRun.Zero() {
			return fmt.Errorf("parent dag-run is not specified for the sub dag-run %s", a.dagRunID)
		}
	}

	var attempt execution.DAGRunAttempt

	// Check if the DAG is already running.
	if err := a.checkIsAlreadyRunning(ctx); err != nil {
		return err
	}

	if !a.dry {
		// Setup the attempt for the dag-run.
		// It's not required for dry-run mode.
		att, err := a.setupDAGRunAttempt(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup execution history: %w", err)
		}
		attempt = att
		a.dagRunAttemptID = attempt.ID()
	}

	// Initialize the scheduler
	a.scheduler = a.newScheduler()

	// Setup the reporter to send the report to the user.
	a.setupReporter(ctx)

	// Setup the execution graph for the DAG.
	if err := a.setupGraph(ctx); err != nil {
		return fmt.Errorf("failed to setup execution graph: %w", err)
	}

	// Create a new environment for the dag-run.
	dbClient := newDBClient(a.dagRunStore, a.dagStore)

	// Initialize coordinator client factory for distributed execution
	coordinatorCli := a.createCoordinatorClient(ctx)

	// Resolve secrets if defined
	secretEnvs, secretErr := a.resolveSecrets(ctx)

	ctx = execution.SetupDAGContext(ctx, a.dag, dbClient, a.rootDAGRun, a.dagRunID, a.logFile, a.dag.Params, coordinatorCli, secretEnvs)

	// Add structured logging context
	logFields := []any{"dag", a.dag.Name, "dagRunId", a.dagRunID}
	if a.isSubDAGRun.Load() {
		logFields = append(logFields, "root", a.rootDAGRun.String(), "parent", a.parentDAGRun.String())
	}
	ctx = logger.WithValues(ctx, logFields...)

	// Handle dry execution.
	if a.dry {
		return a.dryRun(ctx)
	}

	// initErr is used to capture any initialization errors that occur
	// before the agent starts running the DAG.
	var initErr error

	// Open the run file to write the status.
	// TODO: Check if the run file already exists and if it does, return an error.
	// This is to prevent duplicate execution of the same DAG run.
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open execution history: %w", err)
	}

	// Update the status to running
	st := a.Status(ctx)
	st.Status = core.Running
	if err := attempt.Write(ctx, st); err != nil {
		logger.Error(ctx, "Status write failed", "err", err)
	}

	defer func() {
		if initErr != nil {
			logger.Error(ctx, "Failed to initialize DAG execution", "err", err)
			st := a.Status(ctx)
			st.Status = core.Failed
			if err := attempt.Write(ctx, st); err != nil {
				logger.Error(ctx, "Status write failed", "err", err)
			}
		}
		if err := attempt.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close runstore store", "err", err)
		}
	}()

	// If there was an error resolving secrets, stop execution here
	if secretErr != nil {
		initErr = secretErr // Stop execution if secret resolution failed
		return initErr
	}

	if err := attempt.Write(ctx, a.Status(ctx)); err != nil {
		logger.Error(ctx, "Failed to write status", "err", err)
	}

	// Start the unix socket server for receiving HTTP requests from
	// the local client (e.g., the frontend server, scheduler, etc).
	if err := a.setupSocketServer(ctx); err != nil {
		return fmt.Errorf("failed to setup unix socket server: %w", err)
	}

	// Create a new container if the DAG has a container configuration.
	if a.dag.Container != nil {
		ctCfg, err := docker.LoadConfig(a.dag.WorkingDir, *a.dag.Container, a.dag.RegistryAuths)
		if err != nil {
			initErr = fmt.Errorf("failed to load container config: %w", err)
			return initErr
		}
		ctCli, err := docker.InitializeClient(ctx, ctCfg)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize container client: %w", err)
			return initErr
		}
		if err := ctCli.CreateContainerKeepAlive(ctx); err != nil {
			initErr = fmt.Errorf("failed to create keepalive container: %w", err)
			return initErr
		}

		// Set the container client in the context for the execution.
		ctx = docker.WithContainerClient(ctx, ctCli)

		defer func() {
			ctCli.StopContainerKeepAlive(ctx)
			ctCli.Close(ctx)
		}()
	}

	// Create SSH Client if the DAG has SSH configuration.
	if a.dag.SSH != nil {
		sshConfig, err := cmdutil.EvalObject(ctx, ssh.Config{
			User:          a.dag.SSH.User,
			Host:          a.dag.SSH.Host,
			Port:          a.dag.SSH.Port,
			Key:           a.dag.SSH.Key,
			Password:      a.dag.SSH.Password,
			StrictHostKey: a.dag.SSH.StrictHostKey,
			KnownHostFile: a.dag.SSH.KnownHostFile,
		}, a.dag.ParamsMap())
		if err != nil {
			initErr = fmt.Errorf("failed to evaluate ssh config: %w", err)
			return initErr
		}
		cli, err := ssh.NewClient(&sshConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to create ssh client: %w", err)
			return initErr
		}
		ctx = ssh.WithSSHClient(ctx, cli)
	}

	listenerErrCh := make(chan error)
	go execWithRecovery(ctx, func() {
		err := a.socketServer.Serve(ctx, listenerErrCh)
		if err != nil && !errors.Is(err, sock.ErrServerRequestedShutdown) {
			logger.Error(ctx, "Failed to start socket frontend", "err", err)
		}
	})

	// Stop the socket server when the dag-run is finished.
	defer func() {
		if err := a.socketServer.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Failed to shutdown socket frontend", "err", err)
		}
	}()

	// It returns error if it failed to start the unix socket server.
	if err := <-listenerErrCh; err != nil {
		initErr = fmt.Errorf("failed to start the unix socket server: %w", err)
		return initErr
	}

	// Start progress display if enabled
	if a.progressDisplay != nil {
		a.progressDisplay.Start()
		// Don't defer Stop() here - we'll do it after all updates are processed
	}

	// Setup channels to receive status updates for each node in the DAG.
	// It should receive node instance when the node status changes, for
	// example, when started, stopped, or cancelled, etc.
	progressCh := make(chan *runtime.Node)
	progressDone := make(chan struct{})
	defer func() {
		close(progressCh)
		<-progressDone // Wait for progress updates to complete
		if a.progressDisplay != nil {
			// Give a small delay to ensure final render
			time.Sleep(100 * time.Millisecond)
			a.progressDisplay.Stop()
		}
	}()
	go execWithRecovery(ctx, func() {
		defer close(progressDone)
		for node := range progressCh {
			status := a.Status(ctx)
			if err := attempt.Write(ctx, status); err != nil {
				logger.Error(ctx, "Failed to write status", "err", err)
			}
			if err := a.reporter.reportStep(ctx, a.dag, status, node); err != nil {
				logger.Error(ctx, "Failed to report step", "err", err)
			}
			// Update progress display if enabled
			if a.progressDisplay != nil {
				// Convert scheduler node to models node
				nodeData := node.NodeData()
				modelNode := a.nodeToModelNode(nodeData)
				a.progressDisplay.UpdateNode(modelNode)
				a.progressDisplay.UpdateStatus(&status)
			}
		}
	})

	// Write the first status just after the start to store the running status.
	// If the DAG is already finished, skip it.
	go execWithRecovery(ctx, func() {
		time.Sleep(waitForRunning)
		if a.finished.Load() {
			return
		}
		if err := attempt.Write(ctx, a.Status(ctx)); err != nil {
			logger.Error(ctx, "Status write failed", "err", err)
		}
	})

	// Start the dag-run.
	if a.retryTarget != nil {
		logger.Info(ctx, "dag-run retry started",
			"name", a.dag.Name,
			"dagRunId", a.dagRunID,
			"attemptID", a.dagRunAttemptID,
			"retryTargetAttemptID", a.retryTarget.AttemptID,
		)
	} else {
		logger.Info(ctx, "dag-run started",
			"name", a.dag.Name,
			"dagRunId", a.dagRunID,
			"attemptID", a.dagRunAttemptID,
			"params", a.dag.Params,
		)
	}

	// Start watching for cancel requests
	go execWithRecovery(ctx, func() {
		a.watchCancelRequested(ctx, attempt)
	})

	// Add registry authentication to context for docker executors
	if len(a.dag.RegistryAuths) > 0 {
		ctx = docker.WithRegistryAuth(ctx, a.dag.RegistryAuths)
	}

	lastErr := a.scheduler.Schedule(ctx, a.graph, progressCh)

	if coordinatorCli != nil {
		// Cleanup the coordinator client resources if it was created.
		if err := coordinatorCli.Cleanup(ctx); err != nil {
			logger.Warn(ctx, "Failed to cleanup coordinator client", "err", err)
		}
	}

	// Update the finished status to the runstore database.
	finishedStatus := a.Status(ctx)

	// Send final progress update if enabled
	if a.progressDisplay != nil {
		// Update all nodes with their final status
		for _, node := range finishedStatus.Nodes {
			a.progressDisplay.UpdateNode(node)
		}
		a.progressDisplay.UpdateStatus(&finishedStatus)
	}

	// Log execution summary
	logger.Info(ctx, "dag-run finished",
		"name", a.dag.Name,
		"dagRunId", a.dagRunID,
		"attemptID", a.dagRunAttemptID,
		"status", finishedStatus.Status.String(),
		"startedAt", finishedStatus.StartedAt,
		"finishedAt", finishedStatus.FinishedAt,
	)

	if err := attempt.Write(ctx, a.Status(ctx)); err != nil {
		logger.Error(ctx, "Status write failed", "err", err)
	}

	// Send the execution report if necessary.
	a.lastErr = lastErr
	if err := a.reporter.send(ctx, a.dag, finishedStatus, lastErr); err != nil {
		logger.Error(ctx, "Mail notification failed", "err", err)
	}

	// Mark the agent finished.
	a.finished.Store(true)

	// Return the last error on the dag-run.
	return lastErr
}

// nodeToModelNode converts a scheduler NodeData to a models.Node
func (a *Agent) nodeToModelNode(nodeData runtime.NodeData) *execution.Node {
	subRuns := make([]execution.SubDAGRun, len(nodeData.State.SubRuns))
	for i, child := range nodeData.State.SubRuns {
		subRuns[i] = execution.SubDAGRun(child)
	}

	var errText string
	if nodeData.State.Error != nil {
		errText = nodeData.State.Error.Error()
	}

	return &execution.Node{
		Step:            nodeData.Step,
		Stdout:          nodeData.State.Stdout,
		Stderr:          nodeData.State.Stderr,
		StartedAt:       stringutil.FormatTime(nodeData.State.StartedAt),
		FinishedAt:      stringutil.FormatTime(nodeData.State.FinishedAt),
		Status:          nodeData.State.Status,
		RetriedAt:       stringutil.FormatTime(nodeData.State.RetriedAt),
		RetryCount:      nodeData.State.RetryCount,
		DoneCount:       nodeData.State.DoneCount,
		Error:           errText,
		SubRuns:         subRuns,
		OutputVariables: nodeData.State.OutputVariables,
	}
}

func (a *Agent) PrintSummary(ctx context.Context) {
	// Don't print summary if progress display was shown
	if a.progressDisplay != nil {
		return
	}
	status := a.Status(ctx)
	summary := a.reporter.getSummary(ctx, status, a.lastErr)
	println(summary)
}

// Status collects the current running status of the DAG and returns it.
func (a *Agent) Status(ctx context.Context) execution.DAGRunStatus {
	// Lock to avoid race condition.
	a.lock.RLock()
	defer a.lock.RUnlock()

	schedulerStatus := a.scheduler.Status(ctx, a.graph)
	if schedulerStatus == core.NotStarted && a.graph.IsStarted() {
		// Match the status to the execution graph.
		schedulerStatus = core.Running
	}

	opts := []transform.StatusOption{
		transform.WithFinishedAt(a.graph.FinishAt()),
		transform.WithNodes(a.graph.NodeData()),
		transform.WithLogFilePath(a.logFile),
		transform.WithOnExitNode(a.scheduler.HandlerNode(core.HandlerOnExit)),
		transform.WithOnSuccessNode(a.scheduler.HandlerNode(core.HandlerOnSuccess)),
		transform.WithOnFailureNode(a.scheduler.HandlerNode(core.HandlerOnFailure)),
		transform.WithOnCancelNode(a.scheduler.HandlerNode(core.HandlerOnCancel)),
		transform.WithAttemptID(a.dagRunAttemptID),
		transform.WithHierarchyRefs(a.rootDAGRun, a.parentDAGRun),
		transform.WithPreconditions(a.dag.Preconditions),
	}

	// If the current execution is a retry, we need to copy some data
	// from the retry target to the current status.
	if a.retryTarget != nil {
		opts = append(opts, transform.WithQueuedAt(a.retryTarget.QueuedAt))
		opts = append(opts, transform.WithCreatedAt(a.retryTarget.CreatedAt))
	}

	// Create the status object to record the current status.
	return transform.NewStatusBuilder(a.dag).
		Create(
			a.dagRunID,
			schedulerStatus,
			os.Getpid(),
			a.graph.StartAt(),
			opts...,
		)
}

// watchCancelRequested is a goroutine that watches for cancel requests
func (a *Agent) watchCancelRequested(ctx context.Context, attempt execution.DAGRunAttempt) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cancelled, _ := attempt.CancelRequested(ctx); cancelled {
				a.signal(ctx, syscall.SIGTERM, true)
			}
		}
	}
}

// Signal sends the signal to the processes running
func (a *Agent) Signal(ctx context.Context, sig os.Signal) {
	a.signal(ctx, sig, false)
}

// wait before read the running status
const waitForRunning = time.Millisecond * 100

// Simple regular expressions for request routing
var (
	statusRe = regexp.MustCompile(`^/status[/]?$`)
	stopRe   = regexp.MustCompile(`^/stop[/]?$`)
)

// HandleHTTP handles HTTP requests via unix socket.
func (a *Agent) HandleHTTP(ctx context.Context) sock.HTTPHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.Method == http.MethodGet && statusRe.MatchString(r.URL.Path):
			// Return the current status of the dag-run.
			dagStatus := a.Status(ctx)
			dagStatus.Status = core.Running
			statusJSON, err := json.Marshal(dagStatus)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(statusJSON)
		case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
			// Handle Stop request for the dag-run.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			go func() {
				logger.Info(ctx, "Stop request received")
				a.signal(ctx, syscall.SIGTERM, true)
			}()
		default:
			// Unknown request
			encodeError(
				w, &httpError{Code: http.StatusNotFound, Message: "Not found"},
			)
		}
	}
}

// setupReporter setups the reporter to send the report to the user.
func (a *Agent) setupReporter(ctx context.Context) {
	// Lock to prevent race condition.
	a.lock.Lock()
	defer a.lock.Unlock()

	var senderFn SenderFn
	if a.dag.SMTP != nil {
		senderFn = mailer.New(mailer.Config{
			Host:     a.dag.SMTP.Host,
			Port:     a.dag.SMTP.Port,
			Username: a.dag.SMTP.Username,
			Password: a.dag.SMTP.Password,
		}).Send
	} else {
		senderFn = func(ctx context.Context, _ string, _ []string, subject, _ string, _ []string) error {
			logger.Debug(ctx, "Mail notification is disabled", "subject", subject)
			return nil
		}
	}

	a.reporter = newReporter(senderFn)
}

// newScheduler creates a scheduler instance for the dag-run.
func (a *Agent) newScheduler() *runtime.Scheduler {
	// schedulerLogDir is the directory to store the log files for each node in the dag-run.
	const dateTimeFormatUTC = "20060102_150405Z"
	ts := time.Now().UTC().Format(dateTimeFormatUTC)
	schedulerLogDir := filepath.Join(a.logDir, "run_"+ts+"_"+a.dagRunAttemptID)

	cfg := &runtime.Config{
		LogDir:         schedulerLogDir,
		MaxActiveSteps: a.dag.MaxActiveSteps,
		Timeout:        a.dag.Timeout,
		Delay:          a.dag.Delay,
		Dry:            a.dry,
		DAGRunID:       a.dagRunID,
	}

	if a.dag.HandlerOn.Exit != nil {
		cfg.OnExit = a.dag.HandlerOn.Exit
	}

	if a.dag.HandlerOn.Success != nil {
		cfg.OnSuccess = a.dag.HandlerOn.Success
	}

	if a.dag.HandlerOn.Failure != nil {
		cfg.OnFailure = a.dag.HandlerOn.Failure
	}

	if a.dag.HandlerOn.Cancel != nil {
		cfg.OnCancel = a.dag.HandlerOn.Cancel
	}

	return runtime.New(cfg)
}

// createCoordinatorClient creates a coordinator client factory for distributed execution
func (a *Agent) createCoordinatorClient(ctx context.Context) execution.Dispatcher {
	if a.registry == nil {
		logger.Debug(ctx, "Service monitor is not configured, skipping coordinator client creation")
		return nil
	}

	// Create and configure factory
	coordinatorCliCfg := coordinator.DefaultConfig()
	coordinatorCliCfg.MaxRetries = 50

	// Configure the coordinator client based on the global configuration
	coordinatorCliCfg.CAFile = a.peerConfig.ClientCaFile
	coordinatorCliCfg.CertFile = a.peerConfig.CertFile
	coordinatorCliCfg.KeyFile = a.peerConfig.KeyFile
	coordinatorCliCfg.SkipTLSVerify = a.peerConfig.SkipTLSVerify
	coordinatorCliCfg.Insecure = a.peerConfig.Insecure

	return coordinator.New(a.registry, coordinatorCliCfg)
}

// resolveSecrets resolves all secrets defined in the DAG and returns them as
// environment variable strings in "NAME=value" format.
// Returns an empty slice if no secrets are defined.
func (a *Agent) resolveSecrets(ctx context.Context) ([]string, error) {
	if len(a.dag.Secrets) == 0 {
		return nil, nil
	}

	logger.Info(ctx, "Resolving secrets", "count", len(a.dag.Secrets))

	// Create secret registry - all providers auto-registered via init()
	// File provider tries base directories in order:
	// 1. DAG working directory (if set)
	// 2. Directory containing the DAG file (if Location is set)
	baseDirs := []string{a.dag.WorkingDir}
	if a.dag.Location != "" {
		dagDir := filepath.Dir(a.dag.Location)
		baseDirs = append(baseDirs, dagDir)
	}
	secretRegistry := secrets.NewRegistry(baseDirs...)

	// Resolve all secrets - providers handle their own configuration
	resolvedSecrets, err := secretRegistry.ResolveAll(ctx, a.dag.Secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	logger.Debug(ctx, "Secrets resolved successfully", "count", len(resolvedSecrets))
	return resolvedSecrets, nil
}

// dryRun performs a dry-run of the DAG. It only simulates the execution of
// the DAG without running the actual command.
func (a *Agent) dryRun(ctx context.Context) error {
	// progressCh channel receives the node when the node is progressCh.
	// It's a way to update the status in real-time in efficient manner.
	progressCh := make(chan *runtime.Node)
	defer func() {
		close(progressCh)
	}()

	go func() {
		for node := range progressCh {
			status := a.Status(ctx)
			_ = a.reporter.reportStep(ctx, a.dag, status, node)
		}
	}()

	db := newDBClient(a.dagRunStore, a.dagStore)
	dagCtx := execution.SetupDAGContext(ctx, a.dag, db, a.rootDAGRun, a.dagRunID, a.logFile, a.dag.Params, nil, nil)
	lastErr := a.scheduler.Schedule(dagCtx, a.graph, progressCh)
	a.lastErr = lastErr

	logger.Info(ctx, "Dry-run completed", "params", a.dag.Params)

	return lastErr
}

// signal propagates the received signal to the all running child processes.
// allowOverride parameters is used to specify if a node can override
// the signal to send to the process, in case the node is configured
// to send a custom signal (e.g., SIGSTOP instead of SIGTERM).
// The reason we need this is to allow the system to kill the child
// process by sending a SIGKILL to force the process to be shutdown.
// if processes do not terminate after MaxCleanUp time, it sends KILL signal.
func (a *Agent) signal(ctx context.Context, sig os.Signal, allowOverride bool) {
	logger.Info(ctx, "Sending signal to running child processes",
		"signal", sig.String(),
		"allowOverride", allowOverride,
		"maxCleanupTime", a.dag.MaxCleanUpTime/time.Second)

	if !signal.IsTerminationSignalOS(sig) {
		// For non-termination signals, just send the signal once and return.
		a.scheduler.Signal(ctx, a.graph, sig, nil, allowOverride)
		return
	}

	signalCtx, cancel := context.WithTimeout(ctx, a.dag.MaxCleanUpTime)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		a.scheduler.Signal(ctx, a.graph, sig, done, allowOverride)
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			logger.Info(ctx, "All child processes have been terminated")
			return

		case <-signalCtx.Done():
			logger.Info(ctx, "Max cleanup time reached, sending SIGKILL to force termination")
			// Force kill with SIGKILL and don't wait for completion
			a.scheduler.Signal(ctx, a.graph, syscall.SIGKILL, nil, false)
			return

		case <-ticker.C:
			logger.Info(ctx, "Resending signal to processes that haven't terminated",
				"signal", sig.String())
			a.scheduler.Signal(ctx, a.graph, sig, nil, false)

		case <-time.After(500 * time.Millisecond):
			// Quick check to avoid busy waiting, but still responsive
			if a.graph != nil && !a.graph.IsRunning() {
				logger.Info(ctx, "No running processes detected, termination complete")
				return
			}
		}
	}
}

// setupGraph setups the DAG graph. If is retry execution, it loads nodes
// from the retry node so that it runs the same DAG as the previous run.
func (a *Agent) setupGraph(ctx context.Context) error {
	if a.retryTarget != nil {
		return a.setupGraphForRetry(ctx)
	}
	graph, err := runtime.NewExecutionGraph(a.dag.Steps...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

// setupGraphForRetry setsup the graph for retry.
func (a *Agent) setupGraphForRetry(ctx context.Context) error {
	nodes := make([]*runtime.Node, 0, len(a.retryTarget.Nodes))
	for _, n := range a.retryTarget.Nodes {
		nodes = append(nodes, transform.ToNode(n))
	}
	if a.stepRetry != "" {
		return a.setupStepRetryGraph(ctx, nodes)
	}
	return a.setupDefaultRetryGraph(ctx, nodes)
}

// setupStepRetryGraph sets up the graph for retrying a specific step.
func (a *Agent) setupStepRetryGraph(ctx context.Context, nodes []*runtime.Node) error {
	graph, err := runtime.CreateStepRetryGraph(a.dag, nodes, a.stepRetry)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

// setupDefaultRetryGraph sets up the graph for the default retry behavior (all failed/canceled nodes and downstreams).
func (a *Agent) setupDefaultRetryGraph(ctx context.Context, nodes []*runtime.Node) error {
	graph, err := runtime.CreateRetryExecutionGraph(ctx, a.dag, nodes...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

func (a *Agent) setupDAGRunAttempt(ctx context.Context) (execution.DAGRunAttempt, error) {
	retentionDays := a.dag.HistRetentionDays
	if err := a.dagRunStore.RemoveOldDAGRuns(ctx, a.dag.Name, retentionDays); err != nil {
		logger.Error(ctx, "dag-runs data cleanup failed", "err", err)
	}

	opts := execution.NewDAGRunAttemptOptions{Retry: a.retryTarget != nil}
	if a.isSubDAGRun.Load() {
		opts.RootDAGRun = &a.rootDAGRun
	}

	return a.dagRunStore.CreateAttempt(ctx, a.dag, time.Now(), a.dagRunID, opts)
}

// setupSocketServer create socket server instance.
func (a *Agent) setupSocketServer(ctx context.Context) error {
	var socketAddr string
	if a.isSubDAGRun.Load() {
		// Use separate socket address for child
		socketAddr = a.dag.SockAddrForSubDAGRun(a.dagRunID)
	} else {
		socketAddr = a.dag.SockAddr(a.dagRunID)
	}
	socketServer, err := sock.NewServer(socketAddr, a.HandleHTTP(ctx))
	if err != nil {
		return err
	}
	a.socketServer = socketServer
	return nil
}

// checkIsAlreadyRunning returns error if the DAG is already running.
func (a *Agent) checkIsAlreadyRunning(ctx context.Context) error {
	if a.isSubDAGRun.Load() {
		return nil // Skip the check for sub dag-runs
	}
	if a.dagRunMgr.IsRunning(ctx, a.dag, a.dagRunID) {
		return fmt.Errorf("already running. dag-run ID=%s, socket=%s", a.dagRunID, a.dag.SockAddr(a.dagRunID))
	}
	return nil
}

// execWithRecovery executes a function with panic recovery and detailed error reporting
// It captures stack traces and provides structured error information for debugging
func execWithRecovery(ctx context.Context, fn func()) {
	defer func() {
		if panicObj := recover(); panicObj != nil {
			stack := debug.Stack()

			// Convert panic object to error
			var err error
			switch v := panicObj.(type) {
			case error:
				err = v
			case string:
				err = fmt.Errorf("panic: %s", v)
			default:
				err = fmt.Errorf("panic: %v", v)
			}

			// Log with structured information
			logger.Error(ctx, "Recovered from panic",
				"err", err.Error(),
				"errType", fmt.Sprintf("%T", panicObj),
				"stackTrace", stack,
				"fullStack", string(stack))
		}
	}()

	// Execute the function
	fn()
}

type httpError struct {
	Code    int
	Message string
}

// Error implements error interface.
func (e *httpError) Error() string { return e.Message }

// encodeError returns error to the HTTP client.
func encodeError(w http.ResponseWriter, err error) {
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Error(), httpErr.Code)
	} else {
		http.Error(w, httpErr.Error(), http.StatusInternalServerError)
	}
}
