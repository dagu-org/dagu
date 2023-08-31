package scheduler

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/config"
	"log"
	"os"
	"os/signal"
	"path"
	"sort"
	"syscall"
	"time"

	"github.com/dagu-dev/dagu/internal/utils"
)

type Ticker struct {
	cfg         *config.Config
	entryReader EntryReader
	stop        chan struct{}
	running     bool
}

func NewTicker(params Params) *Ticker {
	return &Ticker{
		entryReader: params.EntryReader,
		cfg:         params.Config,
		stop:        make(chan struct{}),
		running:     false,
	}
}

func (r *Ticker) Start() error {
	if err := r.setupLogFile(); err != nil {
		return fmt.Errorf("setup log file: %w", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		r.Stop()
	}()

	log.Printf("start scheduler")
	r.start()

	return nil
}

func (a *Ticker) setupLogFile() (err error) {
	filename := path.Join(a.cfg.LogDir, "scheduler.log")
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// TODO: fix this to use logger
	log.Printf("setup log file: %s", filename)
	log.Print("log file is ready")
	return
}

func (r *Ticker) start() {
	t := utils.Now().Truncate(time.Second * 60)
	timer := time.NewTimer(0)
	r.running = true
	for {
		select {
		case <-timer.C:
			r.run(t)
			t = r.nextTick(t)
			timer = time.NewTimer(t.Sub(utils.Now()))
		case <-r.stop:
			_ = timer.Stop()
			return
		}
	}
}

func (r *Ticker) run(now time.Time) {
	entries, err := r.entryReader.Read(now.Add(-time.Second))
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

func (r *Ticker) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (r *Ticker) Stop() {
	if !r.running {
		return
	}
	if r.stop != nil {
		r.stop <- struct{}{}
	}
	r.running = false
}
