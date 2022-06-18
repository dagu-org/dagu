package agent

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/internal/mailer"
	"github.com/yohamta/dagu/internal/models"
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
	logFilename  string
	reporter     *reporter.Reporter
	database     *database.Database
	dbFile       string
	dbWriter     *database.Writer
	socketServer *sock.Server
	requestId    string
}

type AgentConfig struct {
	DAG *config.Config
	Dry bool
}

type RetryConfig struct {
	Status *models.Status
}

// Run starts the workflow.
func (a *Agent) Run() error {
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
	}
	for _, fn := range setup {
		err := fn()
		if err != nil {
			return err
		}
	}
	return a.run()
}

// Status returns the current status of the workflow.
func (a *Agent) Status() *models.Status {
	status := models.NewStatus(
		a.DAG,
		a.graph.Nodes(),
		a.scheduler.Status(a.graph),
		os.Getpid(),
		&a.graph.StartedAt,
		&a.graph.FinishedAt,
	)
	status.RequestId = a.requestId
	status.Log = a.logFilename
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
	log.Printf("Sending %s signal to running child processes.", sig)
	done := make(chan bool)
	go func() {
		a.scheduler.Signal(a.graph, sig, done)
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
			a.scheduler.Signal(a.graph, sig, nil)
			tick = time.After(time.Second * 5)
		default:
			log.Printf("Waiting for child processes to exit...")
			time.Sleep(time.Second * 3)
		}
	}
}

// Kill sends KILL signal to all child processes.
func (a *Agent) Kill() {
	log.Printf("Sending KILL signal to running child processes.")
	a.scheduler.Signal(a.graph, syscall.SIGKILL, nil)
}

// Cancel sends signal -1 to all child processes.
func (a *Agent) Cancel() {
	log.Printf("Sending -1 signal to running child processes.")
	a.scheduler.Cancel(a.graph)
	for a.scheduler.Status(a.graph) == scheduler.SchedulerStatus_Running {
		time.Sleep(time.Second * 5)
		a.scheduler.Cancel(a.graph)
	}
}

func (a *Agent) init() {
	logDir := path.Join(a.DAG.LogDir, utils.ValidFilename(a.DAG.Name, "_"))
	a.scheduler = scheduler.New(
		&scheduler.Config{
			LogDir:        logDir,
			MaxActiveRuns: a.DAG.MaxActiveRuns,
			Delay:         a.DAG.Delay,
			Dry:           a.Dry,
			OnExit:        a.DAG.HandlerOn.Exit,
			OnSuccess:     a.DAG.HandlerOn.Success,
			OnFailure:     a.DAG.HandlerOn.Failure,
			OnCancel:      a.DAG.HandlerOn.Cancel,
			RequestId:     a.requestId,
		})
	a.reporter = &reporter.Reporter{
		Config: &reporter.Config{
			Mailer: &mailer.Mailer{
				Config: &mailer.Config{
					Host: a.DAG.Smtp.Host,
					Port: a.DAG.Smtp.Port,
				},
			},
		}}
	a.logFilename = filepath.Join(
		logDir,
		fmt.Sprintf("%s.%s.%s.log",
			utils.ValidFilename(a.DAG.Name, "_"),
			time.Now().Format("20060102.15:04:05.000"),
			utils.TruncString(a.requestId, 8),
		))
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
	a.graph, err = scheduler.RetryExecutionGraph(nodes...)
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

func (a *Agent) setupDatabase() (err error) {
	a.database = database.New(database.DefaultConfig())
	a.dbWriter, a.dbFile, err = a.database.NewWriter(a.DAG.ConfigPath, time.Now(), a.requestId)
	return
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
		log.Printf("checking pre conditions for \"%s\"", a.DAG.Name)
		if err := config.EvalConditions(a.DAG.Preconditions); err != nil {
			a.scheduler.Cancel(a.graph)
			return err
		}
	}
	return nil
}

func (a *Agent) run() error {
	tl := &logger.TeeLogger{
		Filename: a.logFilename,
	}
	if err := tl.Open(); err != nil {
		return err
	}
	defer func() {
		utils.LogErr("close logger", tl.Close())
	}()

	if err := a.dbWriter.Open(); err != nil {
		return err
	}

	defer func() {
		if err := a.dbWriter.Close(); err != nil {
			log.Printf("failed to close db writer: %v", err)
		}
	}()

	utils.LogErr("write status", a.dbWriter.Write(a.Status()))

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
			utils.LogErr("write status", a.dbWriter.Write(status))
			utils.LogErr("report step", a.reporter.ReportStep(a.DAG, status, node))
		}
	}()

	go func() {
		time.Sleep(time.Millisecond * 100)
		utils.LogErr("write status", a.dbWriter.Write(a.Status()))
	}()

	lastErr := a.scheduler.Schedule(a.graph, done)
	status := a.Status()

	log.Println("schedule finished.")
	utils.LogErr("write status", a.dbWriter.Write(a.Status()))

	a.reporter.ReportSummary(status, lastErr)
	utils.LogErr("send email", a.reporter.SendMail(a.DAG, status, lastErr))

	utils.LogErr("close data file", a.dbWriter.Close())
	utils.LogErr("data compaction", a.database.Compact(a.DAG.ConfigPath, a.dbFile))

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

	lastErr := a.scheduler.Schedule(a.graph, done)
	status := a.Status()
	a.reporter.ReportSummary(status, lastErr)

	log.Printf("***** Finished DRY-RUN *****")

	return lastErr
}

func (a *Agent) checkIsRunning() error {
	status, err := controller.New(a.DAG).GetStatus()
	if err != nil {
		return err
	}
	if status.Status != scheduler.SchedulerStatus_None {
		return fmt.Errorf("the DAG is already running. socket=%s",
			a.DAG.SockAddr())
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
		w.Write(b)
	case r.Method == http.MethodPost && stopRe.MatchString(r.URL.Path):
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		go func() {
			a.Signal(syscall.SIGTERM)
		}()
	default:
		encodeError(w, errNotFound)
	}
}

var errNotFound = errors.New("not found")

func encodeError(w http.ResponseWriter, err error) {
	switch err {
	case errNotFound:
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
