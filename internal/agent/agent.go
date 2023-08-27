package agent

import (
	"context"
	"errors"
	"fmt"
	"github.com/yohamta/dagu/internal/persistence"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/internal/mailer"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/pb"
	"github.com/yohamta/dagu/internal/reporter"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/sock"
	"github.com/yohamta/dagu/internal/utils"
)

// Agent is the interface to run / cancel / signal / status / etc.
type Agent struct {
	*AgentConfig
	*RetryConfig

	scheduler    *scheduler.Scheduler
	graph        *scheduler.ExecutionGraph
	logManager   *logManager
	reporter     *reporter.Reporter
	historyStore persistence.HistoryStore
	socketServer *sock.Server
	requestId    string
}

// AgentConfig contains the configuration for an Agent.
type AgentConfig struct {
	DAG *dag.DAG
	Dry bool
}

// RetryConfig contains the configuration for retrying a workflow.
type RetryConfig struct {
	Status *models.Status
}

// Run starts the workflow execution.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.setupRequestId(); err != nil {
		return err
	}
	a.init()
	if err := a.setupGraph(); err != nil {
		return err
	}
	if err := a.checkPreconditions(); err != nil {
		return err
	}
	if a.Dry {
		return a.dryRun()
	}
	setup := []func() error{
		a.checkIsRunning,
		a.setupDatabase,
		a.setupSocketServer,
		a.logManager.setupLogFile,
	}
	for _, fn := range setup {
		err := fn()
		if err != nil {
			return err
		}
	}
	return a.run(ctx)
}

// Status returns the current status of the workflow.
func (a *Agent) Status() *models.Status {
	scStatus := a.scheduler.Status(a.graph)
	if scStatus == scheduler.SchedulerStatus_None && !a.graph.StartedAt.IsZero() {
		scStatus = scheduler.SchedulerStatus_Running
	}

	status := models.NewStatus(
		a.DAG,
		a.graph.Nodes(),
		scStatus,
		os.Getpid(),
		&a.graph.StartedAt,
		&a.graph.FinishedAt,
	)
	status.RequestId = a.requestId
	status.Log = a.logManager.logFilename
	if node := a.scheduler.HandlerNode(constants.OnExit); node != nil {
		status.OnExit = models.FromNode(node)
	}
	if node := a.scheduler.HandlerNode(constants.OnSuccess); node != nil {
		status.OnSuccess = models.FromNode(node)
	}
	if node := a.scheduler.HandlerNode(constants.OnFailure); node != nil {
		status.OnFailure = models.FromNode(node)
	}
	if node := a.scheduler.HandlerNode(constants.OnCancel); node != nil {
		status.OnCancel = models.FromNode(node)
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
	timeout := time.After(a.DAG.MaxCleanUpTime)
	tick := time.After(time.Second * 5)
	for {
		select {
		case <-done:
			log.Printf("All child processes have been terminated.")
			return
		case <-timeout:
			log.Printf("Time reached to max cleanup time")
			a.Kill()
			return
		case <-tick:
			log.Printf("Sending signal again")
			a.scheduler.Signal(a.graph, sig, nil, false)
			tick = time.After(time.Second * 5)
		default:
			log.Printf("Waiting for child processes to exit...")
			time.Sleep(time.Second * 3)
		}
	}
}

func (a *Agent) init() {
	logDir := path.Join(a.DAG.LogDir, utils.ValidFilename(a.DAG.Name, "_"))
	config := &scheduler.Config{
		LogDir:        logDir,
		MaxActiveRuns: a.DAG.MaxActiveRuns,
		Delay:         a.DAG.Delay,
		Dry:           a.Dry,
		RequestId:     a.requestId,
	}

	if a.DAG.HandlerOn.Exit != nil {
		onExit, _ := pb.ToPbStep(a.DAG.HandlerOn.Exit)
		config.OnExit = onExit
	}

	if a.DAG.HandlerOn.Success != nil {
		onSuccess, _ := pb.ToPbStep(a.DAG.HandlerOn.Success)
		config.OnSuccess = onSuccess
	}

	if a.DAG.HandlerOn.Failure != nil {
		onFailure, _ := pb.ToPbStep(a.DAG.HandlerOn.Failure)
		config.OnFailure = onFailure
	}

	if a.DAG.HandlerOn.Cancel != nil {
		onCancel, _ := pb.ToPbStep(a.DAG.HandlerOn.Cancel)
		config.OnCancel = onCancel
	}

	a.scheduler = &scheduler.Scheduler{
		Config: config,
	}
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
			utils.ValidFilename(a.DAG.Name, "_"),
			time.Now().Format("20060102.15:04:05.000"),
			utils.TruncString(a.requestId, 8),
		))
	a.logManager = &logManager{logFilename: logFilename}
}

func (a *Agent) setupGraph() (err error) {
	if a.RetryConfig != nil && a.RetryConfig.Status != nil {
		log.Printf("setup for retry")
		return a.setupRetry()
	}
	a.graph, err = scheduler.NewExecutionGraph(a.DAG.Steps...)
	return
}

func (a *Agent) setupRetry() (err error) {
	nodes := []*scheduler.Node{}
	for _, n := range a.RetryConfig.Status.Nodes {
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
	a.historyStore = jsondb.New()
	if err := a.historyStore.RemoveOld(a.DAG.Location, a.DAG.HistRetentionDays); err != nil {
		utils.LogErr("clean old history data", err)
	}
	if err := a.historyStore.Open(a.DAG.Location, time.Now(), a.requestId); err != nil {
		return err
	}
	return nil
}

func (a *Agent) setupSocketServer() (err error) {
	a.socketServer, err = sock.NewServer(
		&sock.Config{
			Addr:        a.DAG.SockAddr(),
			HandlerFunc: a.handleHTTP,
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
	tl := &logger.TeeLogger{Writer: a.logManager.logFile}
	if err := tl.Open(); err != nil {
		return err
	}
	defer func() {
		utils.LogErr("close log file", a.closeLogFile())
		tl.Close()
	}()

	defer func() {
		if err := a.historyStore.Close(); err != nil {
			log.Printf("failed to close history store: %v", err)
		}
	}()

	utils.LogErr("write status", a.historyStore.Write(a.Status()))

	listen := make(chan error)
	go func() {
		err := a.socketServer.Serve(listen)
		if err != nil && err != sock.ErrServerRequestedShutdown {
			log.Printf("failed to start socket server %v", err)
		}
	}()

	defer func() {
		utils.LogErr("shutdown socket server", a.socketServer.Shutdown())
	}()

	if err := <-listen; err != nil {
		return fmt.Errorf("failed to start the socket server")
	}

	done := make(chan *scheduler.Node)
	defer close(done)

	go func() {
		for node := range done {
			status := a.Status()
			utils.LogErr("write status", a.historyStore.Write(status))
			utils.LogErr("report step", a.reporter.ReportStep(a.DAG, status, node))
		}
	}()

	go func() {
		time.Sleep(time.Millisecond * 100)
		utils.LogErr("write status", a.historyStore.Write(a.Status()))
	}()

	ctx = dag.NewContext(ctx, a.DAG)

	lastErr := a.scheduler.Schedule(ctx, a.graph, done)
	status := a.Status()

	log.Println("schedule finished.")
	utils.LogErr("write status", a.historyStore.Write(a.Status()))

	a.reporter.ReportSummary(status, lastErr)
	utils.LogErr("send email", a.reporter.SendMail(a.DAG, status, lastErr))
	utils.LogErr("close data file", a.historyStore.Close())

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

	ctx := dag.NewContext(context.Background(), a.DAG)

	lastErr := a.scheduler.Schedule(ctx, a.graph, done)
	status := a.Status()
	a.reporter.ReportSummary(status, lastErr)

	log.Printf("***** Finished DRY-RUN *****")

	return lastErr
}

func (a *Agent) checkIsRunning() error {
	status, err := controller.New(a.DAG, jsondb.New()).GetStatus()
	if err != nil {
		return err
	}
	if status.Status != scheduler.SchedulerStatus_None {
		return fmt.Errorf("the DAG is already running. socket=%s",
			a.DAG.SockAddr())
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

func (a *Agent) handleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && statusRe.MatchString(r.URL.Path):
		status := a.Status()
		status.Status = scheduler.SchedulerStatus_Running
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
	l.logFile, err = utils.OpenOrCreateFile(l.logFilename)
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
