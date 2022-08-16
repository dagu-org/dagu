package runner

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"github.com/yohamta/dagu/internal/utils"
)

type Entry struct {
	Next time.Time
	Job  Job
}

type EntryReader interface {
	Read(now time.Time) ([]*Entry, error)
}

func NewEntryReader(cfg *admin.Config) *entryReader {
	er := &entryReader{
		Admin: cfg,
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(
				settings.MustGet(settings.SETTING__SUSPEND_FLAGS_DIR),
			),
		),
		dagsLock: sync.Mutex{},
		dags:     map[string]*dag.DAG{},
	}
	if err := er.initDags(); err != nil {
		log.Printf("failed to init entry dags %v", err)
	}
	go er.watchDags()
	return er
}

type entryReader struct {
	Admin          *admin.Config
	suspendChecker *suspend.SuspendChecker
	dagsLock       sync.Mutex
	dags           map[string]*dag.DAG
}

var _ EntryReader = (*entryReader)(nil)

func (er *entryReader) Read(now time.Time) ([]*Entry, error) {
	entries := []*Entry{}
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()

	for _, dag := range er.dags {
		if er.suspendChecker.IsSuspended(dag) {
			continue
		}
		for _, sc := range dag.Schedule {
			next := sc.Next(now)
			entries = append(entries, &Entry{
				Next: sc.Next(now),
				Job: &job{
					DAG:       dag,
					Config:    er.Admin,
					StartTime: next,
				},
			})
		}
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
			dag, err := cl.LoadHeadOnly(filepath.Join(er.Admin.DAGs, fi.Name()))
			if err != nil {
				log.Printf("failed to read dag config: %s", err)
				continue
			}
			er.dags[fi.Name()] = dag
			fileNames = append(fileNames, fi.Name())
		}
	}
	log.Printf("init scheduler dags: %s", strings.Join(fileNames, ","))
	return nil
}

func (er *entryReader) watchDags() {
	cl := dag.Loader{}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	watcher.Add(er.Admin.DAGs)
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			er.dagsLock.Lock()
			if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
				dag, err := cl.LoadHeadOnly(filepath.Join(er.Admin.DAGs, filepath.Base(event.Name)))
				if err != nil {
					log.Printf("failed to read dag config: %s", err)
					continue
				}
				er.dags[filepath.Base(event.Name)] = dag
				log.Printf("reload dag entry %s", event.Name)
			}
			if event.Op == fsnotify.Rename || event.Op == fsnotify.Remove {
				delete(er.dags, filepath.Base(event.Name))
				log.Printf("remove dag entry %s", event.Name)
			}
			er.dagsLock.Unlock()
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("watch entry dags error:", err)
		}
	}

}
