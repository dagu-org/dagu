package runner

import (
	"log"
	"sort"
	"time"

	"github.com/yohamta/dagu/internal/utils"
)

type Runner struct {
	entryReader EntryReader
	running     bool
	stop        chan struct{}
}

func New(er EntryReader) *Runner {
	return &Runner{
		entryReader: er,
		stop:        make(chan struct{}),
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
			if e.Job != nil {
				err := e.Job.Run()
				if err != nil {
					log.Printf("runner: failed to run job %s: %v", e.Job, err)
				}
			}
		}(e)
	}
}

func (r *Runner) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (r *Runner) Stop() {
	if !r.running {
		return
	}
	r.stop <- struct{}{}
	r.running = false
}
