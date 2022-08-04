package runner

import (
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/logger"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Agent struct {
	*admin.Config
	logger *logger.TeeLogger
	stop   chan struct{}
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
	if err := a.logger.Open(); err != nil {
		return err
	}
	defer func() {
		a.logger.Close()
	}()

	log.Printf("starting dagu scheduler")
	a.stop = make(chan struct{})
	runner := New(NewEntryReader(a.Config))
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
	filename := path.Join(a.LogDir, "scheduler.log")
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	log.Printf("setup log file: %s", filename)

	a.logger = &logger.TeeLogger{
		Writer: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    20, // megabytes
			MaxBackups: 3,
			MaxAge:     28,    //days
			Compress:   false, // disabled by default
		},
	}

	log.Print("log file setuped")
	return
}
