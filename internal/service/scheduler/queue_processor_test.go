package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
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
		YamlData: []byte(fmt.Sprintf("name: %s\nmaxActiveRuns: %d\nsteps:\n  - name: test\n    command: echo hello", name, maxActiveRuns)),
		Steps:    []core.Step{{Name: "test", Command: "echo hello"}},
	}
	return f
}

func (f *queueFixture) enqueueRuns(n int) *queueFixture {
	for i := 1; i <= n; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, _ := f.dagRunStore.CreateAttempt(f.ctx, f.dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		_ = run.Open(f.ctx)
		st := exec.InitialStatus(f.dag)
		st.Status, st.DAGRunID = core.Queued, runID
		_ = run.Write(f.ctx, st)
		_ = run.Close(f.ctx)
		_ = f.queueStore.Enqueue(f.ctx, f.dag.Name, exec.QueuePriorityHigh, exec.NewDAGRunRef(f.dag.Name, runID))
	}
	return f
}

func (f *queueFixture) withProcessor(cfg config.Queues) *queueFixture {
	f.processor = NewQueueProcessor(f.queueStore, f.dagRunStore, f.procStore,
		NewDAGExecutor(nil, runtime.NewSubCmdBuilder(&config.Config{Paths: config.PathsConfig{Executable: "/usr/bin/dagu"}})),
		cfg, WithBackoffConfig(BackoffConfig{InitialInterval: 10 * time.Millisecond, MaxInterval: 50 * time.Millisecond, MaxRetries: 2}),
	)
	return f
}

func (f *queueFixture) simulateQueue(maxConcurrency int, isGlobal bool) *queueFixture {
	f.processor.queues.Store(f.dag.Name, &queue{maxConcurrency: maxConcurrency, isGlobal: isGlobal})
	return f
}

func (f *queueFixture) logs() string { return f.logBuffer.String() }

func TestQueueProcessor_DynamicQueueConcurrency(t *testing.T) {
	f := newQueueFixture(t).withDAG("concurrent-dag", 3).enqueueRuns(3).withProcessor(config.Queues{}).simulateQueue(1, false)
	v, _ := f.processor.queues.Load("concurrent-dag")
	require.Equal(t, 1, v.(*queue).maxConc())

	f.processor.ProcessQueueItems(f.ctx, "concurrent-dag")
	time.Sleep(200 * time.Millisecond)

	v, _ = f.processor.queues.Load("concurrent-dag")
	assert.Equal(t, 3, v.(*queue).maxConc())
	assert.True(t, strings.Contains(f.logs(), "count=3"))
}

func TestQueueProcessor_RetryLogic(t *testing.T) {
	f := newQueueFixture(t).withDAG("retry-dag", 3).enqueueRuns(3).withProcessor(config.Queues{}).simulateQueue(1, false)

	f.processor.ProcessQueueItems(f.ctx, "retry-dag")
	time.Sleep(200 * time.Millisecond)

	v, _ := f.processor.queues.Load("retry-dag")
	assert.Equal(t, 3, v.(*queue).maxConc())
	assert.True(t, strings.Contains(f.logs(), "count=3"))
}

func TestQueueProcessor_GlobalQueue(t *testing.T) {
	f := newQueueFixture(t).withDAG("global-dag", 1).withProcessor(config.Queues{
		Enabled: true, Config: []config.QueueConfig{{Name: "global-queue", MaxActiveRuns: 3}},
	})

	for i := 1; i <= 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, _ := f.dagRunStore.CreateAttempt(f.ctx, f.dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		_ = run.Open(f.ctx)
		st := exec.InitialStatus(f.dag)
		st.Status, st.DAGRunID = core.Queued, runID
		_ = run.Write(f.ctx, st)
		_ = run.Close(f.ctx)
		_ = f.queueStore.Enqueue(f.ctx, "global-queue", exec.QueuePriorityHigh, exec.NewDAGRunRef(f.dag.Name, runID))
	}

	v, ok := f.processor.queues.Load("global-queue")
	require.True(t, ok)
	require.Equal(t, 3, v.(*queue).maxConc())

	f.processor.ProcessQueueItems(f.ctx, "global-queue")
	time.Sleep(200 * time.Millisecond)
	assert.True(t, strings.Contains(f.logs(), "count=3"))
}

func TestQueueProcessor_StrictFIFO(t *testing.T) {
	f := newQueueFixture(t).withDAG("fifo-dag", 1).enqueueRuns(2).
		withProcessor(config.Queues{Enabled: true, Config: []config.QueueConfig{{Name: "fifo-dag", MaxActiveRuns: 1}}}).
		simulateQueue(1, false)

	f.processor.ProcessQueueItems(f.ctx, "fifo-dag")
	time.Sleep(150 * time.Millisecond)

	items, _ := f.queueStore.List(f.ctx, "fifo-dag")
	require.Len(t, items, 2, "Both items should still be in queue")
}
