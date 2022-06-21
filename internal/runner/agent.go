package runner

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yohamta/dagu/internal/admin"
)

type Agent struct {
	*admin.Config
	now  time.Time // For testing
	stop chan struct{}
}

func NewAgent(cfg *admin.Config) *Agent {
	return &Agent{
		Config: cfg,
	}
}

func (a *Agent) Start() error {
	a.stop = make(chan struct{})
	runner := New(&Config{Admin: a.Config, Now: a.now})
	a.registerRunnerShutdown(runner)

	go runner.Start()

	<-a.stop
	runner.Stop()

	return nil
}

func (a *Agent) Stop() {
	a.stop <- struct{}{}
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
