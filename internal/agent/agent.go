// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
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
	dag              *dag.DAG
	dry              bool
	fromWaitingQueue bool //to check if the dag is coming from the waiting queue
	retry            bool // to check if the job dag is being retried
	retryTarget      *model.Status
	dataStore        persistence.DataStores
	queueStore       persistence.QueueStore
	statsStore       persistence.StatsStore
	client           client.Client
	scheduler        *scheduler.Scheduler
	graph            *scheduler.ExecutionGraph
	reporter         *reporter
	historyStore     persistence.HistoryStore
	socketServer     *sock.Server
	logDir           string
	logFile          string
	dagQueueLength   int
	logger           logger.Logger

	// requestID is request ID to identify DAG execution uniquely.
	// The request ID can be used for history lookup, retry, etc.
	requestID string
	finished  atomic.Bool

	lock sync.RWMutex
}

// Options is the configuration for the Agent.
type Options struct {
	// Dry is a dry-run mode. It does not execute the actual command.
	// Dry run does not create history data.
	Dry bool
	// retry if the task it retried
	Retry bool
	// fromQueue if the task is started from dequeue
	FromWaitingQueue bool
	// RetryTarget is the target status (history of execution) to retry.
	// If it's specified the agent will execute the DAG with the same
	// configuration as the specified history.
	RetryTarget *model.Status
}

// New creates a new Agent.
func New(
	requestID string,
	workflow *dag.DAG,
	lg logger.Logger,
	logDir, logFile string,
	dagQueueLength int,
	cli client.Client,
	dataStore persistence.DataStores,
	queueStore persistence.QueueStore,
	statsStore persistence.StatsStore,
	opts *Options,
) *Agent {
	return &Agent{
		requestID:        requestID,
		dag:              workflow,
		dry:              opts.Dry,
		retry:            opts.Retry,
		fromWaitingQueue: opts.FromWaitingQueue,
		retryTarget:      opts.RetryTarget,
		logDir:           logDir,
		logFile:          logFile,
		dagQueueLength:   dagQueueLength,
		logger:           lg,
		client:           cli,
		dataStore:        dataStore,
		queueStore:       queueStore,
		statsStore:       statsStore,
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
	if err := a.setup(); err != nil {
		return err
	}

	// It should not run the DAG if the condition is unmet.
	if err := a.checkPreconditions(); err != nil {
		return err
	}

	// Handle dry execution.
	if a.dry {
		return a.dryRun()
	}

	// Check if the DAG is already running.
	if err := a.checkIsAlreadyRunning(); err != nil {
		return err
	}

	// Make a connection to the database.
	// It should close the connection to the history database when the DAG
	// execution is finished.
	if err := a.setupDatabase(); err != nil {
		return err
	}
	// Make a connection to queue dir
	if err := a.setupQueue(); err != nil {
		return err
	}
	// Make a connection to stats dir
	if err := a.setupStats(); err != nil {
		return err
	}

	// queue implementation
	noOfRunningDAGS, _ := a.statsStore.GetRunningDags()
	// this will check if the dag is being retried and if the queue.json is empty
	queueLength := a.queueStore.QueueLength()

	if noOfRunningDAGS < a.dagQueueLength && queueLength == 0 || a.retry || a.fromWaitingQueue {
		// Start the unix socket server for receiving HTTP requests from
		// the local client (e.g., the frontend server, scheduler, etc).
		if err := a.setupSocketServer(); err != nil {
			return err
		}
		lnErr := make(chan error)
		go func() {
			err := a.socketServer.Serve(lnErr)
			if err != nil && !errors.Is(err, sock.ErrServerRequestedShutdown) {
				a.logger.Error("Failed to start socket frontend", "error", err)
			}
		}()

		// Stop the socket server when finishing the DAG execution.
		defer func() {
			if err := a.socketServer.Shutdown(); err != nil {
				a.logger.Error("Failed to shutdown socket frontend", "error", err)
			}
		}()

		// It returns error if it failed to start the unix socket server.
		if err := <-lnErr; err != nil {
			return errFailedSetupUnixSocket
		}

		// Setup channels to receive status updates for each node in the DAG.
		// It should receive node instance when the node status changes, for
		// example, when started, stopped, or cancelled, etc.
		done := make(chan *scheduler.Node)
		defer close(done)
		// this writes status to historyStore at each node runs.
		go func() {
			for node := range done {
				status := a.Status()
				if err := a.historyStore.Write(status); err != nil {
					a.logger.Error("Failed to write status", "error", err)
				}
				if err := a.reporter.reportStep(a.dag, status, node); err != nil {
					a.logger.Error("Failed to report step", "error", err)
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
			if err := a.historyStore.Write(a.Status()); err != nil {
				a.logger.Error("Status write failed", "error", err)
			}
		}()

		// increment the number of running DAGs
		a.statsStore.IncrementRunningDags()

		// decrement running
		defer a.statsStore.DecrementRunningDags()
		dagCtx := dag.NewContext(context.Background(), a.dag, a.dataStore.DAGStore(), a.requestID, a.logFile)

		// Start the DAG execution.
		lastErr := a.scheduler.Schedule(
			dagCtx,
			a.graph,
			done,
		)

		// Update the finished status to the history database.
		finishedStatus := a.Status()
		a.logger.Info("Workflow execution finished", "status", finishedStatus.Status)
		if err := a.historyStore.Write(a.Status()); err != nil {
			a.logger.Error("Status write failed", "error", err)
		}

		// Send the execution report if necessary.
		a.reporter.report(finishedStatus, lastErr)
		if err := a.reporter.send(a.dag, finishedStatus, lastErr); err != nil {
			a.logger.Error("Mail notification failed", "error", err)
		}

		// Mark the agent finished.
		a.finished.Store(true)
		// Return the last error on the DAG execution.
		return lastErr

	} else {
		if err := a.queueStore.Enqueue(a.dag); err != nil {
			a.logger.Error("error queuing dag.", a.dag.Name)
		}
		if err := a.setStatus(a.dag); err != nil {
			log.Println("dag queued.")
		}

		a.logger.Infof("dag queued \"%s\".", a.dag.Name)

		return nil
	}
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
	if node := a.scheduler.HandlerNode(dag.HandlerOnExit); node != nil {
		status.OnExit = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(dag.HandlerOnSuccess); node != nil {
		status.OnSuccess = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(dag.HandlerOnFailure); node != nil {
		status.OnFailure = model.FromNode(node.Data())
	}
	if node := a.scheduler.HandlerNode(dag.HandlerOnCancel); node != nil {
		status.OnCancel = model.FromNode(node.Data())
	}

	return status
}

// Signal sends the signal to the processes running
func (a *Agent) Signal(sig os.Signal) {
	a.signal(sig, false)
}

// Simple regular expressions for request routing
var (
	statusRe = regexp.MustCompile(`^/status[/]?$`)
	stopRe   = regexp.MustCompile(`^/stop[/]?$`)
)

// HandleHTTP handles HTTP requests via unix socket.
func (a *Agent) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && statusRe.MatchString(r.URL.Path):
		// Return the current status of the execution.
		status := a.Status()
		status.Status = scheduler.StatusRunning
		b, err := status.ToJSON()
		if err != nil {
			encodeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
		// Handle Stop request for the DAG execution.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		go func() {
			a.logger.Info("Stop request received")
			a.signal(syscall.SIGTERM, true)
		}()
	default:
		// Unknown request
		encodeError(
			w, &httpError{Code: http.StatusNotFound, Message: "Not found"},
		)
	}
}

// setup the agent instance for DAG execution.
func (a *Agent) setup() error {
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
		a.logger,
	)

	return a.setupGraph()
}

// newScheduler creates a scheduler instance for the DAG execution.
func (a *Agent) newScheduler() *scheduler.Scheduler {
	cfg := &scheduler.Config{
		LogDir:        a.logDir,
		Logger:        a.logger,
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
func (a *Agent) dryRun() error {
	// done channel receives the node when the node is done.
	// It's a way to update the status in real-time in efficient manner.
	done := make(chan *scheduler.Node)
	defer func() {
		close(done)
	}()

	go func() {
		for node := range done {
			status := a.Status()
			_ = a.reporter.reportStep(a.dag, status, node)
		}
	}()

	a.logger.Info("Dry-run started", "reqId", a.requestID)

	dagCtx := dag.NewContext(context.Background(), a.dag, a.dataStore.DAGStore(), a.requestID, a.logFile)
	lastErr := a.scheduler.Schedule(dagCtx, a.graph, done)

	a.reporter.report(a.Status(), lastErr)

	a.logger.Info("Dry-run finished", "reqId", a.requestID)

	return lastErr
}

// signal propagates the received signal to the all running child processes.
// allowOverride parameters is used to specify if a node can override
// the signal to send to the process, in case the node is configured
// to send a custom signal (e.g., SIGSTOP instead of SIGTERM).
// The reason we need this is to allow the system to kill the child
// process by sending a SIGKILL to force the process to be shutdown.
// if processes do not terminate after MaxCleanUp time, it sends KILL signal.
func (a *Agent) signal(sig os.Signal, allowOverride bool) {
	a.logger.Info("Sending signal to running child processes", "signal", sig)
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
			a.logger.Info("All child processes have been terminated")
			return
		case <-timeout.C:
			a.logger.Info("Time reached to max cleanup time")
			a.logger.Info("Sending KILL signal to running child processes.")
			a.scheduler.Signal(a.graph, syscall.SIGKILL, nil, false)
			return
		case <-tick.C:
			a.logger.Info("Sending signal again")
			a.scheduler.Signal(a.graph, sig, nil, false)
			tick.Reset(time.Second * 5)
		default:
			a.logger.Info("Waiting for child processes to exit...")
			time.Sleep(time.Second * 3)
		}
	}
}

// setupGraph setups the DAG graph. If is retry execution, it loads nodes
// from the retry node so that it runs the same DAG as the previous run.
func (a *Agent) setupGraph() error {
	if a.retryTarget != nil {
		a.logger.Info("Retry execution", "reqId", a.requestID)
		return a.setupGraphForRetry()
	}
	graph, err := scheduler.NewExecutionGraph(a.logger, a.dag.Steps...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

// setupGraphForRetry setsup the graph for retry.
func (a *Agent) setupGraphForRetry() error {
	nodes := make([]*scheduler.Node, 0, len(a.retryTarget.Nodes))
	for _, n := range a.retryTarget.Nodes {
		nodes = append(nodes, n.ToNode())
	}
	graph, err := scheduler.NewExecutionGraphForRetry(a.logger, nodes...)
	if err != nil {
		return err
	}
	a.graph = graph
	return nil
}

// setup database prepare database connection and remove old history data.
func (a *Agent) setupDatabase() error {
	a.historyStore = a.dataStore.HistoryStore()
	location, retentionDays := a.dag.Location, a.dag.HistRetentionDays
	if err := a.historyStore.RemoveOld(location, retentionDays); err != nil {
		a.logger.Error("History data cleanup failed", "error", err)
	}
	if err := a.historyStore.RemoveEmptyQueue(a.dag.Location); err != nil {
		a.logger.Error("Queue History data clean up failed", "error", err)
	}

	return a.historyStore.Open(a.dag.Location, time.Now(), a.requestID)
}

func (a *Agent) setStatus(d *dag.DAG) error {
	status := model.NewStatusQueue(d)
	a.historyStore.Write(status)
	return nil
}

// TODO
func (a *Agent) setupQueue() error {
	// TODO: do not use the persistence package directly.
	a.queueStore = a.dataStore.QueueStore()
	err := a.queueStore.Create()
	if err != nil {
		return err
	}
	return nil
}

// TODO
func (a *Agent) setupStats() error {
	// TODO: do not use the persistence package directly.
	a.statsStore = a.dataStore.StatsStore()
	err := a.statsStore.Create()
	if err != nil {
		return err
	}
	return nil
}

// setupSocketServer create socket server instance.
func (a *Agent) setupSocketServer() error {
	socketServer, err := sock.NewServer(
		a.dag.SockAddr(),
		a.HandleHTTP,
		a.logger,
	)
	if err != nil {
		return err
	}
	a.socketServer = socketServer
	return nil
}

// checkPrecondition check if the preconditions are met. If not, it returns
// error.
func (a *Agent) checkPreconditions() error {
	if len(a.dag.Preconditions) == 0 {
		return nil
	}
	// If one of the conditions does not met, cancel the execution.
	if err := dag.EvalConditions(a.dag.Preconditions); err != nil {
		a.logger.Error("Preconditions are not met", "error", err)
		a.scheduler.Cancel(a.graph)
		return err
	}
	return nil
}

// checkIsAlreadyRunning returns error if the DAG is already running.
func (a *Agent) checkIsAlreadyRunning() error {
	status, err := a.client.GetCurrentStatus(a.dag)
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
