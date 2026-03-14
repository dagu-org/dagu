// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package queue_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fixture provides setup for queue integration tests.
type fixture struct {
	t            *testing.T
	th           test.Command
	dag          *core.DAG
	queue        string
	runIDs       []string
	schedDone    chan error
	cancel       context.CancelFunc
	globalQueues []config.QueueConfig
	retryWindow  time.Duration
}

// newFixture creates a new queue integration test fixture.
func newFixture(t *testing.T, dagYAML string, opts ...func(*fixture)) *fixture {
	t.Helper()
	t.Parallel()

	f := &fixture{t: t, schedDone: make(chan error, 1)}

	// Apply options to collect configuration
	for _, opt := range opts {
		opt(f)
	}

	// Setup test helper with queue config
	helperOpts := []test.HelperOption{
		test.WithConfigMutator(func(c *config.Config) {
			c.Queues.Enabled = true
			if len(f.globalQueues) > 0 {
				c.Queues.Config = f.globalQueues
			}
			if f.retryWindow > 0 {
				c.Scheduler.RetryFailureWindow = f.retryWindow
			}
			// Disable scheduler health server (port 8090) to avoid "address already in use"
			// when multiple tests run in parallel
			c.Scheduler.Port = 0
		}),
	}
	f.th = test.SetupCommand(t, helperOpts...)

	// Create DAG file
	require.NoError(t, os.MkdirAll(f.th.Config.Paths.DAGsDir, 0755))
	dagFile := filepath.Join(f.th.Config.Paths.DAGsDir, "test.yaml")
	require.NoError(t, os.WriteFile(dagFile, []byte(dagYAML), 0644))

	dag, err := spec.Load(f.th.Context, dagFile)
	require.NoError(t, err)
	f.dag = dag
	if f.queue == "" {
		f.queue = dag.ProcGroup()
	}

	t.Cleanup(f.cleanup)
	return f
}

// WithQueue sets a custom queue name.
func WithQueue(name string) func(*fixture) {
	return func(f *fixture) { f.queue = name }
}

// WithGlobalQueue adds a global queue configuration.
func WithGlobalQueue(name string, maxActiveRuns int) func(*fixture) {
	return func(f *fixture) {
		f.globalQueues = append(f.globalQueues, config.QueueConfig{
			Name:          name,
			MaxActiveRuns: maxActiveRuns,
		})
	}
}

// WithRetryWindow overrides scheduler.retry_failure_window for the fixture.
func WithRetryWindow(window time.Duration) func(*fixture) {
	return func(f *fixture) { f.retryWindow = window }
}

// Enqueue adds n DAG runs to the queue.
func (f *fixture) Enqueue(n int) *fixture {
	f.runIDs = make([]string, n)
	for i := range n {
		f.runIDs[i] = f.enqueueOne()
	}
	return f
}

func (f *fixture) enqueueOne() string {
	return f.enqueueWithPriority(exec.QueuePriorityLow)
}

func (f *fixture) enqueueWithPriority(priority exec.QueuePriority) string {
	id := uuid.New().String()
	att, err := f.th.DAGRunStore.CreateAttempt(f.th.Context, f.dag, time.Now(), id, exec.NewDAGRunAttemptOptions{})
	require.NoError(f.t, err)
	logFile := filepath.Join(f.th.Config.Paths.LogDir, f.dag.Name, id+".log")
	require.NoError(f.t, os.MkdirAll(filepath.Dir(logFile), 0755))
	st := transform.NewStatusBuilder(f.dag).Create(id, core.Queued, 0, time.Time{},
		transform.WithLogFilePath(logFile),
		transform.WithAttemptID(att.ID()),
		transform.WithHierarchyRefs(exec.NewDAGRunRef(f.dag.Name, id), exec.DAGRunRef{}),
	)
	require.NoError(f.t, att.Open(f.th.Context))
	require.NoError(f.t, att.Write(f.th.Context, st))
	require.NoError(f.t, att.Close(f.th.Context))
	require.NoError(f.t, f.th.QueueStore.Enqueue(f.th.Context, f.queue, priority, exec.NewDAGRunRef(f.dag.Name, id)))
	return id
}

// EnqueueWithPriority adds a single DAG run with specified priority.
func (f *fixture) EnqueueWithPriority(priority exec.QueuePriority) *fixture {
	f.runIDs = append(f.runIDs, f.enqueueWithPriority(priority))
	return f
}

// StartScheduler starts the scheduler in background.
func (f *fixture) StartScheduler(timeout time.Duration) *fixture {
	var ctx context.Context
	ctx, f.cancel = context.WithTimeout(f.th.Context, timeout)
	home := filepath.Dir(f.th.Config.Paths.DAGsDir)
	go func() {
		th := f.th
		th.Context = ctx
		f.schedDone <- th.RunCommandWithError(f.t, cmd.Scheduler(), test.CmdTest{
			Args:        []string{"scheduler", "--dagu-home", home},
			ExpectedOut: []string{"Scheduler started"},
		})
	}()
	return f
}

// WaitDrain waits for the queue to empty.
func (f *fixture) WaitDrain(timeout time.Duration) *fixture {
	require.Eventually(f.t, func() bool {
		items, err := f.th.QueueStore.List(f.th.Context, f.queue)
		require.NoError(f.t, err)
		f.t.Logf("Queue %s: %d remaining", f.queue, len(items))
		return len(items) == 0
	}, timeout, 200*time.Millisecond)
	return f
}

// Stop stops the scheduler.
func (f *fixture) Stop() {
	if f.cancel != nil {
		f.cancel()
	}
	f.th.Cancel()
	select {
	case err := <-f.schedDone:
		require.NoError(f.t, err)
	case <-time.After(5 * time.Second):
	}
}

// AssertConcurrent verifies all DAGs started within maxDiff of each other.
func (f *fixture) AssertConcurrent(maxDiff time.Duration) {
	times := f.collectStartTimes()
	var max time.Duration
	for i := range times {
		for j := i + 1; j < len(times); j++ {
			if d := times[i].Sub(times[j]).Abs(); d > max {
				max = d
			}
		}
	}
	f.t.Logf("Start times: %v, max diff: %v", times, max)
	require.LessOrEqual(f.t, max, maxDiff)
}

func (f *fixture) collectStartTimes() []time.Time {
	var times []time.Time
	for _, id := range f.runIDs {
		st := f.Status(id)
		t, err := stringutil.ParseTime(st.StartedAt)
		require.NoError(f.t, err)
		times = append(times, t)
	}
	return times
}

type runStatusOptions struct {
	RunID        string
	StartedAt    time.Time
	FinishedAt   time.Time
	QueuedAt     time.Time
	ScheduleTime time.Time
	RetryCount   int
	TriggerType  core.TriggerType
}

func (f *fixture) writeRunStatus(status core.Status, opts runStatusOptions) string {
	runID := opts.RunID
	if runID == "" {
		runID = uuid.New().String()
	}

	att, err := f.th.DAGRunStore.CreateAttempt(f.th.Context, f.dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(f.t, err)
	logFile := filepath.Join(f.th.Config.Paths.LogDir, f.dag.Name, runID+".log")
	require.NoError(f.t, os.MkdirAll(filepath.Dir(logFile), 0755))

	startedAt := opts.StartedAt
	if startedAt.IsZero() && status.IsActive() {
		startedAt = time.Now()
	}

	statusOpts := []transform.StatusOption{
		transform.WithLogFilePath(logFile),
		transform.WithAttemptID(att.ID()),
		transform.WithHierarchyRefs(exec.NewDAGRunRef(f.dag.Name, runID), exec.DAGRunRef{}),
		transform.WithRetryCount(opts.RetryCount),
	}
	if !opts.FinishedAt.IsZero() {
		statusOpts = append(statusOpts, transform.WithFinishedAt(opts.FinishedAt))
	}
	if !opts.QueuedAt.IsZero() {
		statusOpts = append(statusOpts, transform.WithQueuedAt(exec.FormatTime(opts.QueuedAt)))
	}
	if !opts.ScheduleTime.IsZero() {
		statusOpts = append(statusOpts, transform.WithScheduleTime(exec.FormatTime(opts.ScheduleTime)))
	}
	if opts.TriggerType != core.TriggerTypeUnknown {
		statusOpts = append(statusOpts, transform.WithTriggerType(opts.TriggerType))
	}

	st := transform.NewStatusBuilder(f.dag).Create(runID, status, 0, startedAt, statusOpts...)
	require.NoError(f.t, att.Open(f.th.Context))
	require.NoError(f.t, att.Write(f.th.Context, st))
	require.NoError(f.t, att.Close(f.th.Context))
	return runID
}

// FailedRun creates a DAGRunAttempt with Failed status, simulating a completed but failed run.
func (f *fixture) FailedRun() *fixture {
	f.FailedRunWithMetadata(runStatusOptions{
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	})
	return f
}

// FailedRunWithMetadata creates a failed DAG run with explicit persisted metadata.
func (f *fixture) FailedRunWithMetadata(opts runStatusOptions) string {
	runID := f.writeRunStatus(core.Failed, opts)
	f.runIDs = append(f.runIDs, runID)
	return runID
}

// RunningRunWithMetadata creates a running DAG run with explicit persisted metadata.
func (f *fixture) RunningRunWithMetadata(opts runStatusOptions) string {
	return f.writeRunStatus(core.Running, opts)
}

// RetryEnqueue enqueues a previously failed run for retry using exec.EnqueueRetry.
func (f *fixture) RetryEnqueue(runID string) *fixture {
	require.NoError(f.t, exec.EnqueueRetry(f.th.Context, f.th.DAGRunStore, f.th.QueueStore, f.dag, f.Status(runID)))
	return f
}

// Status returns the latest persisted status for the given DAG run.
func (f *fixture) Status(runID string) *exec.DAGRunStatus {
	ref := exec.NewDAGRunRef(f.dag.Name, runID)
	att, err := f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(f.t, err)
	status, err := att.ReadStatus(f.th.Context)
	require.NoError(f.t, err)
	return status
}

func (f *fixture) cleanup() {
	if f.cancel != nil {
		f.cancel()
	}
}
