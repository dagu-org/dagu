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

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler/filenotify"
	"github.com/robfig/cron/v3"

	"github.com/fsnotify/fsnotify"
)

// EntryReader is responsible for managing scheduled Jobs.
type EntryReader interface {
	// Init initializes the DAG registry by loading all DAGs from the target directory.
	// This must be called before Start or Next.
	Init(ctx context.Context) error
	// Start starts watching the DAG directory for changes.
	// This method blocks until Stop is called or context is canceled.
	Start(ctx context.Context)
	// Stop stops watching the DAG directory.
	Stop()
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

// entryReaderImpl manages DAGs on local filesystem across all namespaces.
// It iterates NamespaceStore.List() and watches {DAGsDir}/{id}/ for each
// namespace, setting dag.Namespace on every loaded DAG.
type entryReaderImpl struct {
	dagsDir        string               // base DAGs directory
	namespaceStore exec.NamespaceStore  // namespace store for listing namespaces
	registry       map[string]*core.DAG // key: id/filename → DAG
	knownNS        map[string]string    // id → namespace name
	nsDirs         map[string]string    // dir path → id
	lock           sync.Mutex
	dagStore       exec.DAGStore
	dagRunMgr      runtime.Manager
	executable     string
	dagExecutor    *DAGExecutor
	watcher        filenotify.FileWatcher
	quit           chan struct{}
	closeOnce      sync.Once
}

// NewEntryReader creates a new namespace-aware DAG manager.
// It scans {dagsDir}/{id}/ subdirectories for each namespace.
func NewEntryReader(
	dir string,
	dagCli exec.DAGStore,
	drm runtime.Manager,
	de *DAGExecutor,
	executable string,
	namespaceStore exec.NamespaceStore,
) EntryReader {
	return &entryReaderImpl{
		dagsDir:        dir,
		registry:       map[string]*core.DAG{},
		knownNS:        map[string]string{},
		nsDirs:         map[string]string{},
		dagStore:       dagCli,
		dagRunMgr:      drm,
		executable:     executable,
		dagExecutor:    de,
		namespaceStore: namespaceStore,
		quit:           make(chan struct{}),
	}
}

func (er *entryReaderImpl) Init(ctx context.Context) error {
	er.lock.Lock()
	defer er.lock.Unlock()

	// Create the file watcher
	er.watcher = filenotify.New(time.Minute)

	// Discover all namespaces and load their DAGs
	if err := er.syncNamespaces(ctx); err != nil {
		_ = er.watcher.Close()
		logger.Error(ctx, "Failed to initialize namespace DAG registry", tag.Error(err))
		return fmt.Errorf("failed to initialize namespace DAGs: %w", err)
	}

	return nil
}

func (er *entryReaderImpl) Start(ctx context.Context) {
	// Periodic namespace rescan to pick up new namespaces without restart
	namespaceTicker := time.NewTicker(5 * time.Minute)
	defer namespaceTicker.Stop()

	for {
		select {
		case <-er.quit:
			return

		case <-ctx.Done():
			return

		case <-namespaceTicker.C:
			er.lock.Lock()
			if err := er.syncNamespaces(ctx); err != nil {
				logger.Error(ctx, "Failed to resync namespaces", tag.Error(err))
			}
			er.lock.Unlock()

		case event, ok := <-er.watcher.Events():
			if !ok {
				return
			}

			if !fileutil.IsYAMLFile(event.Name) {
				continue
			}

			er.lock.Lock()
			er.handleFileEvent(ctx, event)
			er.lock.Unlock()

		case err, ok := <-er.watcher.Errors():
			if !ok {
				return
			}
			logger.Error(ctx, "Watcher error", tag.Error(err))
		}
	}
}

func (er *entryReaderImpl) Stop() {
	er.lock.Lock()
	defer er.lock.Unlock()

	er.closeOnce.Do(func() {
		close(er.quit)
		if er.watcher != nil {
			_ = er.watcher.Close()
		}
	})
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

// syncNamespaces discovers namespaces from the NamespaceStore and loads DAGs
// from each namespace's subdirectory. It also registers watchers for new
// namespace directories and removes DAGs for deleted namespaces.
// The caller must hold er.lock.
func (er *entryReaderImpl) syncNamespaces(ctx context.Context) error {
	namespaces, err := er.namespaceStore.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Build set of current namespace IDs
	currentIDs := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		currentIDs[ns.ID] = struct{}{}

		// Skip already known namespaces
		if _, known := er.knownNS[ns.ID]; known {
			continue
		}

		// New namespace found — register it
		nsDir := filepath.Join(er.dagsDir, ns.ID)

		// Create directory if it doesn't exist
		if err := os.MkdirAll(nsDir, 0750); err != nil {
			logger.Error(ctx, "Failed to create namespace DAGs directory",
				tag.Error(err),
				slog.String("namespace", ns.Name),
				tag.Dir(nsDir),
			)
			continue
		}

		// Add to watcher
		if err := er.watcher.Add(nsDir); err != nil {
			logger.Error(ctx, "Failed to watch namespace directory",
				tag.Error(err),
				slog.String("namespace", ns.Name),
				tag.Dir(nsDir),
			)
			continue
		}

		// Register namespace
		er.knownNS[ns.ID] = ns.Name
		er.nsDirs[nsDir] = ns.ID

		// Load DAGs from this namespace directory
		er.loadNamespaceDAGs(ctx, ns.Name, ns.ID, nsDir)

		logger.Info(ctx, "Namespace directory registered",
			slog.String("namespace", ns.Name),
			slog.String("id", ns.ID),
			tag.Dir(nsDir),
		)
	}

	// Remove DAGs from deleted namespaces
	for nsID := range er.knownNS {
		if _, exists := currentIDs[nsID]; !exists {
			er.removeNamespace(ctx, nsID)
		}
	}

	return nil
}

// loadNamespaceDAGs loads all YAML DAG files from a namespace directory.
// The caller must hold er.lock.
func (er *entryReaderImpl) loadNamespaceDAGs(ctx context.Context, nsName, nsID, nsDir string) {
	fis, err := os.ReadDir(nsDir)
	if err != nil {
		logger.Error(ctx, "Failed to read namespace DAG directory",
			tag.Dir(nsDir),
			tag.Error(err),
		)
		return
	}

	var dags []string
	for _, fi := range fis {
		if !fileutil.IsYAMLFile(fi.Name()) {
			continue
		}
		filePath := filepath.Join(nsDir, fi.Name())
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
				tag.Name(fi.Name()),
				slog.String("namespace", nsName),
			)
			continue
		}
		dag.Namespace = nsName
		registryKey := nsID + "/" + fi.Name()
		er.registry[registryKey] = dag
		dags = append(dags, fi.Name())
	}

	logger.Debug(ctx, "Namespace DAGs loaded",
		slog.String("namespace", nsName),
		slog.String("dags", strings.Join(dags, ",")),
	)
}

// removeNamespace removes all DAGs for a deleted namespace from the registry.
// The caller must hold er.lock.
func (er *entryReaderImpl) removeNamespace(ctx context.Context, nsID string) {
	nsName := er.knownNS[nsID]
	prefix := nsID + "/"
	for key := range er.registry {
		if strings.HasPrefix(key, prefix) {
			delete(er.registry, key)
		}
	}

	// Remove from tracking maps
	nsDir := filepath.Join(er.dagsDir, nsID)
	delete(er.nsDirs, nsDir)
	delete(er.knownNS, nsID)

	logger.Info(ctx, "Namespace removed from scheduler",
		slog.String("namespace", nsName),
		slog.String("id", nsID),
	)
}

// handleFileEvent processes a file system event and updates the registry.
// The caller must hold er.lock.
func (er *entryReaderImpl) handleFileEvent(ctx context.Context, event fsnotify.Event) {
	// Determine which namespace this file belongs to
	eventDir := filepath.Dir(event.Name)
	nsID, ok := er.nsDirs[eventDir]
	if !ok {
		logger.Debug(ctx, "File event from unknown directory",
			tag.File(event.Name),
		)
		return
	}

	nsName := er.knownNS[nsID]
	fileName := filepath.Base(event.Name)
	registryKey := nsID + "/" + fileName

	switch event.Op {
	case fsnotify.Create, fsnotify.Write:
		filePath := filepath.Join(eventDir, fileName)
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
				tag.File(event.Name),
				slog.String("namespace", nsName),
			)
		} else {
			dag.Namespace = nsName
			er.registry[registryKey] = dag
			logger.Info(ctx, "DAG added/updated",
				tag.Name(fileName),
				slog.String("namespace", nsName),
			)
		}
	case fsnotify.Rename, fsnotify.Remove:
		delete(er.registry, registryKey)
		logger.Info(ctx, "DAG removed",
			tag.Name(fileName),
			slog.String("namespace", nsName),
		)
	default:
		// Ignore other events (e.g., Chmod)
	}
}
