package filequeue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/require"
)

func TestJob(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the test file with proper JSON content
	testFilePath := filepath.Join(tmpDir, "test-file.json")
	itemData := ItemData{
		FileName: "test-file.json",
		DAGRun: execution.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	}
	fileContent, err := json.Marshal(itemData)
	require.NoError(t, err, "expected no error when marshaling item data")
	err = os.WriteFile(testFilePath, fileContent, 0644)
	require.NoError(t, err, "expected no error when writing test file")

	// Create a new job
	job := NewJob(testFilePath, itemData)

	// Check if the job ID is correct
	require.Equal(t, "test-file", job.ID(), "expected job ID to be 'test-file'")

	// Check if the job data is correct
	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", jobData.ID, "expected job ID to be 'test-dag'")
}

func TestJob_DataError(t *testing.T) {
	t.Parallel()

	// Create a job with a non-existent file
	job := NewJob("/nonexistent/path/test-file.json", ItemData{
		FileName: "test-file.json",
		DAGRun: execution.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	})

	// Check if Data() returns an error for non-existent file
	_, err := job.Data()
	require.Error(t, err, "expected error when reading non-existent file")
}
