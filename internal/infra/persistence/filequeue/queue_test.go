package filequeue_test

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/infra/persistence/filequeue"
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
	err = queue.Enqueue(th.Context, execution.QueuePriorityLow, core.DAGRunRef{
		Name: "test-name",
		ID:   "low-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, execution.QueuePriorityHigh, core.DAGRunRef{
		Name: "test-name",
		ID:   "high-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if the queue length is 2
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 2, queueLen, "expected queue length to be 2")

	// Check if pop returns the high priority job
	job, err := queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")

	jobData := job.Data()
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "high-priority-dag-run", jobData.ID, "expected job ID to be 'high-priority-dag-run'")

	// Now the queue should have only one item left
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 1, queueLen, "expected queue length to be 1")

	// Check if pop returns the low priority job
	job, err = queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")
	jobData = job.Data()
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "low-priority-dag-run", jobData.ID, "expected job ID to be 'low-priority-dag-run'")
}

func TestQueue_FindByDAGRunID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")
	queue := filequeue.NewDualQueue(queueDir, "test-name")

	// Add a low priority job to the queue
	err := queue.Enqueue(th.Context, execution.QueuePriorityLow, core.DAGRunRef{
		Name: "test-name",
		ID:   "low-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, execution.QueuePriorityHigh, core.DAGRunRef{
		Name: "test-name",
		ID:   "high-priority-dag-run",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if FindByDAGRunID returns the high priority job
	job, err := queue.FindByDAGRunID(th.Context, "high-priority-dag-run")
	require.NoError(t, err, "expected no error when finding job by dag-run ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData := job.Data()
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "high-priority-dag-run", jobData.ID, "expected job ID to be 'high-priority-dag-run'")

	// Check if FindByDAGRunID returns the low priority job
	job, err = queue.FindByDAGRunID(th.Context, "low-priority-dag-run")
	require.NoError(t, err, "expected no error when finding job by dag-run ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData = job.Data()
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "low-priority-dag-run", jobData.ID, "expected job ID to be 'low-priority-dag-run'")
}
