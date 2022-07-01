package runner

import (
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/internal/utils"
)

type Agent struct {
	*admin.Config
	now     time.Time // For testing
	stop    chan struct{}
	logFile *os.File
}

func NewAgent(cfg *admin.Config) *Agent {
	return &Agent{
		Config: cfg,
	}
}

func (a *Agent) Start() error {
	setup := []func() error{
		a.setupLogFile,
	}
	for _, fn := range setup {
		err := fn()
		if err != nil {
			return err
		}
	}
	return a.start()
}

func (a *Agent) Stop() {
	a.stop <- struct{}{}
}

func (a *Agent) start() error {
	// TODO: log rotation
	tl := &logger.TeeLogger{Writer: a.logFile}
	if err := tl.Open(); err != nil {
		return err
	}
	defer func() {
		utils.LogErr("close log file", a.closeLogFile())
		tl.Close()
	}()

	log.Printf("starting dagu scheduler")
	a.stop = make(chan struct{})
	runner := New(&Config{Admin: a.Config, Now: a.now})
	a.registerRunnerShutdown(runner)

	go runner.Start()

	<-a.stop
	runner.Stop()

	return nil
}

func (a *Agent) registerRunnerShutdown(runner *Runner) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		log.Printf("got signal: %v", s)
		a.Stop()

		log.Printf("terminated")
		os.Exit(1)
	}()
}

func (a *Agent) setupLogFile() (err error) {
	file := path.Join(a.LogDir, "scheduler.log")
	dir := path.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	a.logFile, err = utils.OpenOrCreateFile(file)
	return
}

func (a *Agent) closeLogFile() error {
	if a.logFile != nil {
		return a.logFile.Close()
	}
	return nil
}
