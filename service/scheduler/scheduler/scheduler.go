package scheduler

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/dag"
	"log"
	"os"
	"os/signal"
	"path"
	"sort"
	"syscall"
	"time"

	"github.com/dagu-dev/dagu/internal/utils"
)

type Scheduler struct {
	entryReader EntryReader
	logDir      string
	stop        chan struct{}
	running     bool
}

type EntryReader interface {
	Read(now time.Time) ([]*Entry, error)
}

type Entry struct {
	Next      time.Time
	Job       Job
	EntryType Type
}

type Job interface {
	GetDAG() *dag.DAG
	Start() error
	Stop() error
	Restart() error
	String() string
}

type Type int

const (
	Start Type = iota
	Stop
	Restart
)

func (e *Entry) Invoke() error {
	if e.Job == nil {
		return nil
	}
	switch e.EntryType {
	case Start:
		log.Printf("[%s] start %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Start()
	case Stop:
		log.Printf("[%s] stop %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Stop()
	case Restart:
		log.Printf("[%s] restart %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Restart()
	}
	return nil
}

type Params struct {
	EntryReader EntryReader
	LogDir      string
}

func New(params Params) *Scheduler {
	return &Scheduler{
		entryReader: params.EntryReader,
		logDir:      params.LogDir,
		stop:        make(chan struct{}),
		running:     false,
	}
}

func (s *Scheduler) Start() error {
	if err := s.setupLogFile(); err != nil {
		return fmt.Errorf("setup log file: %w", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		s.Stop()
	}()

	log.Printf("starting dagu scheduler")
	s.start()

	return nil
}

func (s *Scheduler) setupLogFile() (err error) {
	filename := path.Join(s.logDir, "Scheduler.log")
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// TODO: fix this to use logger
	log.Printf("setup log file: %s", filename)
	log.Print("log file is ready")
	return
}

func (s *Scheduler) start() {
	t := utils.Now().Truncate(time.Second * 60)
	timer := time.NewTimer(0)
	s.running = true
	for {
		select {
		case <-timer.C:
			s.run(t)
			t = s.nextTick(t)
			timer = time.NewTimer(t.Sub(utils.Now()))
		case <-s.stop:
			_ = timer.Stop()
			return
		}
	}
}

func (s *Scheduler) run(now time.Time) {
	entries, err := s.entryReader.Read(now.Add(-time.Second))
	utils.LogErr("failed to read entries", err)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})
	for _, e := range entries {
		t := e.Next
		if t.After(now) {
			break
		}
		go func(e *Entry) {
			err := e.Invoke()
			if err != nil {
				log.Printf("backend: entry failed %s: %v", e.Job, err)
			}
		}(e)
	}
}

func (s *Scheduler) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (s *Scheduler) Stop() {
	if !s.running {
		return
	}
	if s.stop != nil {
		s.stop <- struct{}{}
	}
	s.running = false
}
