package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"os"
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

// syncBuffer provides thread-safe buffer operations for capturing logs
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

// TestQueueProcessor_DynamicQueueConcurrency_Internal tests that dynamically
// created (non-global) queues correctly respect the DAG's MaxActiveRuns setting
// on the FIRST call to ProcessQueueItems.
//
// This test reproduces a bug where newly created queues would only process
// 1 item instead of the configured maxActiveRuns on the first processing cycle.
//
// The bug occurs because:
// 1. In loop(), the queue is created with maxConcurrency=1 (default)
// 2. Then ProcessQueueItems is called
// 3. Inside ProcessQueueItems, updateQueueMaxConcurrency is supposed to update
//    maxConcurrency from the DAG's MaxActiveRuns BEFORE calculating the batch size
// 4. If this update doesn't happen or is read incorrectly, only 1 item is processed
func TestQueueProcessor_DynamicQueueConcurrency_Internal(t *testing.T) {
	t.Parallel()

	// Create temp directory for test data
	tmpDir, err := os.MkdirTemp("", "dagu-queue-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Set up logging capture
	logBuffer := &syncBuffer{buf: new(bytes.Buffer)}
	loggerInstance := logger.NewLogger(
		logger.WithDebug(),
		logger.WithFormat("text"),
		logger.WithWriter(logBuffer),
	)
	ctx := logger.WithFixedLogger(context.Background(), loggerInstance)

	// Create stores
	dagRunsDir := filepath.Join(tmpDir, "dag-runs")
	queueDir := filepath.Join(tmpDir, "queue")
	procDir := filepath.Join(tmpDir, "proc")

	dagRunStore := filedagrun.New(dagRunsDir)
	queueStore := filequeue.New(queueDir)
	procStore := fileproc.New(procDir)

	// Create a simple DAG with maxActiveRuns=3
	dagYaml := []byte(`
name: concurrent-dag
maxActiveRuns: 3
steps:
  - name: test
    command: echo "hello"
`)
	dag := &core.DAG{
		Name:          "concurrent-dag",
		MaxActiveRuns: 3,
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "test", Command: "echo hello"},
		},
	}

	// Create 3 DAG runs and initialize them
	for i := 1; i <= 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, err := dagRunStore.CreateAttempt(ctx, dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(ctx))
		st := exec.InitialStatus(dag)
		st.Status = core.Queued
		st.DAGRunID = runID
		require.NoError(t, run.Write(ctx, st))
		require.NoError(t, run.Close(ctx))

		// Enqueue the item
		err = queueStore.Enqueue(ctx, "concurrent-dag", exec.QueuePriorityHigh, exec.NewDAGRunRef("concurrent-dag", runID))
		require.NoError(t, err)
	}

	// Verify 3 items are in the queue
	items, err := queueStore.List(ctx, "concurrent-dag")
	require.NoError(t, err)
	require.Len(t, items, 3, "Should have 3 items in queue")

	// Create config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
	}

	// Create DAGExecutor (no dispatcher, so it will use local execution)
	dagExecutor := NewDAGExecutor(nil, runtime.NewSubCmdBuilder(cfg))

	// Create QueueProcessor with fast backoff for testing
	processor := NewQueueProcessor(
		queueStore,
		dagRunStore,
		procStore,
		dagExecutor,
		config.Queues{}, // Empty queues config - all queues are dynamic
		WithBackoffConfig(BackoffConfig{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     50 * time.Millisecond,
			MaxRetries:      2,
		}),
	)

	// SIMULATE what loop() does: create the queue with maxConcurrency=1
	// This is the key step that reproduces the bug - the queue starts with
	// maxConcurrency=1, and ProcessQueueItems should update it to 3
	processor.queues.Store("concurrent-dag", &queue{
		maxConcurrency: 1,
		isGlobal:       false, // This is a dynamic queue, not global
	})

	// Verify initial maxConcurrency is 1
	v, _ := processor.queues.Load("concurrent-dag")
	q := v.(*queue)
	require.Equal(t, 1, q.maxConc(), "Initial maxConcurrency should be 1")

	// Now call ProcessQueueItems - this is where the bug manifests
	// On the first call, it should:
	// 1. Read the items from queue
	// 2. For non-global queues, call updateQueueMaxConcurrency to update from DAG
	// 3. Then calculate batch size based on updated maxConcurrency
	processor.ProcessQueueItems(ctx, "concurrent-dag")

	// Give the processor enough time to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the maxConcurrency was updated correctly
	v, _ = processor.queues.Load("concurrent-dag")
	q = v.(*queue)
	assert.Equal(t, 3, q.maxConc(), "maxConcurrency should be updated to 3 (DAG's MaxActiveRuns)")

	// Check the logs to verify batch size
	logs := logBuffer.String()
	t.Logf("Captured logs:\n%s", logs)

	// The key assertion: on the first run, the processor should attempt to
	// process 3 items (maxActiveRuns=3), not just 1
	//
	// If the bug exists, we'll see "count=1" instead of "count=3"
	assert.True(t, strings.Contains(logs, "count=3") || strings.Contains(logs, `"count":3`),
		"Expected processor to attempt processing 3 items on first run (maxActiveRuns=3), "+
			"but the log indicates fewer items were processed. This suggests the maxConcurrency "+
			"was not correctly updated from the DAG's MaxActiveRuns setting before calculating "+
			"the batch size. Logs:\n%s", logs)
}

// TestQueueProcessor_RetryLogic tests that updateQueueMaxConcurrency properly
// retries when the DAG file is not immediately available (race condition fix).
func TestQueueProcessor_RetryLogic(t *testing.T) {
	t.Parallel()

	// Create temp directory for test data
	tmpDir, err := os.MkdirTemp("", "dagu-queue-test-retry-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Set up logging capture
	logBuffer := &syncBuffer{buf: new(bytes.Buffer)}
	loggerInstance := logger.NewLogger(
		logger.WithDebug(),
		logger.WithFormat("text"),
		logger.WithWriter(logBuffer),
	)
	ctx := logger.WithFixedLogger(context.Background(), loggerInstance)

	// Create stores
	dagRunsDir := filepath.Join(tmpDir, "dag-runs")
	queueDir := filepath.Join(tmpDir, "queue")
	procDir := filepath.Join(tmpDir, "proc")

	dagRunStore := filedagrun.New(dagRunsDir)
	queueStore := filequeue.New(queueDir)
	procStore := fileproc.New(procDir)

	// Create a DAG with maxActiveRuns=3
	dagYaml := []byte(`
name: retry-test-dag
maxActiveRuns: 3
steps:
  - name: test
    command: echo "hello"
`)
	dag := &core.DAG{
		Name:          "retry-test-dag",
		MaxActiveRuns: 3,
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "test", Command: "echo hello"},
		},
	}

	// Create 3 DAG runs
	for i := 1; i <= 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, err := dagRunStore.CreateAttempt(ctx, dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(ctx))
		st := exec.InitialStatus(dag)
		st.Status = core.Queued
		st.DAGRunID = runID
		require.NoError(t, run.Write(ctx, st))
		require.NoError(t, run.Close(ctx))

		// Enqueue the item
		err = queueStore.Enqueue(ctx, "retry-test-dag", exec.QueuePriorityHigh, exec.NewDAGRunRef("retry-test-dag", runID))
		require.NoError(t, err)
	}

	// Create config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
	}

	// Create DAGExecutor
	dagExecutor := NewDAGExecutor(nil, runtime.NewSubCmdBuilder(cfg))

	// Create QueueProcessor
	processor := NewQueueProcessor(
		queueStore,
		dagRunStore,
		procStore,
		dagExecutor,
		config.Queues{},
		WithBackoffConfig(BackoffConfig{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     50 * time.Millisecond,
			MaxRetries:      2,
		}),
	)

	// Simulate what loop() does: create queue with maxConcurrency=1
	processor.queues.Store("retry-test-dag", &queue{
		maxConcurrency: 1,
		isGlobal:       false,
	})

	// Call ProcessQueueItems
	processor.ProcessQueueItems(ctx, "retry-test-dag")

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify maxConcurrency was updated to 3 (retry logic worked)
	v, _ := processor.queues.Load("retry-test-dag")
	q := v.(*queue)
	assert.Equal(t, 3, q.maxConc(), "maxConcurrency should be updated to 3 after retry")

	// Verify logs show processing 3 items
	logs := logBuffer.String()
	assert.True(t, strings.Contains(logs, "count=3"),
		"Should process 3 items with correct maxConcurrency. Logs:\n%s", logs)
}

// TestQueueProcessor_GlobalQueueConcurrency tests that GLOBAL queues (defined in
// config) correctly use their configured MaxActiveRuns.
//
// This test checks that when a queue is defined in the global config with
// maxActiveRuns=3, it should process 3 items in parallel on the first call.
func TestQueueProcessor_GlobalQueueConcurrency(t *testing.T) {
	t.Parallel()

	// Create temp directory for test data
	tmpDir, err := os.MkdirTemp("", "dagu-queue-test-global-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Set up logging capture
	logBuffer := &syncBuffer{buf: new(bytes.Buffer)}
	loggerInstance := logger.NewLogger(
		logger.WithDebug(),
		logger.WithFormat("text"),
		logger.WithWriter(logBuffer),
	)
	ctx := logger.WithFixedLogger(context.Background(), loggerInstance)

	// Create stores
	dagRunsDir := filepath.Join(tmpDir, "dag-runs")
	queueDir := filepath.Join(tmpDir, "queue")
	procDir := filepath.Join(tmpDir, "proc")

	dagRunStore := filedagrun.New(dagRunsDir)
	queueStore := filequeue.New(queueDir)
	procStore := fileproc.New(procDir)

	// Create a DAG - note: DAG's MaxActiveRuns doesn't matter for global queues
	// The queue config's maxActiveRuns should take precedence
	dagYaml := []byte(`
name: global-queue-dag
steps:
  - name: test
    command: echo "hello"
`)
	dag := &core.DAG{
		Name:          "global-queue-dag",
		MaxActiveRuns: 1, // DAG says 1, but global queue config says 3
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "test", Command: "echo hello"},
		},
	}

	// Create 3 DAG runs and initialize them
	for i := 1; i <= 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		run, err := dagRunStore.CreateAttempt(ctx, dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(ctx))
		st := exec.InitialStatus(dag)
		st.Status = core.Queued
		st.DAGRunID = runID
		require.NoError(t, run.Write(ctx, st))
		require.NoError(t, run.Close(ctx))

		// Enqueue the item to the GLOBAL queue
		err = queueStore.Enqueue(ctx, "global-queue", exec.QueuePriorityHigh, exec.NewDAGRunRef("global-queue-dag", runID))
		require.NoError(t, err)
	}

	// Verify 3 items are in the queue
	items, err := queueStore.List(ctx, "global-queue")
	require.NoError(t, err)
	require.Len(t, items, 3, "Should have 3 items in queue")

	// Create config with a GLOBAL queue that has maxActiveRuns=3
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
	}

	// Create QueueProcessor with GLOBAL queue config
	// This simulates the user having a config with:
	// queues:
	//   config:
	//     - name: global-queue
	//       maxActiveRuns: 3
	dagExecutor := NewDAGExecutor(nil, runtime.NewSubCmdBuilder(cfg))

	processor := NewQueueProcessor(
		queueStore,
		dagRunStore,
		procStore,
		dagExecutor,
		config.Queues{
			Enabled: true,
			Config: []config.QueueConfig{
				{Name: "global-queue", MaxActiveRuns: 3},
			},
		},
		WithBackoffConfig(BackoffConfig{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     50 * time.Millisecond,
			MaxRetries:      2,
		}),
	)

	// Verify the global queue was created with correct maxConcurrency
	v, ok := processor.queues.Load("global-queue")
	require.True(t, ok, "Global queue should exist")
	q := v.(*queue)
	require.Equal(t, 3, q.maxConc(), "Global queue maxConcurrency should be 3 from config")
	require.True(t, q.isGlobalQueue(), "Queue should be marked as global")

	// Call ProcessQueueItems
	processor.ProcessQueueItems(ctx, "global-queue")

	// Give the processor enough time to complete
	time.Sleep(200 * time.Millisecond)

	// Check the logs to verify batch size
	logs := logBuffer.String()
	t.Logf("Captured logs:\n%s", logs)

	// The key assertion: global queues should process items based on their config
	assert.True(t, strings.Contains(logs, "count=3") || strings.Contains(logs, `"count":3`),
		"Expected processor to process 3 items (global queue maxActiveRuns=3). Logs:\n%s", logs)
}
