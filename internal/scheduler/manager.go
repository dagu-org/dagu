package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/scheduler/filenotify"
	"github.com/robfig/cron/v3"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/fsnotify/fsnotify"
)

// JobManager is responsible for managing scheduled Jobs.
type JobManager interface {
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

var _ JobManager = (*dagJobManager)(nil)

// dagJobManager manages DAGs on local filesystem.
type dagJobManager struct {
	targetDir      string
	registry       map[string]*digraph.DAG
	lock           sync.Mutex
	dagClient      models.DAStorage
	historyManager history.Manager
	executable     string
	workDir        string
}

// NewDAGJobManager creates a new DAG manager with the given configuration.
func NewDAGJobManager(dir string, dagCli models.DAStorage, hm history.Manager, executable, workDir string) JobManager {
	return &dagJobManager{
		targetDir:      dir,
		lock:           sync.Mutex{},
		registry:       map[string]*digraph.DAG{},
		dagClient:      dagCli,
		historyManager: hm,
		executable:     executable,
		workDir:        workDir,
	}
}

func (m *dagJobManager) Start(ctx context.Context, done chan any) error {
	if err := m.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize DAGs: %w", err)
	}

	go m.watchDags(ctx, done)

	return nil
}

func (m *dagJobManager) Next(ctx context.Context, now time.Time) ([]*ScheduledJob, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var jobs []*ScheduledJob

	for _, dag := range m.registry {
		dagName := strings.TrimSuffix(filepath.Base(dag.Location), filepath.Ext(dag.Location))
		if m.dagClient.IsSuspended(ctx, dagName) {
			continue
		}

		schedules := []struct {
			items []digraph.Schedule
			typ   ScheduleType
		}{
			{dag.Schedule, ScheduleTypeStart},
			{dag.StopSchedule, ScheduleTypeStop},
			{dag.RestartSchedule, ScheduleTypeRestart},
		}

		for _, s := range schedules {
			for _, schedule := range s.items {
				next := schedule.Parsed.Next(now)
				job := NewScheduledJob(next, m.createJob(dag, next, schedule.Parsed), s.typ)
				jobs = append(jobs, job)
			}
		}
	}

	return jobs, nil
}

func (m *dagJobManager) createJob(dag *digraph.DAG, next time.Time, schedule cron.Schedule) Job {
	return &DAG{
		DAG:        dag,
		Executable: m.executable,
		WorkDir:    m.workDir,
		Next:       next,
		Schedule:   schedule,
		Client:     m.historyManager,
	}
}

func (m *dagJobManager) initialize(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	logger.Info(ctx, "Loading DAGs", "dir", m.targetDir)
	fis, err := os.ReadDir(m.targetDir)
	if err != nil {
		return err
	}

	var dags []string
	for _, fi := range fis {
		if fileutil.IsYAMLFile(fi.Name()) {
			dag, err := digraph.Load(ctx, filepath.Join(m.targetDir, fi.Name()), digraph.OnlyMetadata(), digraph.WithoutEval())
			if err != nil {
				logger.Error(ctx, "DAG load failed", "err", err, "name", fi.Name())
				continue
			}
			m.registry[fi.Name()] = dag
			dags = append(dags, fi.Name())
		}
	}

	logger.Info(ctx, "DAGs loaded", "dags", strings.Join(dags, ","))
	return nil
}

func (m *dagJobManager) watchDags(ctx context.Context, done chan any) {
	watcher, err := filenotify.New(time.Minute)
	if err != nil {
		logger.Error(ctx, "Watcher creation failed", "err", err)
		return
	}

	defer func() {
		_ = watcher.Close()
	}()

	_ = watcher.Add(m.targetDir)

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

			m.lock.Lock()
			if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
				filePath := filepath.Join(m.targetDir, filepath.Base(event.Name))
				dag, err := digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
				if err != nil {
					logger.Error(ctx, "DAG load failed", "err", err, "file", event.Name)
				} else {
					m.registry[filepath.Base(event.Name)] = dag
					logger.Info(ctx, "DAG added/updated", "name", filepath.Base(event.Name))
				}
			}
			if event.Op == fsnotify.Rename || event.Op == fsnotify.Remove {
				delete(m.registry, filepath.Base(event.Name))
				logger.Info(ctx, "DAG removed", "name", filepath.Base(event.Name))
			}
			m.lock.Unlock()

		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			logger.Error(ctx, "Watcher error", "err", err)

		}
	}
}
