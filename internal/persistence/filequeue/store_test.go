package filequeue_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Check if the store is empty
	length, err := store.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting store length")
	require.Equal(t, 0, length, "expected store length to be 0")

	// Add a job to thestore
	err = store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Check if the store length is 1
	length, err = store.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting store length")
	require.Equal(t, 1, length, "expected store length to be 1")

	// Check if other queue is empty
	length, err = store.Len(th.Context, "other-name")
	require.NoError(t, err, "expected no error when getting store length")
	require.Equal(t, 0, length, "expected store length to be 0")

	// Check if dequeue returns the job
	// Note: After dequeue, the file is removed, so we can't call Data() anymore.
	// We use the embedded ItemData fields directly instead.
	job, err := store.DequeueByName(th.Context, "test-name")
	require.NoError(t, err, "expected no error when dequeueing job from store")
	require.NotNil(t, job, "expected job to be not nil")
	require.Contains(t, job.ID(), "test-dag", "expected job ID to contain 'test-dag'")

	// Check if the queue is empty again
	length, err = store.Len(th.Context, "test-name")
	require.NoError(t, err, "expected no error when getting store length")
	require.Equal(t, 0, length, "expected store length to be 0")
}

func TestStore_DequeueByDAGRunID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a newstore
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add a job to thestore
	err := store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Add another job to thestore
	err = store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Check if dequeue by dag-run ID returns the job
	// Note: After dequeue, the file is removed, so we can't call Data() anymore.
	// We verify using the ID() method which still works.
	jobs, err := store.DequeueByDAGRunID(th.Context, "test-name", exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when dequeueing job by dag-run ID from store")
	require.Len(t, jobs, 1, "expected to dequeue one job")
	require.Contains(t, jobs[0].ID(), "test-dag-2", "expected job ID to contain 'test-dag-2'")
}

func TestStore_List(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a newstore
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add a job to thestore
	err := store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Add another job to thestore
	err = store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Check if list returns the jobs
	jobs, err := store.List(th.Context, "test-name")
	require.NoError(t, err, "expected no error when listing jobs from store")
	require.Len(t, jobs, 2, "expected to list two jobs")
}

func TestStore_All(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a newstore
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add a job to thestore
	err := store.Enqueue(th.Context, "test-name", exec.QueuePriorityLow, exec.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Add another job to thestore
	err = store.Enqueue(th.Context, "test-name2", exec.QueuePriorityHigh, exec.DAGRunRef{
		Name: "test-name2",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Check if all returns the jobs
	jobs, err := store.All(th.Context)
	require.NoError(t, err, "expected no error when listing all jobs from store")
	require.Len(t, jobs, 2, "expected to list two jobs")

	// Check if the jobs are sorted by priority
	data1, err := jobs[0].Data()
	require.NoError(t, err, "expected no error when getting job data")
	data2, err := jobs[1].Data()
	require.NoError(t, err, "expected no error when getting job data")

	// Check if the jobs are sorted by priority
	require.Equal(t, "test-name2", data1.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag-2", data1.ID, "expected job ID to be 'test-dag-2'")
	require.Equal(t, "test-name", data2.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", data2.ID, "expected job ID to be 'test-dag'")
}
