package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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

	// dagRepo is the database to store the DAG definitions.
	dagRepo models.DAGRepository

	// historyRepo is the database to store the run history.
	historyRepo models.HistoryRepository

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

	// rootRun indicates the DAG name and request ID of the root run history.
	// It is same as the current DAG name and request ID if it's not a sub-DAG.
	rootRun digraph.RootRun

	// reqID is request ID to identify DAG run uniquely.
	// The request ID can be used for runstore lookup, retry, etc.
	reqID string

	// parentReqID is the request ID of the parent DAG run.
	parentReqID string

	// finished is true if the DAG run is finished.
	finished atomic.Bool

	// lastErr is the last error occurred during the DAG run.
	lastErr error

	// subExecution is true if the agent is running as a sub-DAG.
	subExecution atomic.Bool
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
	// ParentID is the request ID of the parent DAG run.
	// It is required for sub-DAG runs to identify the parent DAG.
	ParentID string
}

// New creates a new Agent.
func New(
	reqID string,
	dag *digraph.DAG,
	logDir string,
	logFile string,
	cli history.Manager,
	dagRepo models.DAGRepository,
	historyRepo models.HistoryRepository,
	rootRun digraph.RootRun,
	opts Options,
) *Agent {
	return &Agent{
		rootRun:     rootRun,
		reqID:       reqID,
		dag:         dag,
		dry:         opts.Dry,
		retryTarget: opts.RetryTarget,
		logDir:      logDir,
		logFile:     logFile,
		client:      cli,
		dagRepo:     dagRepo,
		historyRepo: historyRepo,
		parentReqID: opts.ParentID,
	}
}

// Run setups the scheduler and runs the DAG.
func (a *Agent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := a.setup(ctx); err != nil {
		return fmt.Errorf("agent setup failed: %w", err)
	}

	// Create a new context for the DAG run with all necessary information
	dbClient := newDBClient(a.historyRepo, a.dagRepo)
	ctx = digraph.SetupEnv(ctx, a.dag, dbClient, a.rootRun, a.reqID, a.logFile, a.dag.Params)

	// Add structured logging context
	logFields := []any{"dag", a.dag.Name, "reqId", a.reqID}
	if a.subExecution.Load() {
		logFields = append(logFields, "rootDAG", a.rootRun.Name, "rootReqID", a.rootRun.ReqID)
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

	// Make a connection to the database.
	// It should close the connection to the runstore database when the DAG
	// execution is finished.
	historyRecord, err := a.setupRunRecord(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup run record: %w", err)
	}
	if err := historyRecord.Open(ctx); err != nil {
		return fmt.Errorf("failed to open run record: %w", err)
	}
	defer func() {
		if err := historyRecord.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close runstore store", "err", err)
		}
	}()

	if err := historyRecord.Write(ctx, a.Status()); err != nil {
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

	// Stop the socket server when finishing the DAG run.
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
	done := make(chan *scheduler.Node)
	defer close(done)
	go execWithRecovery(ctx, func() {
		for node := range done {
			status := a.Status()
			if err := historyRecord.Write(ctx, status); err != nil {
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
		if err := historyRecord.Write(ctx, a.Status()); err != nil {
			logger.Error(ctx, "Status write failed", "err", err)
		}
	})

	// Start the DAG run.
	logger.Debug(ctx, "DAG run started", "reqId", a.reqID, "name", a.dag.Name, "params", a.dag.Params)
	lastErr := a.scheduler.Schedule(ctx, a.graph, done)

	// Update the finished status to the runstore database.
	finishedStatus := a.Status()
	logger.Info(ctx, "DAG run finished", "status", finishedStatus.Status.String())
	if err := historyRecord.Write(ctx, a.Status()); err != nil {
		logger.Error(ctx, "Status write failed", "err", err)
	}

	// Send the execution report if necessary.
	a.lastErr = lastErr
	if err := a.reporter.send(ctx, a.dag, finishedStatus, lastErr); err != nil {
		logger.Error(ctx, "Mail notification failed", "err", err)
	}

	// Mark the agent finished.
	a.finished.Store(true)

	// Return the last error on the DAG run.
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
	}

	if a.subExecution.Load() {
		opts = append(opts, models.WithSubRunMetadata(a.rootRun, a.parentReqID))
	}

	// Create the status object to record the current status.
	return models.NewStatusBuilder(a.dag).
		Create(
			a.reqID,
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
			// Handle Stop request for the DAG run.
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

// setup the agent instance for DAG run.
func (a *Agent) setup(ctx context.Context) error {
	// Lock to prevent race condition.
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.rootRun.ReqID != a.reqID {
		logger.Debug(ctx, "Initiating sub-DAG run", "rootDAG", a.rootRun.Name, "rootReqID", a.rootRun.ReqID)
		a.subExecution.Store(true)
		if a.parentReqID == "" {
			logger.Error(ctx, "Parent request ID is required for sub-DAG run")
			return fmt.Errorf("parent request ID is required for sub-DAG run")
		}
	}

	a.scheduler = a.newScheduler()
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

	return a.setupGraph(ctx)
}

// newScheduler creates a scheduler instance for the DAG run.
func (a *Agent) newScheduler() *scheduler.Scheduler {
	cfg := &scheduler.Config{
		LogDir:        a.logDir,
		MaxActiveRuns: a.dag.MaxActiveRuns,
		Timeout:       a.dag.Timeout,
		Delay:         a.dag.Delay,
		Dry:           a.dry,
		ReqID:         a.reqID,
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
	// done channel receives the node when the node is done.
	// It's a way to update the status in real-time in efficient manner.
	done := make(chan *scheduler.Node)
	defer func() {
		close(done)
	}()

	go func() {
		for node := range done {
			status := a.Status()
			_ = a.reporter.reportStep(ctx, a.dag, status, node)
		}
	}()

	logger.Info(ctx, "Dry-run started", "reqId", a.reqID, "name", a.dag.Name, "params", a.dag.Params)

	db := newDBClient(a.historyRepo, a.dagRepo)
	dagCtx := digraph.SetupEnv(ctx, a.dag, db, a.rootRun, a.reqID, a.logFile, a.dag.Params)
	lastErr := a.scheduler.Schedule(dagCtx, a.graph, done)
	a.lastErr = lastErr

	logger.Info(ctx, "Dry-run finished", "reqId", a.reqID)

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
		logger.Info(ctx, "Retry run", "reqId", a.reqID)
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
	graph, err := scheduler.CreateRetryExecutionGraph(ctx, nodes...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

func (a *Agent) setupRunRecord(ctx context.Context) (models.Record, error) {
	retentionDays := a.dag.HistRetentionDays
	if err := a.historyRepo.RemoveOld(ctx, a.dag.Name, retentionDays); err != nil {
		logger.Error(ctx, "History data cleanup failed", "err", err)
	}

	opts := models.NewRecordOptions{Retry: a.retryTarget != nil}
	if a.subExecution.Load() {
		opts.Root = &a.rootRun
	}

	return a.historyRepo.Create(ctx, a.dag, time.Now(), a.reqID, opts)
}

// setupSocketServer create socket server instance.
func (a *Agent) setupSocketServer(ctx context.Context) error {
	var socketAddr string
	if a.subExecution.Load() {
		// Use separate socket address for sub-DAGs to allow them run concurrently.
		socketAddr = a.dag.SockAddrSub(a.reqID)
	} else {
		socketAddr = a.dag.SockAddr(a.reqID)
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
	if a.subExecution.Load() {
		return nil // Skip the check for sub-DAGs
	}
	if a.client.IsRunning(ctx, a.dag, a.reqID) {
		return fmt.Errorf("the DAG is already running. requestID=%s, socket=%s", a.reqID, a.dag.SockAddr(a.reqID))
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
	dagRepo     models.DAGRepository
	historyRepo models.HistoryRepository
}

func newDBClient(h models.HistoryRepository, d models.DAGRepository) *dbClient {
	return &dbClient{
		historyRepo: h,
		dagRepo:     d,
	}
}

// GetDAG implements digraph.DBClient.
func (o *dbClient) GetDAG(ctx context.Context, name string) (*digraph.DAG, error) {
	return o.dagRepo.GetDetails(ctx, name)
}

func (o *dbClient) GetSubStatus(ctx context.Context, reqID string, rootDAG digraph.RootRun) (*digraph.Status, error) {
	runRecord, err := o.historyRepo.FindSubRun(ctx, rootDAG.Name, rootDAG.ReqID, reqID)
	if err != nil {
		return nil, err
	}
	status, err := runRecord.ReadStatus(ctx)
	if err != nil {
		return nil, err
	}

	outputVariables := map[string]string{}
	for _, node := range status.Nodes {
		if node.Step.OutputVariables != nil {
			node.Step.OutputVariables.Range(func(_, value any) bool {
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
		Outputs: outputVariables,
		Name:    status.Name,
		Params:  status.Params,
	}, nil
}
