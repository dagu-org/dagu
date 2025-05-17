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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/mailer"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
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
	retryTarget *models.Status

	// dagStore is the database to store the DAG definitions.
	dagStore models.DAGStore

	// historyStore is the database to store the run history.
	historyStore models.HistoryStore

	// client is the runstore client to communicate with the history.
	client history.Manager

	// scheduler is the scheduler instance to run the DAG.
	scheduler *scheduler.Scheduler

	// graph is the execution graph for the DAG.
	graph *scheduler.ExecutionGraph

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
	dag *digraph.DAG

	// root indicates the DAG name and execution reference of the root execution history.
	// It is same as the current execution reference if it's not a child workflow.
	root digraph.WorkflowRef

	// parent is the execution reference of the parent workflow.
	parent digraph.WorkflowRef

	// workflowID is the workflow ID of the current workflow.
	workflowID string

	// runID is the ID for the current run of the workflow.
	runID string

	// finished is true if the workflow is finished.
	finished atomic.Bool

	// lastErr is the last error occurred during the workflow.
	lastErr error

	// childWorkflow is true if the agent is running as a child workflow.
	childWorkflow atomic.Bool
}

// Options is the configuration for the Agent.
type Options struct {
	// Dry is a dry-run mode. It does not execute the actual command.
	// Dry run does not create runstore data.
	Dry bool
	// RetryTarget is the target status (runstore of execution) to retry.
	// If it's specified the agent will execute the DAG with the same
	// configuration as the specified history.
	RetryTarget *models.Status
	// Parent is the workflow ID of the parent workflow.
	// It is required for child workflows to identify the parent DAG.
	Parent digraph.WorkflowRef
}

// New creates a new Agent.
func New(
	workflowID string,
	dag *digraph.DAG,
	logDir string,
	logFile string,
	cli history.Manager,
	ds models.DAGStore,
	hs models.HistoryStore,
	root digraph.WorkflowRef,
	opts Options,
) *Agent {
	return &Agent{
		root:         root,
		parent:       opts.Parent,
		workflowID:   workflowID,
		dag:          dag,
		dry:          opts.Dry,
		retryTarget:  opts.RetryTarget,
		logDir:       logDir,
		logFile:      logFile,
		client:       cli,
		dagStore:     ds,
		historyStore: hs,
	}
}

// Run setups the scheduler and runs the DAG.
func (a *Agent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if a.root.WorkflowID != a.workflowID {
		logger.Debug(ctx, "Initiating child workflow", "rootRef", a.root.String())
		a.childWorkflow.Store(true)
		if a.parent.Zero() {
			logger.Error(ctx, "Parent workflow ID is required for child workflow")
			return fmt.Errorf("parent workflow ID is required for child workflow")
		}
	}

	var run models.Run

	if !a.dry {
		// Setup the run record for the workflow.
		// It's not required for dry-run because it does not create data.
		r, err := a.setupRun(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup execution history: %w", err)
		}
		run = r
		a.runID = run.ID()
	}

	// Initialize the scheduler
	a.scheduler = a.newScheduler()

	// Setup the reporter to send the report to the user.
	a.setupReporter(ctx)

	// Setup the graph for the workflow.
	if err := a.setupGraph(ctx); err != nil {
		return fmt.Errorf("failed to setup execution graph: %w", err)
	}

	// Create a new environment for the workflow execution.
	dbClient := newDBClient(a.historyStore, a.dagStore)
	ctx = digraph.SetupEnv(ctx, a.dag, dbClient, a.root, a.workflowID, a.logFile, a.dag.Params)

	// Add structured logging context
	logFields := []any{"dag", a.dag.Name, "workflowId", a.workflowID}
	if a.childWorkflow.Load() {
		logFields = append(logFields, "root", a.root.String(), "parent", a.parent.String())
	}
	ctx = logger.WithValues(ctx, logFields...)

	// Handle dry execution.
	if a.dry {
		return a.dryRun(ctx)
	}

	// Check if the DAG is already running.
	if err := a.checkIsAlreadyRunning(ctx); err != nil {
		a.scheduler.Cancel(ctx, a.graph)
		return err
	}

	if err := run.Open(ctx); err != nil {
		return fmt.Errorf("failed to open execution history: %w", err)
	}
	defer func() {
		if err := run.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close runstore store", "err", err)
		}
	}()

	if err := run.Write(ctx, a.Status()); err != nil {
		logger.Error(ctx, "Failed to write status", "err", err)
	}

	// Start the unix socket server for receiving HTTP requests from
	// the local client (e.g., the frontend server, scheduler, etc).
	if err := a.setupSocketServer(ctx); err != nil {
		return fmt.Errorf("failed to setup unix socket server: %w", err)
	}

	listenerErrCh := make(chan error)
	go execWithRecovery(ctx, func() {
		err := a.socketServer.Serve(ctx, listenerErrCh)
		if err != nil && !errors.Is(err, sock.ErrServerRequestedShutdown) {
			logger.Error(ctx, "Failed to start socket frontend", "err", err)
		}
	})

	// Stop the socket server when finishing the workflow.
	defer func() {
		if err := a.socketServer.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Failed to shutdown socket frontend", "err", err)
		}
	}()

	// It returns error if it failed to start the unix socket server.
	if err := <-listenerErrCh; err != nil {
		return fmt.Errorf("failed to start the unix socket server: %w", err)
	}

	// Setup channels to receive status updates for each node in the DAG.
	// It should receive node instance when the node status changes, for
	// example, when started, stopped, or cancelled, etc.
	progressCh := make(chan *scheduler.Node)
	defer close(progressCh)
	go execWithRecovery(ctx, func() {
		for node := range progressCh {
			status := a.Status()
			if err := run.Write(ctx, status); err != nil {
				logger.Error(ctx, "Failed to write status", "err", err)
			}
			if err := a.reporter.reportStep(ctx, a.dag, status, node); err != nil {
				logger.Error(ctx, "Failed to report step", "err", err)
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
		if err := run.Write(ctx, a.Status()); err != nil {
			logger.Error(ctx, "Status write failed", "err", err)
		}
	})

	// Start the workflow.
	logger.Debug(ctx, "Workflow started", "workflowId", a.workflowID, "name", a.dag.Name, "params", a.dag.Params)
	lastErr := a.scheduler.Schedule(ctx, a.graph, progressCh)

	// Update the finished status to the runstore database.
	finishedStatus := a.Status()

	// Log execution summary
	logger.Info(ctx, "Workflow finished",
		"name", a.dag.Name,
		"workflowId", a.workflowID,
		"runID", a.runID,
		"status", finishedStatus.Status.String(),
		"startedAt", finishedStatus.StartedAt,
		"finishedAt", finishedStatus.FinishedAt,
	)

	if err := run.Write(ctx, a.Status()); err != nil {
		logger.Error(ctx, "Status write failed", "err", err)
	}

	// Send the execution report if necessary.
	a.lastErr = lastErr
	if err := a.reporter.send(ctx, a.dag, finishedStatus, lastErr); err != nil {
		logger.Error(ctx, "Mail notification failed", "err", err)
	}

	// Mark the agent finished.
	a.finished.Store(true)

	// Return the last error on the workflow.
	return lastErr
}

func (a *Agent) PrintSummary(ctx context.Context) {
	status := a.Status()
	summary := a.reporter.getSummary(ctx, status, a.lastErr)
	println(summary)
}

// Status collects the current running status of the DAG and returns it.
func (a *Agent) Status() models.Status {
	// Lock to avoid race condition.
	a.lock.RLock()
	defer a.lock.RUnlock()

	schedulerStatus := a.scheduler.Status(a.graph)
	if schedulerStatus == scheduler.StatusNone && a.graph.IsStarted() {
		// Match the status to the execution graph.
		schedulerStatus = scheduler.StatusRunning
	}

	opts := []models.StatusOption{
		models.WithFinishedAt(a.graph.FinishAt()),
		models.WithNodes(a.graph.NodeData()),
		models.WithLogFilePath(a.logFile),
		models.WithOnExitNode(a.scheduler.HandlerNode(digraph.HandlerOnExit)),
		models.WithOnSuccessNode(a.scheduler.HandlerNode(digraph.HandlerOnSuccess)),
		models.WithOnFailureNode(a.scheduler.HandlerNode(digraph.HandlerOnFailure)),
		models.WithOnCancelNode(a.scheduler.HandlerNode(digraph.HandlerOnCancel)),
		models.WithRunID(a.runID),
		models.WithHierarchyRefs(a.root, a.parent),
		models.WithPreconditions(a.dag.Preconditions),
	}

	// Create the status object to record the current status.
	return models.NewStatusBuilder(a.dag).
		Create(
			a.workflowID,
			schedulerStatus,
			os.Getpid(),
			a.graph.StartAt(),
			opts...,
		)
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
			// Return the current status of the execution.
			status := a.Status()
			status.Status = scheduler.StatusRunning
			statusJSON, err := json.Marshal(status)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(statusJSON)
		case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
			// Handle Stop request for the workflow.
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

// newScheduler creates a scheduler instance for the workflow.
func (a *Agent) newScheduler() *scheduler.Scheduler {
	// schedulerLogDir is the directory to store the log files for each node in the workflow.
	const dateTimeFormatUTC = "20060102_150405Z"
	ts := time.Now().UTC().Format(dateTimeFormatUTC)
	schedulerLogDir := filepath.Join(a.logDir, "run_"+ts+"_"+a.runID)

	cfg := &scheduler.Config{
		LogDir:        schedulerLogDir,
		MaxActiveRuns: a.dag.MaxActiveRuns,
		Timeout:       a.dag.Timeout,
		Delay:         a.dag.Delay,
		Dry:           a.dry,
		WorkflowID:    a.workflowID,
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

	return scheduler.New(cfg)
}

// dryRun performs a dry-run of the DAG. It only simulates the execution of
// the DAG without running the actual command.
func (a *Agent) dryRun(ctx context.Context) error {
	// progressCh channel receives the node when the node is progressCh.
	// It's a way to update the status in real-time in efficient manner.
	progressCh := make(chan *scheduler.Node)
	defer func() {
		close(progressCh)
	}()

	go func() {
		for node := range progressCh {
			status := a.Status()
			_ = a.reporter.reportStep(ctx, a.dag, status, node)
		}
	}()

	db := newDBClient(a.historyStore, a.dagStore)
	dagCtx := digraph.SetupEnv(ctx, a.dag, db, a.root, a.workflowID, a.logFile, a.dag.Params)
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
	graph, err := scheduler.NewExecutionGraph(a.dag.Steps...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

// setupGraphForRetry setsup the graph for retry.
func (a *Agent) setupGraphForRetry(ctx context.Context) error {
	nodes := make([]*scheduler.Node, 0, len(a.retryTarget.Nodes))
	for _, n := range a.retryTarget.Nodes {
		nodes = append(nodes, n.ToNode())
	}
	graph, err := scheduler.CreateRetryExecutionGraph(ctx, a.dag, nodes...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

func (a *Agent) setupRun(ctx context.Context) (models.Run, error) {
	retentionDays := a.dag.HistRetentionDays
	if err := a.historyStore.RemoveOldWorkflows(ctx, a.dag.Name, retentionDays); err != nil {
		logger.Error(ctx, "History data cleanup failed", "err", err)
	}

	opts := models.NewRunOptions{Retry: a.retryTarget != nil}
	if a.childWorkflow.Load() {
		opts.Root = &a.root
	}

	return a.historyStore.CreateRun(ctx, a.dag, time.Now(), a.workflowID, opts)
}

// setupSocketServer create socket server instance.
func (a *Agent) setupSocketServer(ctx context.Context) error {
	var socketAddr string
	if a.childWorkflow.Load() {
		// Use separate socket address for child workflows to allow them run concurrently.
		socketAddr = a.dag.SockAddrSub(a.workflowID)
	} else {
		socketAddr = a.dag.SockAddr(a.workflowID)
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
	if a.childWorkflow.Load() {
		return nil // Skip the check for child workflows
	}
	if a.client.IsDAGRunning(ctx, a.dag, a.workflowID) {
		return fmt.Errorf("the DAG is already running. workflowID=%s, socket=%s", a.workflowID, a.dag.SockAddr(a.workflowID))
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
				"error", err.Error(),
				"errorType", fmt.Sprintf("%T", panicObj),
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

var _ digraph.DB = &dbClient{}

type dbClient struct {
	ds models.DAGStore
	hs models.HistoryStore
}

func newDBClient(hs models.HistoryStore, ds models.DAGStore) *dbClient {
	return &dbClient{
		hs: hs,
		ds: ds,
	}
}

// GetDAG implements digraph.DBClient.
func (o *dbClient) GetDAG(ctx context.Context, name string) (*digraph.DAG, error) {
	return o.ds.GetDetails(ctx, name)
}

func (o *dbClient) GetChildWorkflowStatus(ctx context.Context, workflowID string, root digraph.WorkflowRef) (*digraph.Status, error) {
	run, err := o.hs.FindChildWorkflowRun(ctx, root, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find run for workflow ID %s: %w", workflowID, err)
	}
	status, err := run.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	outputVariables := map[string]string{}
	for _, node := range status.Nodes {
		if node.OutputVariables != nil {
			node.OutputVariables.Range(func(_, value any) bool {
				// split the value by '=' to get the key and value
				parts := strings.SplitN(value.(string), "=", 2)
				if len(parts) == 2 {
					outputVariables[parts[0]] = parts[1]
				}
				return true
			})
		}
	}

	return &digraph.Status{
		Outputs:    outputVariables,
		Name:       status.Name,
		WorkflowID: status.WorkflowID,
		Params:     status.Params,
	}, nil
}
