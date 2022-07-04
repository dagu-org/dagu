package runner

import (
	"fmt"
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
	stop   chan struct{}
	logger *logger.SimpleLogger
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
	tl := &logger.TeeLogger{Writer: a.logger}
	if err := tl.Open(); err != nil {
		return err
	}
	defer func() {
		utils.LogErr("close log file", a.closeLogFile())
		tl.Close()
	}()

	log.Printf("starting dagu scheduler")
	a.stop = make(chan struct{})
	runner := New(&Config{Admin: a.Config})
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
	filename := path.Join(
		a.LogDir,
		fmt.Sprintf("scheduler.%s.log",
			time.Now().Format("20060102.15:04:05.000"),
		))
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	a.logger = logger.NewSimpleLogger(
		a.LogDir, "scheduler", time.Hour*24,
	)
	err = a.logger.Open()
	return
}

func (a *Agent) closeLogFile() error {
	if a.logger != nil {
		return a.logger.Close()
	}
	return nil
}
