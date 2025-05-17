package prototype

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queue := NewQueue(th.Config.Paths.QueueDir, "test-name")

	// Check if the queue is empty
	queueLen, err := queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 0, queueLen, "expected queue length to be 0")

	// Add a low priority job to the queue
	err = queue.Enqueue(th.Context, models.QueuePriorityLow, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "low-priority-workflow",
	})

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, models.QueuePriorityHigh, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "high-priority-workflow",
	})

	// Check if the queue length is 2
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 2, queueLen, "expected queue length to be 2")

	// Check if pop returns the high priority job
	job, err := queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")

	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "high-priority-workflow", jobData.WorkflowID, "expected job ID to be 'high-priority-workflow'")

	// Now the queue should have only one item left
	queueLen, err = queue.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 1, queueLen, "expected queue length to be 1")

	// Check if pop returns the low priority job
	job, err = queue.Dequeue(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")
	require.NotNil(t, job, "expected job to be not nil")
	jobData, err = job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "low-priority-workflow", jobData.WorkflowID, "expected job ID to be 'low-priority-workflow'")
}

func TestQueue_FindByWorkflowID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queue := NewQueue(th.Config.Paths.QueueDir, "test-name")

	// Add a low priority job to the queue
	err := queue.Enqueue(th.Context, models.QueuePriorityLow, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "low-priority-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Add a high priority job to the queue
	err = queue.Enqueue(th.Context, models.QueuePriorityHigh, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "high-priority-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if FindByWorkflowID returns the high priority job
	job, err := queue.FindByWorkflowID(th.Context, "high-priority-workflow")
	require.NoError(t, err, "expected no error when finding job by workflow ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "high-priority-workflow", jobData.WorkflowID, "expected job ID to be 'high-priority-workflow'")

	// Check if FindByWorkflowID returns the low priority job
	job, err = queue.FindByWorkflowID(th.Context, "low-priority-workflow")
	require.NoError(t, err, "expected no error when finding job by workflow ID")
	require.NotNil(t, job, "expected job to be not nil")
	jobData, err = job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "low-priority-workflow", jobData.WorkflowID, "expected job ID to be 'low-priority-workflow'")
}
