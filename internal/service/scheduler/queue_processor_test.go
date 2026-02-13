package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syncBuffer struct {
	buf  *bytes.Buffer
	lock sync.Mutex
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.String()
}

type queueFixture struct {
	t           *testing.T
	ctx         context.Context
	logBuffer   *syncBuffer
	dagRunStore exec.DAGRunStore
	queueStore  exec.QueueStore
	procStore   exec.ProcStore
	processor   *QueueProcessor
	dag         *core.DAG
}

func newQueueFixture(t *testing.T) *queueFixture {
	t.Helper()
	t.Parallel()

	tmpDir := t.TempDir()
	logBuffer := &syncBuffer{buf: new(bytes.Buffer)}
	ctx := logger.WithFixedLogger(context.Background(), logger.NewLogger(
		logger.WithDebug(), logger.WithFormat("text"), logger.WithWriter(logBuffer),
	))

	return &queueFixture{
		t: t, ctx: ctx, logBuffer: logBuffer,
		dagRunStore: filedagrun.New(filepath.Join(tmpDir, "dag-runs")),
		queueStore:  filequeue.New(filepath.Join(tmpDir, "queue")),
		procStore:   fileproc.New(filepath.Join(tmpDir, "proc")),
	}
}

func (f *queueFixture) withDAG(name string, maxActiveRuns int) *queueFixture {
	f.dag = &core.DAG{
		Name: name, MaxActiveRuns: maxActiveRuns,
		YamlData: fmt.Appendf(nil, "name: %s\nmaxActiveRuns: %d\nsteps:\n  - name: test\n    command: echo hello", name, maxActiveRuns),
		Steps:    []core.Step{{Name: "test", Command: "echo hello"}},
	}
	return f
}

func (f *queueFixture) enqueueRuns(n int) *queueFixture {
	for i := 1; i <= n; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, err := f.dagRunStore.CreateAttempt(f.ctx, f.dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(f.t, err)
		require.NoError(f.t, run.Open(f.ctx))
		st := exec.InitialStatus(f.dag)
		st.Status, st.DAGRunID = core.Queued, runID
		require.NoError(f.t, run.Write(f.ctx, st))
		require.NoError(f.t, run.Close(f.ctx))
		require.NoError(f.t, f.queueStore.Enqueue(f.ctx, f.dag.Name, exec.QueuePriorityHigh, exec.NewDAGRunRef(f.dag.Name, runID)))
	}
	return f
}

func (f *queueFixture) withProcessor(cfg config.Queues) *queueFixture {
	f.processor = NewQueueProcessor(f.queueStore, f.dagRunStore, f.procStore,
		NewDAGExecutor(nil, runtime.NewSubCmdBuilder(&config.Config{Paths: config.PathsConfig{Executable: "/usr/bin/dagu"}}), config.ExecutionModeLocal),
		cfg, WithBackoffConfig(BackoffConfig{InitialInterval: 10 * time.Millisecond, MaxInterval: 50 * time.Millisecond, MaxRetries: 2}),
	)
	return f
}

func (f *queueFixture) simulateQueue(maxConcurrency int, isGlobal bool) *queueFixture {
	f.processor.queues.Store(f.dag.Name, &queue{maxConcurrency: maxConcurrency, isGlobal: isGlobal})
	return f
}

func (f *queueFixture) logs() string { return f.logBuffer.String() }

func (f *queueFixture) getQueue(name string) *queue {
	v, ok := f.processor.queues.Load(name)
	require.True(f.t, ok, "Queue %s should exist", name)
	return v.(*queue)
}

func (f *queueFixture) enqueueWithPriority(runID string, priority exec.QueuePriority) {
	f.enqueueToQueue(f.dag.Name, runID, priority)
}

func (f *queueFixture) enqueueToQueue(queueName, runID string, priority exec.QueuePriority) {
	run, err := f.dagRunStore.CreateAttempt(f.ctx, f.dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(f.t, err)
	require.NoError(f.t, run.Open(f.ctx))
	st := exec.InitialStatus(f.dag)
	st.Status, st.DAGRunID = core.Queued, runID
	require.NoError(f.t, run.Write(f.ctx, st))
	require.NoError(f.t, run.Close(f.ctx))
	require.NoError(f.t, f.queueStore.Enqueue(f.ctx, queueName, priority, exec.NewDAGRunRef(f.dag.Name, runID)))
}

func TestQueueProcessor_LocalQueueAlwaysFIFO(t *testing.T) {
	// Local queue should always use maxConcurrency=1, ignoring DAG's maxActiveRuns=5
	f := newQueueFixture(t).withDAG("local-dag", 5).enqueueRuns(3).
		withProcessor(config.Queues{}).simulateQueue(1, false)

	// Verify initial maxConcurrency is 1
	require.Equal(t, 1, f.getQueue("local-dag").getMaxConcurrency())

	f.processor.ProcessQueueItems(f.ctx, "local-dag")
	time.Sleep(200 * time.Millisecond)

	// Verify maxConcurrency is STILL 1 (not updated to DAG's 5)
	assert.Equal(t, 1, f.getQueue("local-dag").getMaxConcurrency(), "Local queue should always have maxConcurrency=1")
}

func TestQueueProcessor_GlobalQueue(t *testing.T) {
	f := newQueueFixture(t).withDAG("global-dag", 1).withProcessor(config.Queues{
		Enabled: true, Config: []config.QueueConfig{{Name: "global-queue", MaxActiveRuns: 3}},
	})

	for i := 1; i <= 3; i++ {
		f.enqueueToQueue("global-queue", fmt.Sprintf("run-%d", i), exec.QueuePriorityHigh)
	}

	require.Equal(t, 3, f.getQueue("global-queue").getMaxConcurrency())

	f.processor.ProcessQueueItems(f.ctx, "global-queue")
	time.Sleep(200 * time.Millisecond)
	assert.Contains(t, f.logs(), "count=3")
}

func TestQueueProcessor_ItemsRemainOnFailure(t *testing.T) {
	f := newQueueFixture(t).withDAG("fifo-dag", 1).enqueueRuns(2).
		withProcessor(config.Queues{Enabled: true, Config: []config.QueueConfig{{Name: "fifo-dag", MaxActiveRuns: 1}}}).
		simulateQueue(1, false)

	f.processor.ProcessQueueItems(f.ctx, "fifo-dag")
	time.Sleep(150 * time.Millisecond)

	items, err := f.queueStore.List(f.ctx, "fifo-dag")
	require.NoError(t, err)
	require.Len(t, items, 2, "Both items should still be in queue")
}

func TestQueueProcessor_PriorityOrdering(t *testing.T) {
	f := newQueueFixture(t).withDAG("priority-dag", 1).withProcessor(config.Queues{})

	// Enqueue low priority first, then high priority
	f.enqueueWithPriority("low-1", exec.QueuePriorityLow)
	f.enqueueWithPriority("low-2", exec.QueuePriorityLow)
	f.enqueueWithPriority("high-1", exec.QueuePriorityHigh)
	f.enqueueWithPriority("high-2", exec.QueuePriorityHigh)

	// Dequeue should return high priority first, then low priority
	expectedOrder := []string{"high-1", "high-2", "low-1", "low-2"}
	for _, expectedID := range expectedOrder {
		item, err := f.queueStore.DequeueByName(f.ctx, f.dag.Name)
		require.NoError(t, err)
		ref, err := item.Data()
		require.NoError(t, err)
		assert.Equal(t, expectedID, ref.ID)
	}
}

func TestQueueProcessor_ConcurrencyLimit(t *testing.T) {
	f := newQueueFixture(t).withDAG("conc-dag", 1).enqueueRuns(3).
		withProcessor(config.Queues{}).simulateQueue(1, false)

	// Process with maxConcurrency=1
	f.processor.ProcessQueueItems(f.ctx, "conc-dag")
	time.Sleep(100 * time.Millisecond)

	// Should only process 1 item at a time, leaving 2 in queue
	items, err := f.queueStore.List(f.ctx, "conc-dag")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 2, "Concurrency limit should prevent processing all at once")
}

func TestQueueProcessor_GlobalQueueIgnoresDAGMaxActiveRuns(t *testing.T) {
	// Global queue config: MaxActiveRuns=5
	// DAG config: maxActiveRuns=1
	// Expected: Global queue should use 5, NOT be overwritten by DAG's 1
	f := newQueueFixture(t).withDAG("dag-with-low-concurrency", 1).withProcessor(config.Queues{
		Enabled: true, Config: []config.QueueConfig{{Name: "global-queue", MaxActiveRuns: 5}},
	})

	// Enqueue 5 items to the global queue
	for i := 1; i <= 5; i++ {
		f.enqueueToQueue("global-queue", fmt.Sprintf("run-%d", i), exec.QueuePriorityHigh)
	}

	// Verify initial maxConcurrency is 5 (from global config)
	require.Equal(t, 5, f.getQueue("global-queue").getMaxConcurrency(), "Global queue should have maxConcurrency=5 from config")

	// Process items
	f.processor.ProcessQueueItems(f.ctx, "global-queue")
	time.Sleep(200 * time.Millisecond)

	// Verify maxConcurrency is STILL 5 (not overwritten by DAG's maxActiveRuns=1)
	assert.Equal(t, 5, f.getQueue("global-queue").getMaxConcurrency(), "Global queue maxConcurrency should NOT be overwritten by DAG")

	// Verify all 5 items were processed in the batch (not just 1)
	assert.Contains(t, f.logs(), "count=5", "Should process 5 items, not 1")
}

func TestQueueProcessor_GlobalQueueViaLoop(t *testing.T) {
	// This test mimics the real scheduler flow where ProcessQueueItems
	// is called via the loop, not directly.
	f := newQueueFixture(t).withDAG("loop-dag", 1).withProcessor(config.Queues{
		Enabled: true, Config: []config.QueueConfig{{Name: "global-queue", MaxActiveRuns: 3}},
	})

	// Enqueue 3 items BEFORE calling process (mimics real scenario)
	for i := 1; i <= 3; i++ {
		f.enqueueToQueue("global-queue", fmt.Sprintf("run-%d", i), exec.QueuePriorityHigh)
	}

	// Verify queue list returns the global queue
	queueList, err := f.queueStore.QueueList(f.ctx)
	require.NoError(t, err)
	require.Contains(t, queueList, "global-queue")

	// Simulate what loop() does: check if queue exists in p.queues
	q := f.getQueue("global-queue")
	require.Equal(t, 3, q.getMaxConcurrency(), "Global queue should have maxConcurrency=3")
	require.True(t, q.isGlobalQueue(), "Should be marked as global queue")

	// Process
	f.processor.ProcessQueueItems(f.ctx, "global-queue")
	time.Sleep(200 * time.Millisecond)

	// Verify all 3 items processed
	t.Logf("Logs: %s", f.logs())
	assert.Contains(t, f.logs(), "count=3", "Should process all 3 items")
	assert.Contains(t, f.logs(), "max-concurrency=3", "maxConcurrency should be 3")
}
