package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
)

type Config struct {
	Admin *admin.Config
}

type Runner struct {
	*Config
	running bool
	stop    chan struct{}
}

type Entry struct {
	Next time.Time
	Job  Job
}

func New(cfg *Config) *Runner {
	return &Runner{
		Config: cfg,
		stop:   make(chan struct{}),
	}
}

func (r *Runner) Start() {
	r.init()
	t := utils.Now().Truncate(time.Second * 60)
	timer := time.NewTimer(0)
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

func (r *Runner) init() {
	r.running = true
}

func (r *Runner) run(now time.Time) {
	entries, err := r.readEntries(now.Add(-time.Second))
	utils.LogErr("failed to read entries", err)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})
	for _, e := range entries {
		t := e.Next
		if t.After(now) {
			break
		}
		go runWithRecovery(e.Job)
	}
}

func (r *Runner) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (r *Runner) readEntries(now time.Time) (entries []*Entry, err error) {
	cl := config.Loader{}
	for {
		fis, err := os.ReadDir(r.Admin.DAGs)
		if err != nil {
			return entries, fmt.Errorf("failed to read entries directory: %w", err)
		}
		for _, fi := range fis {
			if utils.MatchExtension(fi.Name(), config.EXTENSIONS) {
				dag, err := cl.LoadHeadOnly(
					filepath.Join(r.Admin.DAGs, fi.Name()),
				)
				if err != nil {
					log.Printf("failed to read dag config: %s", err)
					continue
				}
				if dag.Schedule != nil {
					entries = append(entries, &Entry{
						Next: dag.Schedule.Next(now),
						Job: &job{
							DAG:    dag,
							Config: r.Config.Admin,
						},
					})
				}
			}
		}
		return entries, nil
	}
}

func (r *Runner) Stop() {
	if !r.running {
		return
	}
	r.stop <- struct{}{}
	r.running = false
}

func runWithRecovery(j Job) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("runner: panic running job: %v\n%s", r, buf)
		}
	}()

	err := j.Run()
	if err != nil {
		log.Printf("runner: failed to run job %s: %v", j, err)
	}
}
