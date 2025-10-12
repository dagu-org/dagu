package test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/filedag"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/persistence/fileproc"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
)

// Scheduler represents a test scheduler instance
type Scheduler struct {
	Helper
	EntryReader    scheduler.EntryReader
	QueueStore     models.QueueStore
	CoordinatorCli core.Dispatcher
}

// SetupScheduler creates a test scheduler instance with all dependencies
func SetupScheduler(t *testing.T, opts ...HelperOption) *Scheduler {
	t.Helper()

	// Create scheduler-specific options
	schedulerOpts := make([]HelperOption, 0, len(opts)+1)

	// Set up a test DAGs directory if not already specified
	var hasDAGsDir bool
	for _, opt := range opts {
		schedulerOpts = append(schedulerOpts, opt)
		// Check if DAGsDir option is already provided
		// This is a simple check, in production code you might want a more robust solution
		if opt != nil {
			hasDAGsDir = true
		}
	}

	// If no DAGsDir specified, use the testdata scheduler directory
	if !hasDAGsDir {
		testdataDir := TestdataPath(t, filepath.Join("scheduler"))
		schedulerOpts = append(schedulerOpts, WithDAGsDir(testdataDir))
	}

	// Create the base helper
	helper := Setup(t, schedulerOpts...)

	// Update config for scheduler-specific settings
	helper.Config.Scheduler.LockStaleThreshold = 30 * time.Second
	helper.Config.Scheduler.LockRetryInterval = 50 * time.Millisecond

	// Create additional stores needed for scheduler
	ds := filedag.New(helper.Config.Paths.DAGsDir, filedag.WithFlagsBaseDir(helper.Config.Paths.SuspendFlagsDir), filedag.WithSkipExamples(true))
	drs := filedagrun.New(helper.Config.Paths.DAGRunsDir)
	ps := fileproc.New(helper.Config.Paths.ProcDir)
	qs := filequeue.New(helper.Config.Paths.QueueDir)

	// Create DAG run manager
	drm := dagrun.New(drs, ps, helper.Config)

	// Create entry reader
	coordinatorCli := coordinator.New(helper.ServiceRegistry, coordinator.DefaultConfig())
	de := scheduler.NewDAGExecutor(coordinatorCli, dagrun.NewSubCmdBuilder(helper.Config))
	em := scheduler.NewEntryReader(helper.Config.Paths.DAGsDir, ds, drm, de, "")

	// Update helper with scheduler-specific stores
	helper.DAGStore = ds
	helper.DAGRunStore = drs
	helper.ProcStore = ps
	helper.DAGRunMgr = drm

	sch := &Scheduler{
		Helper:         helper,
		EntryReader:    em,
		QueueStore:     qs,
		CoordinatorCli: coordinatorCli,
	}

	return sch
}

// NewSchedulerInstance creates a new scheduler instance for testing
func (s *Scheduler) NewSchedulerInstance(t *testing.T) (*scheduler.Scheduler, error) {
	t.Helper()

	return scheduler.New(
		s.Config,
		s.EntryReader,
		s.DAGRunMgr,
		s.DAGRunStore,
		s.QueueStore,
		s.ProcStore,
		s.ServiceRegistry,
		s.CoordinatorCli,
	)
}

// Start starts the scheduler instance
func (s *Scheduler) Start(t *testing.T, ctx context.Context) (*scheduler.Scheduler, chan error) {
	t.Helper()

	instance, err := s.NewSchedulerInstance(t)
	require.NoError(t, err, "failed to create scheduler instance")

	errCh := make(chan error, 1)
	go func() {
		errCh <- instance.Start(ctx)
	}()

	// Give scheduler time to start
	time.Sleep(100 * time.Millisecond)

	return instance, errCh
}

// StartAsync starts the scheduler instance asynchronously
func (s *Scheduler) StartAsync(t *testing.T) (*scheduler.Scheduler, chan error) {
	return s.Start(t, s.Context)
}

// WithSchedulerTestDAGs creates a scheduler option for setting up test DAGs directory
func WithSchedulerTestDAGs(dagsDir string) HelperOption {
	return WithDAGsDir(dagsDir)
}
