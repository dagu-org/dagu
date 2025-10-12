package filequeue

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestJob(t *testing.T) {
	t.Parallel()

	// Create a new job
	job := NewJob(ItemData{
		FileName: "/tmp/test-file.json",
		DAGRun: core.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	})

	// Check if the job ID is correct
	require.Equal(t, "test-file", job.ID(), "expected job ID to be 'test-file'")

	// Check if the job data is correct
	jobData := job.Data()
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", jobData.ID, "expected job ID to be 'test-dag'")
}
