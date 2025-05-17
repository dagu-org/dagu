package prototype

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueueFile(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new queue file
	qf := NewQueueFile(th.Config.Paths.QueueDir, "test-name", "priority_")
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
	err = qf.Push(th.Context, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Check if the queue length is 1
	queueLen, err = qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 1, queueLen, "expected queue length to be 1")

	// Dequeue the job
	job, err := qf.Pop(th.Context)
	require.NoError(t, err, "expected no error when popping job from queue")

	require.NotNil(t, job, "expected job to be not nil")
	require.Equal(t, "test-name", job.Workflow.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-workflow", job.Workflow.WorkflowID, "expected job ID to be 'test'")

	// Check if the item has the correct prefix
	require.Regexp(t, "^priority_", job.FileName, "expected job file name to start with 'priority_'")

	// Check if the queue is empty again
	queueLen, err = qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")

	// Check if pop returns an error when the queue is empty
	_, err = qf.Pop(th.Context)
	require.ErrorIs(t, err, ErrQueueFileEmpty, "expected error when popping from empty queue")
}

func TestQueueFile_FindByWorkflowID(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new queue file
	qf := NewQueueFile(th.Config.Paths.QueueDir, "test-name", "priority_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Add a job to the queue
	err := qf.Push(th.Context, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Find the job by workflow ID
	job, err := qf.FindByWorkflowID(th.Context, "test-workflow")
	require.NoError(t, err, "expected no error when finding job by workflow ID")
	require.NotNil(t, job, "expected job to be not nil")
	require.Equal(t, "test-name", job.Workflow.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-workflow", job.Workflow.WorkflowID, "expected job ID to be 'test'")

	// Check if the item has the correct prefix
	require.Regexp(t, "^priority_", job.FileName, "expected job file name to start with 'priority_'")
}

func TestQueueFile_RemoveJob(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new queue file
	qf := NewQueueFile(th.Config.Paths.QueueDir, "test-name", "priority_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Add a job to the queue
	err := qf.Push(th.Context, digraph.WorkflowRef{
		Name:       "test-name",
		WorkflowID: "test-workflow",
	})
	require.NoError(t, err, "expected no error when adding job to queue")

	// Remove the job from the queue
	err = qf.DeleteByWorkflowID(th.Context, "test-workflow")
	require.NoError(t, err, "expected no error when removing job from queue")

	// Check if the queue is empty
	queueLen, err := qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")
	require.Equal(t, 0, queueLen, "expected queue length to be 0")
}

func TestQueueFile_Error(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new queue file
	qf := NewQueueFile(th.Config.Paths.QueueDir, "test-name", "priority_")
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
		require.ErrorIs(t, err, ErrQueueFileEmpty, "expected error when popping from empty queue")
	})

	t.Run("InvalidWorkflowID", func(t *testing.T) {
		// Try to add a job with an invalid workflow name
		err := qf.Push(th.Context, digraph.WorkflowRef{
			Name:       "invalid-name",
			WorkflowID: "test-workflow",
		})
		require.Error(t, err, "expected error when adding job with invalid workflow name")
	})
}
