package entry

import (
	"github.com/dagu-dev/dagu/service/scheduler/filenotify"
	"github.com/dagu-dev/dagu/service/scheduler/scheduler"
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

type JobFactory interface {
	NewJob(dag *dag.DAG, next time.Time) scheduler.Job
}

func NewEntryReader(dagsDir string, jf JobFactory) *EntryReader {
	er := &EntryReader{
		dagsDir: dagsDir,
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

type EntryReader struct {
	dagsDir        string
	suspendChecker *suspend.SuspendChecker
	dagsLock       sync.Mutex
	dags           map[string]*dag.DAG
	jf             JobFactory
}

func (er *EntryReader) Read(now time.Time) ([]*scheduler.Entry, error) {
	var entries []*scheduler.Entry
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()

	f := func(d *dag.DAG, s []*dag.Schedule, e scheduler.Type) {
		for _, ss := range s {
			next := ss.Parsed.Next(now)
			entries = append(entries, &scheduler.Entry{
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
		f(d, d.Schedule, scheduler.Start)
		f(d, d.StopSchedule, scheduler.Stop)
		f(d, d.RestartSchedule, scheduler.Restart)
	}

	return entries, nil
}

func (er *EntryReader) initDags() error {
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()
	cl := dag.Loader{}
	fis, err := os.ReadDir(er.dagsDir)
	if err != nil {
		return err
	}
	fileNames := []string{}
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			workflow, err := cl.LoadMetadataOnly(filepath.Join(er.dagsDir, fi.Name()))
			if err != nil {
				log.Printf("init dags failed to read workflow cfg: %s", err)
				continue
			}
			er.dags[fi.Name()] = workflow
			fileNames = append(fileNames, fi.Name())
		}
	}
	log.Printf("init backend dags: %s", strings.Join(fileNames, ","))
	return nil
}

func (er *EntryReader) watchDags() {
	cl := dag.Loader{}
	watcher, err := filenotify.New(time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = watcher.Close()
	}()
	_ = watcher.Add(er.dagsDir)
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
				workflow, err := cl.LoadMetadataOnly(filepath.Join(er.dagsDir, filepath.Base(event.Name)))
				if err != nil {
					log.Printf("failed to read workflow cfg: %s", err)
				} else {
					er.dags[filepath.Base(event.Name)] = workflow
					log.Printf("reload workflow entry %s", event.Name)
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
