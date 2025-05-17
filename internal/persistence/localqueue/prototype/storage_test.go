package prototype

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new storage
	storage := New(th.Config.Paths.QueueDir)

	// Check if the storage is empty
	length, err := storage.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting storage length")
	require.Equal(t, 0, length, "expected storage length to be 0")

	// Add a job to the storage
	err = storage.Enqueue(th.Context, "test-name", models.QueuePriorityLow, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to storage")

	// Check if the storage length is 1
	length, err = storage.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting storage length")
	require.Equal(t, 1, length, "expected storage length to be 1")

	// Check if other queue is empty
	length, err = storage.Len(th.Context, "other-name")
	require.NoError(t, err, "expected no error when getting storage length")
	require.Equal(t, 0, length, "expected storage length to be 0")

	// Check if dequeue returns the job
	job, err := storage.Dequeue(th.Context, "test-name")
	require.NoError(t, err, "expected no error when dequeueing job from storage")
	require.NotNil(t, job, "expected job to be not nil")
	require.Contains(t, job.ID(), "test-workflow", "expected job ID to contain 'test-workflow'")
	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")

	// Check if the queue is empty again
	length, err = storage.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting storage length")
	require.Equal(t, 0, length, "expected storage length to be 0")
}

func TestStorage_DequeueByWorkflowID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new storage
	storage := New(th.Config.Paths.QueueDir)

	// Add a job to the storage
	err := storage.Enqueue(th.Context, "test-name", models.QueuePriorityLow, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to storage")

	// Add another job to the storage
	err = storage.Enqueue(th.Context, "test-name", models.QueuePriorityLow, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow-2",
	})

	// Check if dequeue by workflow ID returns the job
	jobs, err := storage.DequeueByWorkflowID(th.Context, "test-workflow-2")
	require.NoError(t, err, "expected no error when dequeueing job by workflow ID from storage")
	require.Len(t, jobs, 1, "expected to dequeue one job")
	require.Contains(t, jobs[0].ID(), "test-workflow-2", "expected job ID to contain 'test-workflow-2'")
	jobData, err := jobs[0].Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-workflow-2", jobData.WorkflowID, "expected job ID to be 'test-workflow-2'")
}
