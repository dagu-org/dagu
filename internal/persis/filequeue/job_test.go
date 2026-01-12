package filequeue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestQueuedFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the test file with proper JSON content
	testFilePath := filepath.Join(tmpDir, "test-file.json")
	itemData := ItemData{
		FileName: "test-file.json",
		DAGRun: exec.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	}
	fileContent, err := json.Marshal(itemData)
	require.NoError(t, err, "expected no error when marshaling item data")
	err = os.WriteFile(testFilePath, fileContent, 0644)
	require.NoError(t, err, "expected no error when writing test file")

	// Create a new QueuedFile
	queuedFile := NewQueuedFile(testFilePath)

	// Check if the ID is correct
	require.Equal(t, "test-file", queuedFile.ID(), "expected job ID to be 'test-file'")

	// Check if the data is correct (lazy loading from file)
	jobData, err := queuedFile.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", jobData.ID, "expected job ID to be 'test-dag'")
}

func TestQueuedFile_DataError(t *testing.T) {
	t.Parallel()

	// Create a QueuedFile with a non-existent file
	queuedFile := NewQueuedFile("/nonexistent/path/test-file.json")

	// Check if Data() returns an error for non-existent file
	_, err := queuedFile.Data()
	require.Error(t, err, "expected error when reading non-existent file")
}

func TestQueuedFile_ExtractJob(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the test file with proper JSON content
	testFilePath := filepath.Join(tmpDir, "test-file.json")
	itemData := ItemData{
		FileName: "test-file.json",
		DAGRun: exec.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	}
	fileContent, err := json.Marshal(itemData)
	require.NoError(t, err, "expected no error when marshaling item data")
	err = os.WriteFile(testFilePath, fileContent, 0644)
	require.NoError(t, err, "expected no error when writing test file")

	// Create a QueuedFile and extract the Job
	queuedFile := NewQueuedFile(testFilePath)
	job, err := queuedFile.ExtractJob()
	require.NoError(t, err, "expected no error when extracting job")

	// Delete the file to simulate what happens after Pop()
	err = os.Remove(testFilePath)
	require.NoError(t, err, "expected no error when removing test file")

	// The extracted Job should still have the cached data
	jobData, err := job.Data()
	require.NoError(t, err, "expected no error when getting job data after file deletion")
	require.Equal(t, "test-name", jobData.Name, "expected job name to be 'test-name'")
	require.Equal(t, "test-dag", jobData.ID, "expected job ID to be 'test-dag'")
}

func TestQueuedFile_Caching(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the test file with proper JSON content
	testFilePath := filepath.Join(tmpDir, "test-file.json")
	itemData := ItemData{
		FileName: "test-file.json",
		DAGRun: exec.DAGRunRef{
			Name: "test-name",
			ID:   "test-dag",
		},
	}
	fileContent, err := json.Marshal(itemData)
	require.NoError(t, err, "expected no error when marshaling item data")
	err = os.WriteFile(testFilePath, fileContent, 0644)
	require.NoError(t, err, "expected no error when writing test file")

	// Create a QueuedFile
	queuedFile := NewQueuedFile(testFilePath)

	// First call to Data() should load from file
	jobData1, err := queuedFile.Data()
	require.NoError(t, err, "expected no error when getting job data")
	require.Equal(t, "test-name", jobData1.Name)

	// Delete the file
	err = os.Remove(testFilePath)
	require.NoError(t, err, "expected no error when removing test file")

	// Second call to Data() should use cached data
	jobData2, err := queuedFile.Data()
	require.NoError(t, err, "expected no error when getting cached job data")
	require.Equal(t, "test-name", jobData2.Name)
	require.Equal(t, "test-dag", jobData2.ID)
}
