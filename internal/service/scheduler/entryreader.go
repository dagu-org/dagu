package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler/filenotify"
	"github.com/robfig/cron/v3"

	"github.com/fsnotify/fsnotify"
)

// EntryReader is responsible for managing scheduled Jobs.
type EntryReader interface {
	// Start initializes and starts the process of managing scheduled Jobs.
	Start(ctx context.Context, done chan any) error
	// Next returns the next scheduled jobs.
	Next(ctx context.Context, now time.Time) ([]*ScheduledJob, error)
}

// ScheduledJob stores the next time a job should be run and the job itself.
type ScheduledJob struct {
	Next time.Time // Next is the time when the job should be run.
	Job  Job
	Type ScheduleType // start, stop, restart
}

// NewScheduledJob creates a new ScheduledJob.
func NewScheduledJob(next time.Time, job Job, typ ScheduleType) *ScheduledJob {
	return &ScheduledJob{next, job, typ}
}

var _ EntryReader = (*entryReaderImpl)(nil)

// entryReaderImpl manages DAGs on local filesystem.
type entryReaderImpl struct {
	targetDir   string
	registry    map[string]*core.DAG
	lock        sync.Mutex
	dagStore    execution.DAGStore
	dagRunMgr   runtime.Manager
	executable  string
	dagExecutor *DAGExecutor
}

// NewEntryReader creates a new DAG manager with the given configuration.
func NewEntryReader(dir string, dagCli execution.DAGStore, drm runtime.Manager, de *DAGExecutor, executable string) EntryReader {
	return &entryReaderImpl{
		targetDir:   dir,
		lock:        sync.Mutex{},
		registry:    map[string]*core.DAG{},
		dagStore:    dagCli,
		dagRunMgr:   drm,
		executable:  executable,
		dagExecutor: de,
	}
}

func (er *entryReaderImpl) Start(ctx context.Context, done chan any) error {
	if err := er.initialize(ctx); err != nil {
		logger.Error(ctx, "Failed to initialize DAG registry", tag.Error(err))
		return fmt.Errorf("failed to initialize DAGs: %w", err)
	}

	go er.watchDags(ctx, done)

	return nil
}

func (er *entryReaderImpl) Next(ctx context.Context, now time.Time) ([]*ScheduledJob, error) {
	er.lock.Lock()
	defer er.lock.Unlock()

	var jobs []*ScheduledJob

	for _, dag := range er.registry {
		dagName := strings.TrimSuffix(filepath.Base(dag.Location), filepath.Ext(dag.Location))
		if er.dagStore.IsSuspended(ctx, dagName) {
			logger.Debug(ctx, "Skipping suspended DAG", tag.DAG(dagName))
			continue
		}

		schedules := []struct {
			items []core.Schedule
			typ   ScheduleType
		}{
			{dag.Schedule, ScheduleTypeStart},
			{dag.StopSchedule, ScheduleTypeStop},
			{dag.RestartSchedule, ScheduleTypeRestart},
		}

		for _, s := range schedules {
			for _, schedule := range s.items {
				next := schedule.Parsed.Next(now)
				job := NewScheduledJob(next, er.createJob(dag, next, schedule.Parsed), s.typ)
				jobs = append(jobs, job)
			}
		}
	}

	return jobs, nil
}

func (er *entryReaderImpl) createJob(dag *core.DAG, next time.Time, schedule cron.Schedule) Job {
	return &DAGRunJob{
		DAG:         dag,
		Next:        next,
		Schedule:    schedule,
		Client:      er.dagRunMgr,
		DAGExecutor: er.dagExecutor,
	}
}

func (er *entryReaderImpl) initialize(ctx context.Context) error {
	er.lock.Lock()
	defer er.lock.Unlock()

	logger.Info(ctx, "Loading DAGs", tag.Dir(er.targetDir))
	fis, err := os.ReadDir(er.targetDir)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG directory",
			tag.Dir(er.targetDir),
			tag.Error(err),
		)
		return err
	}

	var dags []string
	for _, fi := range fis {
		if fileutil.IsYAMLFile(fi.Name()) {
			dag, err := spec.Load(
				ctx,
				filepath.Join(er.targetDir, fi.Name()),
				spec.OnlyMetadata(),
				spec.WithoutEval(),
				spec.SkipSchemaValidation(),
			)
			if err != nil {
				logger.Error(ctx, "DAG load failed",
					tag.Error(err),
					tag.Name(fi.Name()))
				continue
			}
			er.registry[fi.Name()] = dag
			dags = append(dags, fi.Name())
		}
	}

	logger.Info(ctx, "DAGs loaded", slog.String("dags", strings.Join(dags, ",")))
	return nil
}

func (er *entryReaderImpl) watchDags(ctx context.Context, done chan any) {
	watcher := filenotify.New(time.Minute)

	defer func() {
		_ = watcher.Close()
	}()

	if err := watcher.Add(er.targetDir); err != nil {
		logger.Warn(ctx, "Failed to watch DAG directory",
			tag.Dir(er.targetDir),
			tag.Error(err),
		)
	}

	for {
		select {
		case <-done:
			return

		case event, ok := <-watcher.Events():
			if !ok {
				return
			}

			if !fileutil.IsYAMLFile(event.Name) {
				continue
			}

			er.lock.Lock()
			if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
				filePath := filepath.Join(er.targetDir, filepath.Base(event.Name))
				dag, err := spec.Load(
					ctx,
					filePath,
					spec.OnlyMetadata(),
					spec.WithoutEval(),
					spec.SkipSchemaValidation(),
				)
				if err != nil {
					logger.Error(ctx, "DAG load failed",
						tag.Error(err),
						tag.File(event.Name))
				} else {
					er.registry[filepath.Base(event.Name)] = dag
					logger.Info(ctx, "DAG added/updated", tag.Name(filepath.Base(event.Name)))
				}
			}
			if event.Op == fsnotify.Rename || event.Op == fsnotify.Remove {
				delete(er.registry, filepath.Base(event.Name))
				logger.Info(ctx, "DAG removed", tag.Name(filepath.Base(event.Name)))
			}
			er.lock.Unlock()

		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			logger.Error(ctx, "Watcher error", tag.Error(err))

		}
	}
}
