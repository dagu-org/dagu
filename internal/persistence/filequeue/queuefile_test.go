package filequeue_test

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueueFile(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")

	// Create a new queue file
	qf := filequeue.NewQueueFile(queueDir, "high_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Check if the queue is empty
	queueLen, err := qf.Len(th.Context)
	if err != nil {
		t.Fatalf("expected no error when getting queue length: %v", err)
	}
	require.Equal(t, 0, queueLen, "expected queue length to be 0")

	// Add a job to the queue
	err = qf.Push(th.Context, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if the queue length is 1
	queueLen, err = qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 1, queueLen, "expected queue length to be 1")

	// Check if pop returns the job
	job, err := qf.Pop(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")

	require.NotNil(t, job, "expected job to be not nil")
	require.Equal(t, "test-name", job.DAGRun.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", job.DAGRun.ID, "expected job ID to be 'test'")

	// Check if the item has the correct prefix
	require.Regexp(t, "^item_high_", job.FileName, "expected job file name to start with 'item_priority_'")

	// Check if the queue is empty again
	_, err = qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")

	// Check if pop returns an error when the queue is empty
	_, err = qf.Pop(th.Context)
	require.ErrorIs(t, err, filequeue.ErrQueueFileEmpty, "expected error when popping from empty queue")
}

func TestQueueFile_FindByDAGRunID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")

	// Create a new queue file
	qf := filequeue.NewQueueFile(queueDir, "high_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Add a job to the queue
	err := qf.Push(th.Context, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Find the job by dag-run ID
	job, err := qf.FindByDAGRunID(th.Context, "test-dag")
	require.NoError(t, err, "expected no error when finding job by dag-run ID")
	require.NotNil(t, job, "expected job to be not nil")
	require.Equal(t, "test-name", job.DAGRun.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", job.DAGRun.ID, "expected job ID to be 'test'")

	// Check if the item has the correct prefix
	require.Regexp(t, "^item_high_", job.FileName, "expected job file name to start with 'high_'")
}

func TestQueueFile_Pop(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")

	// Create a new queue file
	qf := filequeue.NewQueueFile(queueDir, "high_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Add a job to the queue
	err := qf.Push(th.Context, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Remove the job from the queue
	removedJobs, err := qf.PopByDAGRunID(th.Context, "test-dag")
	require.NoError(t, err, "expected no error when removing job from queue")
	require.Len(t, removedJobs, 1, "expected one job to be removed")
	require.Equal(t, "test-name", removedJobs[0].DAGRun.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", removedJobs[0].DAGRun.ID, "expected job ID to be 'test'")

	// Check if the queue is empty
	queueLen, err := qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 0, queueLen, "expected queue length to be 0")
}

func TestQueueFile_Error(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	queueDir := filepath.Join(th.Config.Paths.QueueDir, "test-name")

	// Create a new queue file
	qf := filequeue.NewQueueFile(queueDir, "high_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	t.Run("EmptyQueue", func(t *testing.T) {
		// Check if the queue is empty
		queueLen, err := qf.Len(th.Context)
		require.NoError(t, err, "expected no error when getting queue length")
		require.Equal(t, 0, queueLen, "expected queue length to be 0")

		// Check if pop returns an error when the queue is empty
		_, err = qf.Pop(th.Context)
		require.ErrorIs(t, err, filequeue.ErrQueueFileEmpty, "expected error when popping from empty queue")
	})
}
