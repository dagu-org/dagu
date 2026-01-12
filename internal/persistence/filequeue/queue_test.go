package filequeue_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")
	queue := filequeue.NewDualQueue(queueDir, "test-name")

	// Check if the queue is empty
	queueLen, err := queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 0, queueLen, "expected queue length to be 0")

	// Add a low priority job to the queue
	err = queue.Enqueue(th.Context, exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "low-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, exec.QueuePriorityHigh, exec.DAGRunRef{
		Name: "test-name",
		ID:   "high-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if the queue length is 2
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 2, queueLen, "expected queue length to be 2")

	// Check if pop returns the high priority job
	// Note: After dequeue, the file is removed, so we can't call Data() anymore.
	// We verify using the ID() method which contains the run-id in the filename.
	job, err := queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")
	require.Contains(t, job.ID(), "high-priority-dag-run", "expected job ID to contain 'high-priority-dag-run'")

	// Now the queue should have only one item left
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 1, queueLen, "expected queue length to be 1")

	// Check if pop returns the low priority job
	job, err = queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")
	require.Contains(t, job.ID(), "low-priority-dag-run", "expected job ID to contain 'low-priority-dag-run'")
}

func TestQueue_FindByDAGRunID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")
	queue := filequeue.NewDualQueue(queueDir, "test-name")

	// Add a low priority job to the queue
	err := queue.Enqueue(th.Context, exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "low-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, exec.QueuePriorityHigh, exec.DAGRunRef{
		Name: "test-name",
		ID:   "high-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if FindByDAGRunID returns the high priority job
	job, err := queue.FindByDAGRunID(th.Context, "high-priority-dag-run")
	require.NoError(t, err, "expected no error when finding job by dag-run ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "high-priority-dag-run", jobData.ID, "expected job ID to be 'high-priority-dag-run'")

	// Check if FindByDAGRunID returns the low priority job
	job, err = queue.FindByDAGRunID(th.Context, "low-priority-dag-run")
	require.NoError(t, err, "expected no error when finding job by dag-run ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData, err = job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "low-priority-dag-run", jobData.ID, "expected job ID to be 'low-priority-dag-run'")
}

func TestQueue_OrderingHighFrequency(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-ordering")
	queue := filequeue.NewDualQueue(queueDir, "test-ordering")

	// Enqueue items very quickly
	numItems := 10
	for i := 0; i < numItems; i++ {
		err := queue.Enqueue(th.Context, exec.QueuePriorityLow, exec.DAGRunRef{
			Name: "test-ordering",
			ID:   fmt.Sprintf("run-%d", i),
		})
		require.NoError(t, err)
	}

	// Dequeue and verify order
	// Note: After dequeue, the file is removed, so we can't call Data() anymore.
	// We verify using the ID() method which contains the run-id in the filename.
	for i := 0; i < numItems; i++ {
		item, err := queue.Dequeue(th.Context)
		require.NoError(t, err)
		require.Contains(t, item.ID(), fmt.Sprintf("run-%d", i), "expected items to be dequeued in order")
	}
}
