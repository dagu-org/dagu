// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/scheduler/filenotify"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/util"
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

type jobCreator interface {
	CreateJob(workflow *dag.DAG, next time.Time) job
}

func newEntryReader(dagsDir string, jobCreator jobCreator, logger logger.Logger, client client.Client) *entryReaderImpl {
	er := &entryReaderImpl{
		dagsDir:    dagsDir,
		dagsLock:   sync.Mutex{},
		dags:       map[string]*dag.DAG{},
		jobCreator: jobCreator,
		logger:     logger,
		client:     client,
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
