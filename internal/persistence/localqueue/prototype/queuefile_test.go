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
	err = qf.AddJob(th.Context, digraph.WorkflowRef{
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

	// Check if the queue is empty again
	queueLen, err = qf.Len(th.Context)
	require.NoError(t, err, "expected no error when getting queue length")

	// Check if pop returns an error when the queue is empty
	_, err = qf.Pop(th.Context)
	require.ErrorIs(t, err, ErrQueueFileEmpty, "expected error when popping from empty queue")
}

func TestInvalidWorkflow(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Create a new queue file
	qf := NewQueueFile(th.Config.Paths.QueueDir, "test-name", "priority_")
	if qf == nil {
		t.Fatal("expected queue file to be created")
	}

	// Try to add a job with an invalid workflow name
	err := qf.AddJob(th.Context, digraph.WorkflowRef{
		Name:       "invalid-name",
		WorkflowID: "test-workflow",
	})
	require.Error(t, err, "expected error when adding job with invalid workflow name")
}
