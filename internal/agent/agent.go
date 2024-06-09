package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-dev/dagu/internal/persistence"

	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/mailer"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/reporter"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/google/uuid"
)

var (
	errFailedStartSocketFrontend = errors.New("failed to start the socket frontend")
	errDAGAlreadyRunning         = errors.New("the DAG is already running")
)

// Agent is the interface to run / cancel / signal / status / etc.
type Agent struct {
	*Config

	// TODO: Do not use the persistence package directly.
	dataStoreFactory persistence.DataStoreFactory
	engine           engine.Engine
	scheduler        *scheduler.Scheduler
	graph            *scheduler.ExecutionGraph
	logManager       *logManager
	reporter         *reporter.Reporter
	historyStore     persistence.HistoryStore
	socketServer     *sock.Server
	requestId        string
	finished         atomic.Bool
	lock             sync.RWMutex
}

func New(config *Config, e engine.Engine, ds persistence.DataStoreFactory) *Agent {
	return &Agent{
		Config:           config,
		engine:           e,
		dataStoreFactory: ds,
	}
}

// Config contains the configuration for an Agent.
type Config struct {
	DAG     *dag.DAG
	DAGsDir string
	Dry     bool

	// RetryTarget is the status to retry.
	RetryTarget *model.Status
}

// Run starts the dags execution.
func (a *Agent) Run(ctx context.Context) error {
	if err := func() error {
		a.lock.Lock()
		defer a.lock.Unlock()
		if err := a.setupRequestId(); err != nil {
			return err
		}
		a.init()
		return a.setupGraph()
	}(); err != nil {
		return err
	}
	if err := a.checkPreconditions(); err != nil {
		return err
	}
	if a.Dry {
		return a.dryRun()
	}
	for _, fn := range []func() error{
		a.checkIsRunning,
		a.setupDatabase,
		a.setupSocketServer,
		a.logManager.setupLogFile,
	} {
		if err := fn(); err != nil {
			return err
		}
	}
	return a.run(ctx)
}

// Status returns the current status of the DAG.
func (a *Agent) Status() *model.Status {
	a.lock.RLock()
	defer a.lock.RUnlock()

	scStatus := a.scheduler.Status(a.graph)
	// TODO: fix this weird logic.
	if scStatus == scheduler.StatusNone && a.graph.IsStarted() {
		scStatus = scheduler.StatusRunning
	}
	var ns []model.NodeStepPair
	for _, n := range a.graph.Nodes() {
		ns = append(ns, model.NodeStepPair{
			Node: n.State(),
			Step: n.Step(),
		})
	}
	st, et := model.Time(a.graph.StartAt()), model.Time(a.graph.FinishAt())
	status := model.NewStatus(a.DAG, ns, scStatus, os.Getpid(), st, et)
	status.RequestId = a.requestId
	status.Log = a.logManager.logFilename
	if node := a.scheduler.HandlerNode(constants.OnExit); node != nil {
		status.OnExit = model.FromNode(node.State(), node.Step())
	}
	if node := a.scheduler.HandlerNode(constants.OnSuccess); node != nil {
		status.OnSuccess = model.FromNode(node.State(), node.Step())
	}
	if node := a.scheduler.HandlerNode(constants.OnFailure); node != nil {
		status.OnFailure = model.FromNode(node.State(), node.Step())
	}
	if node := a.scheduler.HandlerNode(constants.OnCancel); node != nil {
		status.OnCancel = model.FromNode(node.State(), node.Step())
	}
	return status
}

// Signal sends the signal to the processes running
// if processes do not terminate after MaxCleanUp time, it will send KILL signal.
func (a *Agent) Signal(sig os.Signal) {
	a.signal(sig, false)
}

// Kill sends KILL signal to all child processes.
func (a *Agent) Kill() {
	log.Printf("Sending KILL signal to running child processes.")
	a.scheduler.Signal(a.graph, syscall.SIGKILL, nil, false)
}

func (a *Agent) signal(sig os.Signal, allowOverride bool) {
	log.Printf("Sending %s signal to running child processes.", sig)
	done := make(chan bool)
	go func() {
		a.scheduler.Signal(a.graph, sig, done, allowOverride)
	}()
	timeout := time.NewTimer(a.DAG.MaxCleanUpTime)
	tick := time.NewTimer(time.Second * 5)
	defer timeout.Stop()
	defer tick.Stop()

	for {
		select {
		case <-done:
			log.Printf("All child processes have been terminated.")
			return
		case <-timeout.C:
			log.Printf("Time reached to max cleanup time")
			a.Kill()
			return
		case <-tick.C:
			log.Printf("Sending signal again")
			a.scheduler.Signal(a.graph, sig, nil, false)
			tick.Reset(time.Second * 5)
		default:
			log.Printf("Waiting for child processes to exit...")
			time.Sleep(time.Second * 3)
		}
	}
}

func (a *Agent) init() {
	logDir := path.Join(a.DAG.LogDir, util.ValidFilename(a.DAG.Name, "_"))
	config := &scheduler.Config{
		LogDir:        logDir,
		MaxActiveRuns: a.DAG.MaxActiveRuns,
		Delay:         a.DAG.Delay,
		Dry:           a.Dry,
		RequestId:     a.requestId,
	}

	if a.DAG.HandlerOn.Exit != nil {
		config.OnExit = a.DAG.HandlerOn.Exit
	}

	if a.DAG.HandlerOn.Success != nil {
		config.OnSuccess = a.DAG.HandlerOn.Success
	}

	if a.DAG.HandlerOn.Failure != nil {
		config.OnFailure = a.DAG.HandlerOn.Failure
	}

	if a.DAG.HandlerOn.Cancel != nil {
		config.OnCancel = a.DAG.HandlerOn.Cancel
	}
	a.scheduler = &scheduler.Scheduler{Config: config}
	a.reporter = &reporter.Reporter{
		Config: &reporter.Config{
			Mailer: &mailer.Mailer{
				Config: &mailer.Config{
					Host:     a.DAG.Smtp.Host,
					Port:     a.DAG.Smtp.Port,
					Username: a.DAG.Smtp.Username,
					Password: a.DAG.Smtp.Password,
				},
			},
		}}
	logFilename := filepath.Join(
		logDir, fmt.Sprintf("agent_%s.%s.%s.log",
			util.ValidFilename(a.DAG.Name, "_"),
			time.Now().Format("20060102.15:04:05.000"),
			util.TruncString(a.requestId, 8),
		))
	a.logManager = &logManager{logFilename: logFilename}
}

func (a *Agent) setupGraph() (err error) {
	if a.RetryTarget != nil {
		log.Printf("setup for retry")
		return a.setupRetry()
	}
	a.graph, err = scheduler.NewExecutionGraph(a.DAG.Steps...)
	return
}

func (a *Agent) setupRetry() (err error) {
	nodes := make([]*scheduler.Node, 0, len(a.RetryTarget.Nodes))
	for _, n := range a.RetryTarget.Nodes {
		nodes = append(nodes, n.ToNode())
	}
	a.graph, err = scheduler.NewExecutionGraphForRetry(nodes...)
	return
}

func (a *Agent) setupRequestId() error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	a.requestId = id.String()
	return nil
}

func (a *Agent) setupDatabase() error {
	// TODO: do not use the persistence package directly.
	a.historyStore = a.dataStoreFactory.NewHistoryStore()
	if err := a.historyStore.RemoveOld(a.DAG.Location, a.DAG.HistRetentionDays); err != nil {
		util.LogErr("clean old history data", err)
	}

	return a.historyStore.Open(a.DAG.Location, time.Now(), a.requestId)
}

func (a *Agent) setupSocketServer() (err error) {
	a.socketServer, err = sock.NewServer(
		&sock.Config{
			Addr:        a.DAG.SockAddr(),
			HandlerFunc: a.HandleHTTP,
		})
	return
}

func (a *Agent) checkPreconditions() error {
	if len(a.DAG.Preconditions) > 0 {
		log.Printf("checking preconditions for \"%s\"", a.DAG.Name)
		if err := dag.EvalConditions(a.DAG.Preconditions); err != nil {
			a.scheduler.Cancel(a.graph)
			return err
		}
	}
	return nil
}

func (a *Agent) run(ctx context.Context) error {
	tl := &logger.Tee{Writer: a.logManager.logFile}
	if err := tl.Open(); err != nil {
		return err
	}
	defer func() {
		util.LogErr("close log file", a.closeLogFile())
		tl.Close()
	}()

	defer func() {
		if err := a.historyStore.Close(); err != nil {
			log.Printf("failed to close history store: %v", err)
		}
	}()

	util.LogErr("write status", a.historyStore.Write(a.Status()))

	listen := make(chan error)
	go func() {
		err := a.socketServer.Serve(listen)
		if err != nil && !errors.Is(err, sock.ErrServerRequestedShutdown) {
			log.Printf("failed to start socket frontend %v", err)
		}
	}()

	defer func() {
		util.LogErr("shutdown socket frontend", a.socketServer.Shutdown())
	}()

	if err := <-listen; err != nil {
		return errFailedStartSocketFrontend
	}

	done := make(chan *scheduler.Node)
	defer close(done)

	go func() {
		for node := range done {
			status := a.Status()
			util.LogErr("write status", a.historyStore.Write(status))
			util.LogErr("report step", a.reporter.ReportStep(a.DAG, status, node))
		}
	}()

	go func() {
		time.Sleep(time.Millisecond * 100)
		if a.finished.Load() {
			return
		}
		util.LogErr("write status", a.historyStore.Write(a.Status()))
	}()

	ctx = dag.NewContext(ctx, a.DAG, a.dataStoreFactory.NewDAGStore())

	lastErr := a.scheduler.Schedule(ctx, a.graph, done)
	status := a.Status()

	log.Println("schedule finished.")
	util.LogErr("write status", a.historyStore.Write(a.Status()))

	a.reporter.ReportSummary(status, lastErr)
	util.LogErr("send email", a.reporter.SendMail(a.DAG, status, lastErr))

	a.finished.Store(true)
	util.LogErr("close data file", a.historyStore.Close())

	return lastErr
}

func (a *Agent) dryRun() error {
	done := make(chan *scheduler.Node)
	defer func() {
		close(done)
	}()

	go func() {
		for node := range done {
			status := a.Status()
			_ = a.reporter.ReportStep(a.DAG, status, node)
		}
	}()

	log.Printf("***** Starting DRY-RUN *****")

	ctx := dag.NewContext(context.Background(), a.DAG, a.dataStoreFactory.NewDAGStore())

	lastErr := a.scheduler.Schedule(ctx, a.graph, done)
	status := a.Status()
	a.reporter.ReportSummary(status, lastErr)

	log.Printf("***** Finished DRY-RUN *****")

	return lastErr
}

func (a *Agent) checkIsRunning() error {
	status, err := a.engine.GetCurrentStatus(a.DAG)
	if err != nil {
		return err
	}
	if status.Status != scheduler.StatusNone {
		return fmt.Errorf("%w. socket=%s", errDAGAlreadyRunning, a.DAG.SockAddr())
	}
	return nil
}

func (a *Agent) closeLogFile() error {
	if a.logManager.logFile != nil {
		return a.logManager.logFile.Close()
	}
	return nil
}

var (
	statusRe = regexp.MustCompile(`^/status[/]?$`)
	stopRe   = regexp.MustCompile(`^/stop[/]?$`)
)

func (a *Agent) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && statusRe.MatchString(r.URL.Path):
		status := a.Status()
		status.Status = scheduler.StatusRunning
		b, err := status.ToJson()
		if err != nil {
			encodeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		go func() {
			log.Printf("stop request received. shutting down...")
			a.signal(syscall.SIGTERM, true)
		}()
	default:
		encodeError(w, &HTTPError{Code: http.StatusNotFound, Message: "Not found"})
	}
}

type logManager struct {
	logFilename string
	logFile     *os.File
}

func (l *logManager) setupLogFile() (err error) {
	dir := path.Dir(l.logFilename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	l.logFile, err = util.OpenOrCreateFile(l.logFilename)
	return
}

type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func encodeError(w http.ResponseWriter, err error) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Error(), httpErr.Code)
	} else {
		http.Error(w, httpErr.Error(), http.StatusInternalServerError)
	}
}
