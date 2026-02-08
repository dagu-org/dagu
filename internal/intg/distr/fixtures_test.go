package distr_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagstate"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workerMode int

const (
	sharedNothingMode workerMode = iota
	sharedFSMode
)

type fixtureConfig struct {
	workerMode     workerMode
	workerCount    int
	workerLabels   map[string]string
	logPersistence bool
	dagsDir        string
}

type fixtureOption func(*fixtureConfig)

func withWorkerMode(mode workerMode) fixtureOption {
	return func(c *fixtureConfig) { c.workerMode = mode }
}

func withWorkerCount(n int) fixtureOption {
	return func(c *fixtureConfig) { c.workerCount = n }
}

func withLabels(labels map[string]string) fixtureOption {
	return func(c *fixtureConfig) { c.workerLabels = labels }
}

func withLogPersistence() fixtureOption {
	return func(c *fixtureConfig) { c.logPersistence = true }
}

func withDAGsDir(dir string) fixtureOption {
	return func(c *fixtureConfig) { c.dagsDir = dir }
}

type testFixture struct {
	t                 *testing.T
	coord             *test.Coordinator
	dagWrapper        *test.DAG
	coordinatorClient coordinator.Client
	workers           []*worker.Worker
	scheduler         *scheduler.Scheduler
	schedulerCancel   context.CancelFunc
	schedulerCtx      context.Context
}

func newTestFixture(t *testing.T, yaml string, opts ...fixtureOption) *testFixture {
	t.Helper()

	cfg := &fixtureConfig{
		workerMode:   sharedNothingMode,
		workerCount:  1,
		workerLabels: map[string]string{"test": "true"},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var coordOpts []test.HelperOption
	coordOpts = append(coordOpts, test.WithStatusPersistence())
	coordOpts = append(coordOpts, test.WithConfigMutator(func(c *config.Config) {
		c.Queues.Enabled = true
	}))
	if cfg.logPersistence {
		coordOpts = append(coordOpts, test.WithLogPersistence())
	}
	if cfg.dagsDir != "" {
		coordOpts = append(coordOpts, test.WithDAGsDir(cfg.dagsDir))
	}

	coord := test.SetupCoordinator(t, coordOpts...)
	coord.Config.Queues.Enabled = true

	f := &testFixture{
		t:                 t,
		coord:             coord,
		coordinatorClient: coord.GetCoordinatorClient(t),
	}

	for i := range cfg.workerCount {
		workerID := fmt.Sprintf("worker-%d", i+1)
		var w *worker.Worker
		switch cfg.workerMode {
		case sharedNothingMode:
			w = f.setupSharedNothingWorker(workerID, cfg.workerLabels)
		case sharedFSMode:
			w = f.setupSharedFSWorker(workerID, cfg.workerLabels)
		}
		f.workers = append(f.workers, w)
	}

	dag := coord.DAG(t, yaml)
	f.dagWrapper = &dag

	return f
}

func (f *testFixture) setupSharedNothingWorker(workerID string, labels map[string]string) *worker.Worker {
	f.t.Helper()

	handlerCfg := worker.RemoteTaskHandlerConfig{
		WorkerID:          workerID,
		CoordinatorClient: f.coordinatorClient,
		DAGRunStore:       nil,
		DAGStore:          f.coord.DAGStore,
		DAGRunMgr:         f.coord.DAGRunMgr,
		ServiceRegistry:   f.coord.ServiceRegistry,
		PeerConfig:        f.coord.Config.Core.Peer,
		Config:            f.coord.Config,
	}

	w := worker.NewWorker(workerID, 10, f.coordinatorClient, labels, f.coord.Config)
	w.SetHandler(worker.NewRemoteTaskHandler(handlerCfg))

	return f.startWorker(w, workerID)
}

func (f *testFixture) setupSharedFSWorker(workerID string, labels map[string]string) *worker.Worker {
	f.t.Helper()

	w := worker.NewWorker(workerID, 10, f.coordinatorClient, labels, f.coord.Config)
	w.SetHandler(worker.NewTaskHandler(f.coord.Config))

	return f.startWorker(w, workerID)
}

func (f *testFixture) startWorker(w *worker.Worker, workerID string) *worker.Worker {
	f.t.Helper()

	go func() {
		if err := w.Start(f.coord.Context); err != nil {
			f.t.Logf("Worker stopped: %v", err)
		}
	}()

	f.waitForWorkerRegistration(workerID, 5*time.Second)

	f.t.Cleanup(func() {
		if err := w.Stop(f.coord.Context); err != nil {
			f.t.Logf("Error stopping worker: %v", err)
		}
	})

	return w
}

func (f *testFixture) waitForWorkerRegistration(workerID string, timeout time.Duration) {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		workers, err := f.coordinatorClient.GetWorkers(f.coord.Context)
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.WorkerId == workerID {
				return true
			}
		}
		return false
	}, timeout, 50*time.Millisecond, "worker %s should register with coordinator", workerID)
}

func (f *testFixture) startScheduler(timeout time.Duration) {
	f.t.Helper()

	de := scheduler.NewDAGExecutor(f.coordinatorClient, runtime.NewSubCmdBuilder(f.coord.Config), f.coord.Config.DefaultExecMode)
	em := scheduler.NewEntryReader(f.coord.Config.Paths.DAGsDir, f.coord.DAGStore, f.coord.DAGRunMgr, de, "")

	dss := filedagstate.New(f.coord.Config.Paths.DataDir, f.coord.Config.Paths.DAGsDir)
	schedulerInst, err := scheduler.New(
		f.coord.Config,
		em,
		f.coord.DAGRunMgr,
		f.coord.DAGRunStore,
		f.coord.QueueStore,
		f.coord.ProcStore,
		f.coord.ServiceRegistry,
		f.coordinatorClient,
		dss,
	)
	require.NoError(f.t, err)

	f.scheduler = schedulerInst
	f.schedulerCtx, f.schedulerCancel = context.WithTimeout(f.coord.Context, timeout)
	go func() { _ = f.scheduler.Start(f.schedulerCtx) }()
}

func (f *testFixture) enqueue() error {
	f.t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(f.coord.Config)
	enqueueSpec := subCmdBuilder.Enqueue(f.dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
	return runtime.Run(f.coord.Context, enqueueSpec)
}

func (f *testFixture) start() error {
	f.t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(f.coord.Config)
	startSpec := subCmdBuilder.Start(f.dagWrapper.DAG, runtime.StartOptions{Quiet: true})
	return runtime.Start(f.coord.Context, startSpec)
}

func (f *testFixture) retry(dagRunID string) error {
	f.t.Helper()
	subCmdBuilder := runtime.NewSubCmdBuilder(f.coord.Config)
	retrySpec := subCmdBuilder.Retry(f.dagWrapper.DAG, dagRunID, "")
	return runtime.Start(f.coord.Context, retrySpec)
}

func (f *testFixture) waitForQueued() {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		items, err := f.coord.QueueStore.ListByDAGName(f.coord.Context, f.dagWrapper.ProcGroup(), f.dagWrapper.Name)
		return err == nil && len(items) == 1
	}, 5*time.Second, 100*time.Millisecond, "DAG should be enqueued")
}

func (f *testFixture) waitForStatus(expected core.Status, timeout time.Duration) exec.DAGRunStatus {
	f.t.Helper()
	var status exec.DAGRunStatus
	require.Eventually(f.t, func() bool {
		var err error
		status, err = f.coord.DAGRunMgr.GetLatestStatus(f.coord.Context, f.dagWrapper.DAG)
		if err != nil {
			return false
		}
		return status.Status == expected
	}, timeout, 100*time.Millisecond, "timeout waiting for status %s", expected)
	return status
}

func (f *testFixture) waitForStatusIn(expected []core.Status, timeout time.Duration) exec.DAGRunStatus {
	f.t.Helper()
	var status exec.DAGRunStatus
	require.Eventually(f.t, func() bool {
		var err error
		status, err = f.coord.DAGRunMgr.GetLatestStatus(f.coord.Context, f.dagWrapper.DAG)
		if err != nil {
			return false
		}
		for _, exp := range expected {
			if status.Status == exp {
				return true
			}
		}
		return false
	}, timeout, 100*time.Millisecond, "timeout waiting for status in %v", expected)
	return status
}

func (f *testFixture) latestStatus() (exec.DAGRunStatus, error) {
	return f.coord.DAGRunMgr.GetLatestStatus(f.coord.Context, f.dagWrapper.DAG)
}

func (f *testFixture) stop(dagRunID string) error {
	return f.coord.DAGRunMgr.Stop(f.coord.Context, f.dagWrapper.DAG, dagRunID)
}

func (f *testFixture) cleanup() {
	f.t.Helper()
	if f.schedulerCancel != nil {
		f.schedulerCancel()
	}
}

func (f *testFixture) assertAllNodesSucceeded(status exec.DAGRunStatus) {
	f.t.Helper()
	for _, node := range status.Nodes {
		require.Equal(f.t, core.NodeSucceeded, node.Status, "step %s should have succeeded", node.Step.Name)
	}
}

func (f *testFixture) assertWorkerID(status exec.DAGRunStatus, expected string) {
	f.t.Helper()
	require.Equal(f.t, expected, status.WorkerID, "unexpected worker ID")
}

func (f *testFixture) logDir() string {
	return f.coord.LogDir()
}

func findLogFiles(t *testing.T, logDir, dagName, dagRunID, stepName, suffix string) []string {
	t.Helper()

	baseDir := filepath.Join(logDir, dagName, dagRunID)
	var matches []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			filename := fmt.Sprintf("%s.%s.log", stepName, suffix)
			if filepath.Base(path) == filename {
				matches = append(matches, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Logf("Error walking log directory: %v", err)
	}

	return matches
}

func assertLogContains(t *testing.T, logDir, dagName, dagRunID, stepName, expected string) {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)

	content, err := os.ReadFile(matches[0])
	require.NoError(t, err, "failed to read log file %s", matches[0])
	assert.Contains(t, string(content), expected, "log file should contain expected content")
}

func assertLogExists(t *testing.T, logDir, dagName, dagRunID, stepName string) string {
	t.Helper()

	matches := findLogFiles(t, logDir, dagName, dagRunID, stepName, "stdout")
	require.NotEmpty(t, matches, "no stdout log file found for step %s", stepName)
	return matches[0]
}

func getLogContent(t *testing.T, logPath string) string {
	t.Helper()
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "failed to read log file")
	return string(content)
}
