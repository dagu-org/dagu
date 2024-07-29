package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/daguflow/dagu/internal/client"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/scheduler/filenotify"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/util"
	"github.com/fsnotify/fsnotify"
)

var _ entryReader = (*entryReaderImpl)(nil)

type entryReaderImpl struct {
	dagsDir    string
	dagsLock   sync.Mutex
	dags       map[string]*dag.DAG
	jobCreator jobCreator
	logger     logger.Logger
	client     client.Client
}

type newEntryReaderArgs struct {
	DagsDir    string
	JobCreator jobCreator
	Logger     logger.Logger
	Client     client.Client
}

type jobCreator interface {
	CreateJob(workflow *dag.DAG, next time.Time) job
}

func newEntryReader(args newEntryReaderArgs) *entryReaderImpl {
	er := &entryReaderImpl{
		dagsDir:    args.DagsDir,
		dagsLock:   sync.Mutex{},
		dags:       map[string]*dag.DAG{},
		jobCreator: args.JobCreator,
		logger:     args.Logger,
		client:     args.Client,
	}
	if err := er.initDags(); err != nil {
		er.logger.Error("DAG initialization failed", "error", err)
	}
	return er
}

func (er *entryReaderImpl) Start(done chan any) {
	go er.watchDags(done)
}

func (er *entryReaderImpl) Read(now time.Time) ([]*entry, error) {
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()

	var entries []*entry
	addEntriesFn := func(workflow *dag.DAG, s []dag.Schedule, e entryType) {
		for _, ss := range s {
			next := ss.Parsed.Next(now)
			entries = append(entries, &entry{
				Next:      ss.Parsed.Next(now),
				Job:       er.jobCreator.CreateJob(workflow, next),
				EntryType: e,
				Logger:    er.logger,
			})
		}
	}

	for _, workflow := range er.dags {
		id := strings.TrimSuffix(
			filepath.Base(workflow.Location),
			filepath.Ext(workflow.Location),
		)

		if er.client.IsSuspended(id) {
			continue
		}
		addEntriesFn(workflow, workflow.Schedule, entryTypeStart)
		addEntriesFn(workflow, workflow.StopSchedule, entryTypeStop)
		addEntriesFn(workflow, workflow.RestartSchedule, entryTypeRestart)
	}

	return entries, nil
}

func (er *entryReaderImpl) initDags() error {
	er.dagsLock.Lock()
	defer er.dagsLock.Unlock()

	fis, err := os.ReadDir(er.dagsDir)
	if err != nil {
		return err
	}

	var fileNames []string
	for _, fi := range fis {
		if util.MatchExtension(fi.Name(), dag.Exts) {
			workflow, err := dag.LoadMetadata(
				filepath.Join(er.dagsDir, fi.Name()),
			)
			if err != nil {
				er.logger.Error(
					"Workflow load failed",
					"error", err,
					"workflow", fi.Name(),
				)
				continue
			}
			er.dags[fi.Name()] = workflow
			fileNames = append(fileNames, fi.Name())
		}
	}

	er.logger.Info("Scheduler initialized", "specs", strings.Join(fileNames, ","))
	return nil
}

func (er *entryReaderImpl) watchDags(done chan any) {
	watcher, err := filenotify.New(time.Minute)
	if err != nil {
		er.logger.Error("Watcher creation failed", "error", err)
		return
	}

	defer func() {
		_ = watcher.Close()
	}()
	_ = watcher.Add(er.dagsDir)

	for {
		select {
		case <-done:
			return
		case event, ok := <-watcher.Events():
			if !ok {
				return
			}
			if !util.MatchExtension(event.Name, dag.Exts) {
				continue
			}
			er.dagsLock.Lock()
			if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
				workflow, err := dag.LoadMetadata(
					filepath.Join(er.dagsDir, filepath.Base(event.Name)),
				)
				if err != nil {
					er.logger.Error(
						"Workflow load failed",
						"error",
						err,
						"file",
						event.Name,
					)
				} else {
					er.dags[filepath.Base(event.Name)] = workflow
					er.logger.Info("Workflow added/updated", "workflow", filepath.Base(event.Name))
				}
			}
			if event.Op == fsnotify.Rename || event.Op == fsnotify.Remove {
				delete(er.dags, filepath.Base(event.Name))
				er.logger.Info("Workflow removed", "workflow", filepath.Base(event.Name))
			}
			er.dagsLock.Unlock()
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			er.logger.Error("Watcher error", "error", err)
		}
	}

}
