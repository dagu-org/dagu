package entry

import (
	"github.com/dagu-dev/dagu/service/scheduler/filenotify"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/fsnotify/fsnotify"
)

type EntryType int

const (
	EntryTypeStart EntryType = iota
	EntryTypeStop
	EntryTypeRestart
)

type Job interface {
	GetDAG() *dag.DAG
	Start() error
	Stop() error
	Restart() error
	String() string
}

type JobFactory interface {
	NewJob(dag *dag.DAG, next time.Time) Job
}

type Entry struct {
	Next      time.Time
	Job       Job
	EntryType EntryType
}

func (e *Entry) Invoke() error {
	if e.Job == nil {
		return nil
	}
	switch e.EntryType {
	case EntryTypeStart:
		log.Printf("[%s] start %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Start()
	case EntryTypeStop:
		log.Printf("[%s] stop %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Stop()
	case EntryTypeRestart:
		log.Printf("[%s] restart %s", e.Next.Format("2006-01-02 15:04:05"), e.Job.String())
		return e.Job.Restart()
	}
	return nil
}

func NewEntryReader(dagsDir string, cfg *config.Config, jf JobFactory) *entryReader {
	er := &entryReader{
		dagsDir: dagsDir,
		Admin:   cfg,
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(config.Get().SuspendFlagsDir),
		),
		dagsLock: sync.Mutex{},
		dags:     map[string]*dag.DAG{},
		jf:       jf,
	}
	if err := er.initDags(); err != nil {
		log.Printf("failed to init entry dags %v", err)
	}
	go er.watchDags()
	return er
}

type entryReader struct {
	dagsDir        string
	Admin          *config.Config
	suspendChecker *suspend.SuspendChecker
	dagsLock       sync.Mutex
	dags           map[string]*dag.DAG
	jf             JobFactory
}

func (er *entryReader) Read(now time.Time) ([]*Entry, error) {
	entries := []*Entry{}
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()

	f := func(d *dag.DAG, s []*dag.Schedule, e EntryType) {
		for _, ss := range s {
			next := ss.Parsed.Next(now)
			entries = append(entries, &Entry{
				Next: ss.Parsed.Next(now),
				// TODO: fix this
				Job:       er.jf.NewJob(d, next),
				EntryType: e,
			})
		}
	}

	for _, d := range er.dags {
		if er.suspendChecker.IsSuspended(d) {
			continue
		}
		f(d, d.Schedule, EntryTypeStart)
		f(d, d.StopSchedule, EntryTypeStop)
		f(d, d.RestartSchedule, EntryTypeRestart)
	}

	return entries, nil
}

func (er *entryReader) initDags() error {
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()
	cl := dag.Loader{}
	fis, err := os.ReadDir(er.Admin.DAGs)
	if err != nil {
		return err
	}
	fileNames := []string{}
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			dag, err := cl.LoadMetadataOnly(filepath.Join(er.Admin.DAGs, fi.Name()))
			if err != nil {
				log.Printf("init dags failed to read dag cfg: %s", err)
				continue
			}
			er.dags[fi.Name()] = dag
			fileNames = append(fileNames, fi.Name())
		}
	}
	log.Printf("init backend dags: %s", strings.Join(fileNames, ","))
	return nil
}

func (er *entryReader) watchDags() {
	cl := dag.Loader{}
	watcher, err := filenotify.New(time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = watcher.Close()
	}()
	_ = watcher.Add(er.Admin.DAGs)
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				return
			}
			if !utils.MatchExtension(event.Name, dag.EXTENSIONS) {
				continue
			}
			er.dagsLock.Lock()
			if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
				dag, err := cl.LoadMetadataOnly(filepath.Join(er.Admin.DAGs, filepath.Base(event.Name)))
				if err != nil {
					log.Printf("failed to read dag cfg: %s", err)
				} else {
					er.dags[filepath.Base(event.Name)] = dag
					log.Printf("reload dag entry %s", event.Name)
				}
			}
			if event.Op == fsnotify.Rename || event.Op == fsnotify.Remove {
				delete(er.dags, filepath.Base(event.Name))
				log.Printf("remove dag entry %s", event.Name)
			}
			er.dagsLock.Unlock()
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			log.Println("watch entry dags error:", err)
		}
	}

}
