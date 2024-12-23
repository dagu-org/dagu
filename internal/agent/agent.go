// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/mailer"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/sock"
)

// Agent is responsible for running the DAG and handling communication
// via the unix socket. The agent performs the following tasks:
// 1. Start the DAG execution.
// 2. Propagate a signal to the running processes.
// 3. Handle the HTTP request via the unix socket.
// 4. Write the log and status to the data store.
type Agent struct {
	dag          *digraph.DAG
	dry          bool
	retryTarget  *model.Status
	dataStore    persistence.DataStores
	client       client.Client
	scheduler    *scheduler.Scheduler
	graph        *scheduler.ExecutionGraph
	reporter     *reporter
	historyStore persistence.HistoryStore
	socketServer *sock.Server
	logDir       string
	logFile      string

	// requestID is request ID to identify DAG execution uniquely.
	// The request ID can be used for history lookup, retry, etc.
	requestID string
	finished  atomic.Bool

	lock    sync.RWMutex
	lastErr error
}

// Options is the configuration for the Agent.
type Options struct {
	// Dry is a dry-run mode. It does not execute the actual command.
	// Dry run does not create history data.
	Dry bool
	// RetryTarget is the target status (history of execution) to retry.
	// If it's specified the agent will execute the DAG with the same
	// configuration as the specified history.
	RetryTarget *model.Status
}

// New creates a new Agent.
func New(
	requestID string,
	dag *digraph.DAG,
	logDir string,
	logFile string,
	cli client.Client,
	dataStore persistence.DataStores,
	opts *Options,
) *Agent {
	return &Agent{
		requestID:   requestID,
		dag:         dag,
		dry:         opts.Dry,
		retryTarget: opts.RetryTarget,
		logDir:      logDir,
		logFile:     logFile,
		client:      cli,
		dataStore:   dataStore,
	}
}

var (
	// wait before read the running status
	waitForRunning = time.Millisecond * 100

	// errors on running DAG
	errFailedSetupUnixSocket = errors.New("failed to start the unix socket")
	errDAGIsAlreadyRunning   = errors.New("the DAG is already running")
)

// Run setups the scheduler and runs the DAG.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.setup(ctx); err != nil {
		return err
	}

	// It should not run the DAG if the condition is unmet.
	if err := a.checkPreconditions(ctx); err != nil {
		return err
	}

	// Handle dry execution.
	if a.dry {
		return a.dryRun(ctx)
	}

	// Check if the DAG is already running.
	if err := a.checkIsAlreadyRunning(ctx); err != nil {
		return err
	}

	// Make a connection to the database.
	// It should close the connection to the history database when the DAG
	// execution is finished.
	if err := a.setupDatabase(ctx); err != nil {
		return err
	}
	defer func() {
		if err := a.historyStore.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close history store", "error", err)
		}
	}()

	if err := a.historyStore.Write(ctx, a.Status()); err != nil {
		logger.Error(ctx, "Failed to write status", "error", err)
	}

	// Start the unix socket server for receiving HTTP requests from
	// the local client (e.g., the frontend server, scheduler, etc).
	if err := a.setupSocketServer(ctx); err != nil {
		return fmt.Errorf("failed to setup unix socket server: %w", err)
	}
	listenerErrCh := make(chan error)
	go func() {
		err := a.socketServer.Serve(ctx, listenerErrCh)
		if err != nil && !errors.Is(err, sock.ErrServerRequestedShutdown) {
			logger.Error(ctx, "Failed to start socket frontend", "error", err)
		}
	}()

	// Stop the socket server when finishing the DAG execution.
	defer func() {
		if err := a.socketServer.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Failed to shutdown socket frontend", "error", err)
		}
	}()

	// It returns error if it failed to start the unix socket server.
	if err := <-listenerErrCh; err != nil {
		return errFailedSetupUnixSocket
	}

	// Setup channels to receive status updates for each node in the DAG.
	// It should receive node instance when the node status changes, for
	// example, when started, stopped, or cancelled, etc.
	done := make(chan *scheduler.Node)
	defer close(done)
	go func() {
		for node := range done {
			status := a.Status()
			if err := a.historyStore.Write(ctx, status); err != nil {
				logger.Error(ctx, "Failed to write status", "error", err)
			}
			if err := a.reporter.reportStep(ctx, a.dag, status, node); err != nil {
				logger.Error(ctx, "Failed to report step", "error", err)
			}
		}
	}()

	// Write the first status just after the start to store the running status.
	// If the DAG is already finished, skip it.
	go func() {
		time.Sleep(waitForRunning)
		if a.finished.Load() {
			return
		}
		if err := a.historyStore.Write(ctx, a.Status()); err != nil {
			logger.Error(ctx, "Status write failed", "error", err)
		}
	}()

	// Start the DAG execution.
	logger.Info(ctx, "DAG execution started", "reqId", a.requestID, "name", a.dag.Name, "params", a.dag.Params)
	dagCtx := digraph.NewContext(ctx, a.dag, a.dataStore.DAGStore(), newOutputCollector(a.historyStore), a.requestID, a.logFile)
	lastErr := a.scheduler.Schedule(dagCtx, a.graph, done)

	// Update the finished status to the history database.
	finishedStatus := a.Status()
	logger.Info(ctx, "DAG execution finished", "status", finishedStatus.Status)
	if err := a.historyStore.Write(ctx, a.Status()); err != nil {
		logger.Error(ctx, "Status write failed", "error", err)
	}

	// Send the execution report if necessary.
	a.lastErr = lastErr
	if err := a.reporter.send(a.dag, finishedStatus, lastErr); err != nil {
		logger.Error(ctx, "Mail notification failed", "error", err)
	}

	// Mark the agent finished.
	a.finished.Store(true)

	// Return the last error on the DAG execution.
	return lastErr
}

func (a *Agent) PrintSummary(ctx context.Context) {
	status := a.Status()
	summary := a.reporter.getSummary(ctx, status, a.lastErr)
	println(summary)
}

// Status collects the current running status of the DAG and returns it.
func (a *Agent) Status() *model.Status {
	// Lock to avoid race condition.
	a.lock.RLock()
	defer a.lock.RUnlock()

	schedulerStatus := a.scheduler.Status(a.graph)
	if schedulerStatus == scheduler.StatusNone && a.graph.IsStarted() {
		// Match the status to the execution graph.
		schedulerStatus = scheduler.StatusRunning
	}

	// Create the status object to record the current status.
	status := &model.Status{
		RequestID:  a.requestID,
		Name:       a.dag.Name,
		Status:     schedulerStatus,
		StatusText: schedulerStatus.String(),
		PID:        model.PID(os.Getpid()),
		Nodes:      model.FromNodesOrSteps(a.graph.NodeData(), a.dag.Steps),
		StartedAt:  model.FormatTime(a.graph.StartAt()),
		FinishedAt: model.FormatTime(a.graph.FinishAt()),
		Log:        a.logFile,
		Params:     model.Params(a.dag.Params),
	}

	// Collect the handler nodes.
	if node := a.scheduler.HandlerNode(digraph.HandlerOnExit); node != nil {
		status.OnExit = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(digraph.HandlerOnSuccess); node != nil {
		status.OnSuccess = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(digraph.HandlerOnFailure); node != nil {
		status.OnFailure = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(digraph.HandlerOnCancel); node != nil {
		status.OnCancel = model.FromNode(node.Data())
	}

	return status
}

// Signal sends the signal to the processes running
func (a *Agent) Signal(ctx context.Context, sig os.Signal) {
	a.signal(ctx, sig, false)
}

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
			statusJSON, err := status.ToJSON()
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(statusJSON)
		case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
			// Handle Stop request for the DAG execution.
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

// setup the agent instance for DAG execution.
func (a *Agent) setup(ctx context.Context) error {
	// Lock to prevent race condition.
	a.lock.Lock()
	defer a.lock.Unlock()

	a.scheduler = a.newScheduler()
	a.reporter = newReporter(
		mailer.New(&mailer.NewMailerArgs{
			Host:     a.dag.SMTP.Host,
			Port:     a.dag.SMTP.Port,
			Username: a.dag.SMTP.Username,
			Password: a.dag.SMTP.Password,
		}),
	)

	return a.setupGraph(ctx)
}

// newScheduler creates a scheduler instance for the DAG execution.
func (a *Agent) newScheduler() *scheduler.Scheduler {
	cfg := &scheduler.Config{
		LogDir:        a.logDir,
		MaxActiveRuns: a.dag.MaxActiveRuns,
		Timeout:       a.dag.Timeout,
		Delay:         a.dag.Delay,
		Dry:           a.dry,
		ReqID:         a.requestID,
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

	logger.Info(ctx, "Dry-run started", "reqId", a.requestID)

	dagCtx := digraph.NewContext(context.Background(), a.dag, a.dataStore.DAGStore(), newOutputCollector(a.historyStore), a.requestID, a.logFile)
	lastErr := a.scheduler.Schedule(dagCtx, a.graph, done)
	a.lastErr = lastErr

	logger.Info(ctx, "Dry-run finished", "reqId", a.requestID)

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
	logger.Info(ctx, "Sending signal to running child processes", "signal", sig)
	done := make(chan bool)
	go func() {
		a.scheduler.Signal(a.graph, sig, done, allowOverride)
	}()
	timeout := time.NewTimer(a.dag.MaxCleanUpTime)
	tick := time.NewTimer(time.Second * 5)
	defer timeout.Stop()
	defer tick.Stop()

	for {
		select {
		case <-done:
			logger.Info(ctx, "All child processes have been terminated")
			return
		case <-timeout.C:
			logger.Info(ctx, "Time reached to max cleanup time")
			logger.Info(ctx, "Sending KILL signal to running child processes.")
			a.scheduler.Signal(a.graph, syscall.SIGKILL, nil, false)
			return
		case <-tick.C:
			logger.Info(ctx, "Sending signal again")
			a.scheduler.Signal(a.graph, sig, nil, false)
			tick.Reset(time.Second * 5)
		default:
			logger.Info(ctx, "Waiting for child processes to exit...")
			time.Sleep(time.Second * 3)
		}
	}
}

// setupGraph setups the DAG graph. If is retry execution, it loads nodes
// from the retry node so that it runs the same DAG as the previous run.
func (a *Agent) setupGraph(ctx context.Context) error {
	if a.retryTarget != nil {
		logger.Info(ctx, "Retry execution", "reqId", a.requestID)
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

// setup database prepare database connection and remove old history data.
func (a *Agent) setupDatabase(ctx context.Context) error {
	a.historyStore = a.dataStore.HistoryStore()
	location, retentionDays := a.dag.Location, a.dag.HistRetentionDays
	if err := a.historyStore.RemoveOld(ctx, location, retentionDays); err != nil {
		logger.Error(ctx, "History data cleanup failed", "error", err)
	}

	return a.historyStore.Open(ctx, a.dag.Location, time.Now(), a.requestID)
}

// setupSocketServer create socket server instance.
func (a *Agent) setupSocketServer(ctx context.Context) error {
	socketServer, err := sock.NewServer(a.dag.SockAddr(), a.HandleHTTP(ctx))
	if err != nil {
		return err
	}
	a.socketServer = socketServer
	return nil
}

// checkPrecondition check if the preconditions are met. If not, it returns
// error.
func (a *Agent) checkPreconditions(ctx context.Context) error {
	if len(a.dag.Preconditions) == 0 {
		return nil
	}
	// If one of the conditions does not met, cancel the execution.
	if err := digraph.EvalConditions(a.dag.Preconditions); err != nil {
		logger.Error(ctx, "Preconditions are not met", "error", err)
		a.scheduler.Cancel(a.graph)
		return err
	}
	return nil
}

// checkIsAlreadyRunning returns error if the DAG is already running.
func (a *Agent) checkIsAlreadyRunning(ctx context.Context) error {
	status, err := a.client.GetCurrentStatus(ctx, a.dag)
	if err != nil {
		return err
	}
	if status.Status != scheduler.StatusNone {
		return fmt.Errorf(
			"%w. socket=%s", errDAGIsAlreadyRunning, a.dag.SockAddr(),
		)
	}
	return nil
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

var _ digraph.ResultCollector = &resultCollector{}

type resultCollector struct {
	historyStore persistence.HistoryStore
}

func newOutputCollector(store persistence.HistoryStore) *resultCollector {
	return &resultCollector{historyStore: store}
}

func (o *resultCollector) CollectResult(ctx context.Context, name string, requestID string) (*digraph.Result, error) {
	status, err := o.historyStore.FindByRequestID(ctx, name, requestID)
	if err != nil {
		return nil, err
	}

	outputVariables := map[string]string{}
	for _, node := range status.Status.Nodes {
		if node.Step.OutputVariables != nil {
			node.Step.OutputVariables.Range(func(_, value any) bool {
				// split the value by '=' to get the key and value
				parts := strings.Split(value.(string), "=")
				if len(parts) == 2 {
					outputVariables[parts[0]] = parts[1]
				}
				return true
			})
		}
	}

	return &digraph.Result{
		Outputs: outputVariables,
		Name:    status.Status.Name,
		Params:  status.Status.Params,
	}, nil
}
