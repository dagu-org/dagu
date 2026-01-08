package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/mailer"
	"github.com/dagu-org/dagu/internal/common/masking"
	"github.com/dagu-org/dagu/internal/common/secrets"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/common/sock"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/output"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/builtin/docker"
	"github.com/dagu-org/dagu/internal/runtime/builtin/ssh"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/service/coordinator"

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
	dagRunMgr runtime.Manager

	// runner is the runner instance to run the DAG.
	runner *runtime.Runner

	// plan is the execution plan for the DAG.
	plan *runtime.Plan

	// reporter is responsible for sending the report to the user.
	reporter *reporter

	// socketServer is the unix socket server to handle HTTP requests.
	// It listens to the requests from the local client (e.g., frontend server).
	socketServer *sock.Server

	// logDir is the directory to store the log files for each node in the DAG.
	logDir string

	// logFile is the file to write the runner log.
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

	// initFailed is true if initialization failed before the runner could start.
	initFailed atomic.Bool

	// lastErr is the last error occurred during the dag-run.
	lastErr error

	// isSubDAGRun is true if the current dag-run is not the root dag-run,
	// meaning that it is a sub dag-run of another dag-run.
	isSubDAGRun atomic.Bool

	// progressDisplay is the progress display for showing real-time execution progress.
	progressDisplay ProgressReporter

	// stepRetry is the name of the step to retry, if specified.
	stepRetry string

	// workerID is the identifier of the worker executing this DAG run.
	workerID string

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
	// WorkerID is the identifier of the worker executing this DAG run.
	// For distributed execution, this is set to the worker's ID.
	// For local execution, this defaults to "local".
	WorkerID string
}

// New creates a new Agent.
func New(
	dagRunID string,
	dag *core.DAG,
	logDir string,
	logFile string,
	drm runtime.Manager,
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
		workerID:     opts.WorkerID,
	}

	// Initialize progress display if enabled
	if opts.ProgressDisplay {
		a.progressDisplay = createProgressReporter(dag, dagRunID, dag.Params)
	}

	return a
}

// Run setups the runner and runs the DAG.
func (a *Agent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set DAG context for all logs in this function
	ctx = logger.WithValues(ctx,
		tag.Name(a.dag.Name),
		tag.RunID(a.dagRunID),
		tag.AttemptID(a.dagRunAttemptID),
	)

	// Initialize propagators for W3C trace context before anything else
	telemetry.InitializePropagators()

	// Resolve secrets early so they're available for OTel config evaluation
	a.dag.LoadDotEnv(ctx)
	secretEnvs, secretErr := a.resolveSecrets(ctx)

	// Build variables map for config evaluation (DAG env + secrets)
	configVars := make(map[string]string)
	for _, env := range a.dag.Env {
		if key, value, found := strings.Cut(env, "="); found {
			configVars[key] = value
		}
	}
	for _, env := range secretEnvs {
		if key, value, found := strings.Cut(env, "="); found {
			configVars[key] = value
		}
	}

	// Extract trace context from environment variables if present
	// This must be done BEFORE initializing the tracer so sub DAGs
	// can continue the parent's trace
	if a.dag.OTel != nil && a.dag.OTel.Enabled {
		ctx = telemetry.ExtractTraceContext(ctx)
	}

	// Initialize OpenTelemetry tracer
	tracer, err := telemetry.NewTracer(ctx, a.dag, configVars)
	if err != nil {
		logger.Warn(ctx, "Failed to initialize OpenTelemetry tracer", tag.Error(err))
		// Continue without tracing
	} else {
		a.tracer = tracer
		defer func() {
			if err := tracer.Shutdown(ctx); err != nil {
				logger.Warn(ctx, "Failed to shutdown OpenTelemetry tracer", tag.Error(err))
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
		logger.Debug(ctx, "Initiating a sub dag-run",
			slog.String("root-run", a.rootDAGRun.String()),
			slog.String("parent-run", a.parentDAGRun.String()),
		)

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

	// Initialize the runner
	a.runner = a.newRunner(attempt)

	// Setup the execution plan for the DAG.
	if err := a.setupPlan(ctx); err != nil {
		return fmt.Errorf("failed to setup execution plan: %w", err)
	}

	// Create a new environment for the dag-run.
	dbClient := newDBClient(a.dagRunStore, a.dagStore)

	// Initialize coordinator client factory for distributed execution
	coordinatorCli := a.createCoordinatorClient(ctx)

	ctx = runtime.NewContext(ctx, a.dag, a.dagRunID, a.logFile,
		runtime.WithDatabase(dbClient),
		runtime.WithRootDAGRun(a.rootDAGRun),
		runtime.WithParams(a.dag.Params),
		runtime.WithCoordinator(coordinatorCli),
		runtime.WithSecrets(secretEnvs),
	)

	// Add structured logging context
	logFields := []slog.Attr{
		tag.DAG(a.dag.Name),
		tag.RunID(a.dagRunID),
	}
	if a.isSubDAGRun.Load() {
		logFields = append(logFields,
			slog.String("root", a.rootDAGRun.String()),
			slog.String("parent", a.parentDAGRun.String()),
		)
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

	// Evaluate SMTP and mail configs with environment variables and secrets.
	// This must happen AFTER attempt.Open() to avoid persisting expanded secrets.
	if err := a.evaluateMailConfigs(ctx); err != nil {
		return err
	}

	// Setup the reporter to send notifications (must be after mail config evaluation)
	a.setupReporter(ctx)

	// Update the status to running
	st := a.Status(ctx)
	st.Status = core.Running
	if err := attempt.Write(ctx, st); err != nil {
		logger.Error(ctx, "Status write failed", tag.Error(err))
	}

	defer func() {
		if initErr != nil {
			a.initFailed.Store(true)
			logger.Error(ctx, "Failed to initialize DAG execution", tag.Error(initErr))
			st := a.Status(ctx)
			st.Status = core.Failed
			if err := attempt.Write(ctx, st); err != nil {
				logger.Error(ctx, "Status write failed", tag.Error(err))
			}
		}
		if err := attempt.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close runstore store", tag.Error(err))
		}
	}()

	// If there was an error resolving secrets, stop execution here
	if secretErr != nil {
		initErr = secretErr // Stop execution if secret resolution failed
		return initErr
	}

	if err := attempt.Write(ctx, a.Status(ctx)); err != nil {
		logger.Error(ctx, "Failed to write status", tag.Error(err))
	}

	// Start the unix socket server for receiving HTTP requests from
	// the local client (e.g., the frontend server, etc).
	if err := a.setupSocketServer(ctx); err != nil {
		initErr = fmt.Errorf("failed to setup unix socket server: %w", err)
		return initErr
	}

	// Ensure working directory exists
	if err := os.MkdirAll(a.dag.WorkingDir, 0o755); err != nil {
		initErr = fmt.Errorf("failed to create working directory: %w", err)
		return initErr
	}

	// Move to the working directory
	if err := os.Chdir(a.dag.WorkingDir); err != nil {
		initErr = fmt.Errorf("failed to change working directory: %w", err)
		return initErr
	}

	// Create a new container if the DAG has a container configuration.
	if a.dag.Container != nil {
		// Expand environment variables in container fields
		expandedContainer, err := docker.EvalContainerFields(ctx, *a.dag.Container)
		if err != nil {
			initErr = fmt.Errorf("failed to evaluate container config: %w", err)
			return initErr
		}
		ctCfg, err := docker.LoadConfig(a.dag.WorkingDir, expandedContainer, a.dag.RegistryAuths)
		if err != nil {
			initErr = fmt.Errorf("failed to load container config: %w", err)
			return initErr
		}
		ctCli, err := docker.InitializeClient(ctx, ctCfg)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize container client: %w", err)
			return initErr
		}
		// In exec mode, we use an existing container - don't create a new one
		isExecMode := expandedContainer.IsExecMode()
		if !isExecMode {
			if err := ctCli.CreateContainerKeepAlive(ctx); err != nil {
				initErr = fmt.Errorf("failed to create keepalive container: %w", err)
				return initErr
			}
		}

		// Set the container client in the context for the execution.
		ctx = docker.WithContainerClient(ctx, ctCli)

		defer func() {
			// Only stop the container if we created it (non-exec mode)
			if !isExecMode {
				ctCli.StopContainerKeepAlive(ctx)
			}
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
			Shell:         a.dag.SSH.Shell,
			ShellArgs:     a.dag.SSH.ShellArgs,
		}, runtime.AllEnvsMap(ctx))
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
			logger.Error(ctx, "Failed to start socket frontend", tag.Error(err))
		}
	})

	// Stop the socket server when the dag-run is finished.
	defer func() {
		if err := a.socketServer.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Failed to shutdown socket frontend", tag.Error(err))
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
				logger.Error(ctx, "Failed to write status", tag.Error(err))
			}
			if err := a.reporter.reportStep(ctx, a.dag, status, node); err != nil {
				logger.Error(ctx, "Failed to report step", tag.Error(err))
			}
			// Update progress display if enabled
			if a.progressDisplay != nil {
				// Convert runner node to models node
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
			logger.Error(ctx, "Status write failed", tag.Error(err))
		}
	})

	// Start the dag-run.
	if a.retryTarget != nil {
		logger.Info(ctx, "DAG run retry started",
			slog.String("retry-target-attempt-id", a.retryTarget.AttemptID),
		)
	} else {
		logger.Info(ctx, "DAG run started", slog.Any("params", a.dag.Params))
	}

	// Start watching for cancel requests
	go execWithRecovery(ctx, func() {
		a.watchCancelRequested(ctx, attempt)
	})

	// Add registry authentication to context for docker executors
	if len(a.dag.RegistryAuths) > 0 {
		ctx = docker.WithRegistryAuth(ctx, a.dag.RegistryAuths)
	}

	lastErr := a.runner.Run(ctx, a.plan, progressCh)

	if coordinatorCli != nil {
		// Cleanup the coordinator client resources if it was created.
		if err := coordinatorCli.Cleanup(ctx); err != nil {
			logger.Warn(ctx, "Failed to cleanup coordinator client", tag.Error(err))
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
	logger.Info(ctx, "DAG run finished",
		tag.Status(finishedStatus.Status.String()),
		slog.String("started-at", finishedStatus.StartedAt),
		slog.String("finished-at", finishedStatus.FinishedAt),
	)

	// Collect and write step outputs BEFORE finalizing status (per spec)
	if dagOutputs := a.buildOutputs(ctx, finishedStatus.Status); dagOutputs != nil {
		if err := attempt.WriteOutputs(ctx, dagOutputs); err != nil {
			logger.Error(ctx, "Failed to write outputs", tag.Error(err))
		}
	}

	// Finalize status (after outputs are written)
	if err := attempt.Write(ctx, a.Status(ctx)); err != nil {
		logger.Error(ctx, "Status write failed", tag.Error(err))
	}

	// Send the execution report if necessary.
	a.lastErr = lastErr
	if err := a.reporter.send(ctx, a.dag, finishedStatus, lastErr); err != nil {
		logger.Error(ctx, "Mail notification failed", tag.Error(err))
	}

	// Mark the agent finished.
	a.finished.Store(true)

	// Return the last error on the dag-run.
	return lastErr
}

// nodeToModelNode converts a runner NodeData to a models.Node
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

// collectOutputs gathers all step outputs into a map for the outputs.json file.
// It iterates through nodes in execution order and collects output values.
// Steps with OutputOmit=true are skipped. Last value wins for key conflicts.
func (a *Agent) collectOutputs(ctx context.Context) map[string]string {
	outputs := make(map[string]string)

	// Get nodes from the plan in execution order
	nodes := a.plan.Nodes()

	for _, node := range nodes {
		nodeData := node.NodeData()
		step := nodeData.Step

		// Skip if no output defined or omit is set
		if step.Output == "" || step.OutputOmit {
			continue
		}

		// Skip if no output variables captured
		if nodeData.State.OutputVariables == nil {
			continue
		}

		// Get the output value from captured variables
		rawValue, ok := nodeData.State.OutputVariables.Load(step.Output)
		if !ok {
			continue
		}

		// Parse the KeyValue format (KEY=VALUE)
		strValue, ok := rawValue.(string)
		if !ok {
			logger.Warn(ctx, "Output variable is not a string, skipping",
				slog.String("step", step.Name),
				slog.String("output", step.Output),
				slog.String("type", fmt.Sprintf("%T", rawValue)),
			)
			continue
		}
		kv := stringutil.KeyValue(strValue)
		value := kv.Value()

		// Determine the key: use OutputKey if set, otherwise convert from UPPER_CASE
		key := step.OutputKey
		if key == "" {
			key = stringutil.ScreamingSnakeToCamel(step.Output)
		}

		// Store the output (last one wins for conflicts)
		outputs[key] = value
	}

	// Warn if total size exceeds 1MB
	if len(outputs) > 0 {
		totalSize := 0
		for k, v := range outputs {
			totalSize += len(k) + len(v)
		}
		if totalSize > 1024*1024 {
			logger.Warn(ctx, "Outputs size exceeds 1MB",
				slog.String("dag", a.dag.Name),
				slog.String("dagRunId", a.dagRunID),
				slog.Int("size", totalSize),
				slog.Int("count", len(outputs)),
			)
		}
	}

	return outputs
}

// buildOutputs creates the full DAGRunOutputs structure with metadata.
// Returns nil if no outputs were collected.
func (a *Agent) buildOutputs(ctx context.Context, finalStatus core.Status) *execution.DAGRunOutputs {
	outputs := a.collectOutputs(ctx)

	if len(outputs) == 0 {
		return nil
	}

	// Mask any secrets in output values to prevent exposing sensitive data
	rCtx := runtime.GetDAGContext(ctx)
	if len(rCtx.SecretEnvs) > 0 {
		// Convert secret envs map to the format expected by masker
		var secretEnvs []string
		for k, v := range rCtx.SecretEnvs {
			secretEnvs = append(secretEnvs, k+"="+v)
		}
		masker := masking.NewMasker(masking.SourcedEnvVars{
			Secrets: secretEnvs,
		})

		// Mask each output value
		for key, value := range outputs {
			outputs[key] = masker.MaskString(value)
		}
	}

	// Serialize params to JSON
	var paramsJSON string
	if len(a.dag.Params) > 0 {
		if data, err := json.Marshal(a.dag.Params); err == nil {
			paramsJSON = string(data)
		}
	}

	return &execution.DAGRunOutputs{
		Metadata: execution.OutputsMetadata{
			DAGName:     a.dag.Name,
			DAGRunID:    a.dagRunID,
			AttemptID:   a.dagRunAttemptID,
			Status:      finalStatus.String(),
			CompletedAt: stringutil.FormatTime(time.Now()),
			Params:      paramsJSON,
		},
		Outputs: outputs,
	}
}

func (a *Agent) PrintSummary(ctx context.Context) {
	// Always print tree-structured summary after execution
	status := a.Status(ctx)

	// Create a minimal DAG object for the tree renderer
	dag := &core.DAG{Name: status.Name}

	// Enable colors if stdout is a terminal
	config := output.DefaultConfig()
	config.ColorEnabled = term.IsTerminal(int(os.Stdout.Fd()))

	renderer := output.NewRenderer(config)
	summary := renderer.RenderDAGStatus(dag, &status)

	// Write to stdout and sync to ensure output is flushed before program exit
	_, _ = os.Stdout.WriteString(summary)
	_, _ = os.Stdout.WriteString("\n")
	_ = os.Stdout.Sync()
}

// Status collects the current running status of the DAG and returns it.
func (a *Agent) Status(ctx context.Context) execution.DAGRunStatus {
	// Lock to avoid race condition.
	a.lock.RLock()
	defer a.lock.RUnlock()

	runnerStatus := a.runner.Status(ctx, a.plan)
	if a.initFailed.Load() {
		runnerStatus = core.Failed
	} else if runnerStatus == core.NotStarted && a.plan.IsStarted() {
		// Match the status to the execution plan.
		runnerStatus = core.Running
	}

	opts := []transform.StatusOption{
		transform.WithFinishedAt(a.plan.FinishAt()),
		transform.WithNodes(a.plan.NodeData()),
		transform.WithLogFilePath(a.logFile),
		transform.WithOnInitNode(a.runner.HandlerNode(core.HandlerOnInit)),
		transform.WithOnExitNode(a.runner.HandlerNode(core.HandlerOnExit)),
		transform.WithOnSuccessNode(a.runner.HandlerNode(core.HandlerOnSuccess)),
		transform.WithOnFailureNode(a.runner.HandlerNode(core.HandlerOnFailure)),
		transform.WithOnCancelNode(a.runner.HandlerNode(core.HandlerOnCancel)),
		transform.WithOnWaitNode(a.runner.HandlerNode(core.HandlerOnWait)),
		transform.WithAttemptID(a.dagRunAttemptID),
		transform.WithHierarchyRefs(a.rootDAGRun, a.parentDAGRun),
		transform.WithPreconditions(a.dag.Preconditions),
		transform.WithWorkerID(a.workerID),
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
			runnerStatus,
			os.Getpid(),
			a.plan.StartAt(),
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
			if cancelled, _ := attempt.IsAborting(ctx); cancelled {
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
			logger.Debug(ctx, "Mail notification is disabled",
				slog.String("subject", subject),
			)
			return nil
		}
	}

	a.reporter = newReporter(senderFn)
}

// newRunner creates a runner instance for the dag-run.
func (a *Agent) newRunner(attempt execution.DAGRunAttempt) *runtime.Runner {
	// runnerLogDir is the directory to store the log files for each node in the dag-run.
	const dateTimeFormatUTC = "20060102_150405Z"
	ts := time.Now().UTC().Format(dateTimeFormatUTC)
	runnerLogDir := filepath.Join(a.logDir, "run_"+ts+"_"+a.dagRunAttemptID)

	cfg := &runtime.Config{
		LogDir:          runnerLogDir,
		MaxActiveSteps:  a.dag.MaxActiveSteps,
		Timeout:         a.dag.Timeout,
		Delay:           a.dag.Delay,
		Dry:             a.dry,
		DAGRunID:        a.dagRunID,
		MessagesHandler: attempt, // Attempt implements ChatMessagesHandler
	}

	if a.dag.HandlerOn.Init != nil {
		cfg.OnInit = a.dag.HandlerOn.Init
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

	if a.dag.HandlerOn.Wait != nil {
		cfg.OnWait = a.dag.HandlerOn.Wait
	}

	return runtime.New(cfg)
}

// createCoordinatorClient creates a coordinator client factory for distributed execution
func (a *Agent) createCoordinatorClient(ctx context.Context) runtime.Dispatcher {
	if a.registry == nil {
		logger.Debug(ctx, "Service monitor is not configured; skipping coordinator client creation")
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

	logger.Info(ctx, "Resolving secrets",
		tag.Count(len(a.dag.Secrets)),
	)

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

	logger.Debug(ctx, "Secrets resolved successfully",
		tag.Count(len(resolvedSecrets)),
	)
	return resolvedSecrets, nil
}

// evaluateMailConfigs evaluates SMTP and mail notification configs with
// environment variables and secrets. This follows the same pattern used for
// container and SSH configs.
func (a *Agent) evaluateMailConfigs(ctx context.Context) error {
	vars := runtime.AllEnvsMap(ctx)

	// Evaluate SMTP config if defined
	if a.dag.SMTP != nil {
		evaluated, err := cmdutil.EvalObject(ctx, *a.dag.SMTP, vars)
		if err != nil {
			return fmt.Errorf("failed to evaluate smtp config: %w", err)
		}
		a.dag.SMTP = &evaluated
	}

	// Evaluate error mail config if defined
	if a.dag.ErrorMail != nil {
		evaluated, err := cmdutil.EvalObject(ctx, *a.dag.ErrorMail, vars)
		if err != nil {
			return fmt.Errorf("failed to evaluate error mail config: %w", err)
		}
		a.dag.ErrorMail = &evaluated
	}

	// Evaluate info mail config if defined
	if a.dag.InfoMail != nil {
		evaluated, err := cmdutil.EvalObject(ctx, *a.dag.InfoMail, vars)
		if err != nil {
			return fmt.Errorf("failed to evaluate info mail config: %w", err)
		}
		a.dag.InfoMail = &evaluated
	}

	// Evaluate wait mail config if defined
	if a.dag.WaitMail != nil {
		evaluated, err := cmdutil.EvalObject(ctx, *a.dag.WaitMail, vars)
		if err != nil {
			return fmt.Errorf("failed to evaluate wait mail config: %w", err)
		}
		a.dag.WaitMail = &evaluated
	}

	return nil
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
	dagCtx := runtime.NewContext(ctx, a.dag, a.dagRunID, a.logFile,
		runtime.WithDatabase(db),
		runtime.WithRootDAGRun(a.rootDAGRun),
		runtime.WithParams(a.dag.Params),
	)
	lastErr := a.runner.Run(dagCtx, a.plan, progressCh)
	a.lastErr = lastErr

	logger.Info(ctx, "Dry-run completed",
		slog.Any("params", a.dag.Params),
	)

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
		tag.Signal(sig.String()),
		slog.Bool("allow-override", allowOverride),
		slog.Duration("max-cleanup-time", a.dag.MaxCleanUpTime),
	)

	if !signal.IsTerminationSignalOS(sig) {
		// For non-termination signals, just send the signal once and return.
		a.runner.Signal(ctx, a.plan, sig, nil, allowOverride)
		return
	}

	signalCtx, cancel := context.WithTimeout(ctx, a.dag.MaxCleanUpTime)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		a.runner.Signal(ctx, a.plan, sig, done, allowOverride)
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
			a.runner.Signal(ctx, a.plan, syscall.SIGKILL, nil, false)
			return

		case <-ticker.C:
			logger.Info(ctx, "Resending signal to processes that haven't terminated",
				tag.Signal(sig.String()),
			)
			a.runner.Signal(ctx, a.plan, sig, nil, false)

		case <-time.After(500 * time.Millisecond):
			// Quick check to avoid busy waiting, but still responsive
			if a.plan != nil && !a.plan.IsRunning() {
				logger.Info(ctx, "No running processes detected, termination complete")
				return
			}
		}
	}
}

// setupPlan setups the DAG plan. If is retry execution, it loads nodes
// from the retry node so that it runs the same DAG as the previous run.
func (a *Agent) setupPlan(ctx context.Context) error {
	if a.retryTarget != nil {
		return a.setupRetryPlan(ctx)
	}
	plan, err := runtime.NewPlan(a.dag.Steps...)
	if err != nil {
		return err
	}
	a.plan = plan
	return nil
}

// setupRetryPlan sets up the plan for retry.
func (a *Agent) setupRetryPlan(ctx context.Context) error {
	nodes := make([]*runtime.Node, 0, len(a.retryTarget.Nodes))
	for _, n := range a.retryTarget.Nodes {
		nodes = append(nodes, transform.ToNode(n))
	}
	if a.stepRetry != "" {
		return a.setupStepRetryPlan(ctx, nodes)
	}
	return a.setupDefaultRetryPlan(ctx, nodes)
}

// setupStepRetryPlan sets up the plan for retrying a specific step.
func (a *Agent) setupStepRetryPlan(ctx context.Context, nodes []*runtime.Node) error {
	plan, err := runtime.CreateStepRetryPlan(a.dag, nodes, a.stepRetry)
	if err != nil {
		return err
	}
	a.plan = plan
	return nil
}

// setupDefaultRetryPlan sets up the plan for the default retry behavior (all failed/canceled nodes and downstreams).
func (a *Agent) setupDefaultRetryPlan(ctx context.Context, nodes []*runtime.Node) error {
	plan, err := runtime.CreateRetryPlan(ctx, a.dag, nodes...)
	if err != nil {
		return err
	}
	a.plan = plan
	return nil
}

func (a *Agent) setupDAGRunAttempt(ctx context.Context) (execution.DAGRunAttempt, error) {
	retentionDays := a.dag.HistRetentionDays
	if _, err := a.dagRunStore.RemoveOldDAGRuns(ctx, a.dag.Name, retentionDays); err != nil {
		logger.Error(ctx, "DAG runs data cleanup failed", tag.Error(err))
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
				slog.String("err", err.Error()),
				slog.String("errType", fmt.Sprintf("%T", panicObj)),
				slog.String("stackTrace", string(stack)),
				slog.String("fullStack", string(stack)),
			)
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
