package filedagrun

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttempt_Open(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "status.dat")

	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Test successful open
	err = att.Open(context.Background())
	assert.NoError(t, err)

	// Test open when already open
	err = att.Open(context.Background())
	assert.ErrorIs(t, err, ErrStatusFileOpen)

	// Cleanup
	err = att.Close(context.Background())
	assert.NoError(t, err)
}

func TestAttempt_Write(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "status.dat")

	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Test write without open
	testStatus := createTestStatus(status.Running)
	err = att.Write(context.Background(), testStatus)
	assert.ErrorIs(t, err, ErrStatusFileNotOpen)

	// Open and write
	err = att.Open(context.Background())
	require.NoError(t, err)

	// Write test status
	err = att.Write(context.Background(), testStatus)
	assert.NoError(t, err)

	// Verify file content
	actual, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "test", actual.DAGRunID)
	assert.Equal(t, status.Running, actual.Status)

	// Close
	err = att.Close(context.Background())
	assert.NoError(t, err)
}

func TestAttempt_Read(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "status.dat")

	// Create test file with multiple status entries
	status1 := createTestStatus(status.Running)
	status2 := createTestStatus(status.Success)

	// Create file directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(file), 0750)
	require.NoError(t, err)

	// Create test file with two status entries
	f, err := os.Create(file)
	require.NoError(t, err)

	data1, err := json.Marshal(status1)
	require.NoError(t, err)
	_, err = f.Write(append(data1, '\n'))
	require.NoError(t, err)

	data2, err := json.Marshal(status2)
	require.NoError(t, err)
	_, err = f.Write(append(data2, '\n'))
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	// Initialize attempt
	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Read status - should get the last entry (test2)
	dagRunStatus, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, status.Success.String(), dagRunStatus.Status.String())

	// Read using ReadStatus
	latestStatus, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, status.Success.String(), latestStatus.Status.String())
}

func TestAttempt_Compact(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "status.dat")

	// Create test file with multiple status entries
	for i := 0; i < 10; i++ {
		testStatus := createTestStatus(status.Running)

		if i == 9 {
			// Make some status changes to create different attempts
			testStatus.Status = status.Success
		}

		if i == 0 {
			// Create new file for first write
			writeJSONToFile(t, file, testStatus)
		} else {
			// Append to existing file
			data, err := json.Marshal(testStatus)
			require.NoError(t, err)

			f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
			require.NoError(t, err)

			_, err = f.Write(append(data, '\n'))
			require.NoError(t, err)
			_ = f.Close()
		}
	}

	// Get file size before compaction
	fileInfo, err := os.Stat(file)
	require.NoError(t, err)
	beforeSize := fileInfo.Size()

	// Initialize Attempt
	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Compact the file
	err = att.Compact(context.Background())
	assert.NoError(t, err)

	// Get file size after compaction
	fileInfo, err = os.Stat(file)
	require.NoError(t, err)
	afterSize := fileInfo.Size()

	// Verify file size reduced
	assert.Less(t, afterSize, beforeSize)

	// Verify content is still correct
	dagRunStatus, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, status.Success, dagRunStatus.Status)
}

func TestAttempt_Close(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "status.dat")

	// Initialize and open Attempt
	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	err = att.Open(context.Background())
	require.NoError(t, err)

	// Write some data
	err = att.Write(context.Background(), createTestStatus(status.Running))
	require.NoError(t, err)

	// Close
	err = att.Close(context.Background())
	assert.NoError(t, err)

	// Verify we can't write after close
	err = att.Write(context.Background(), createTestStatus(status.Success))
	assert.ErrorIs(t, err, ErrStatusFileNotOpen)

	// Test double close is safe
	err = att.Close(context.Background())
	assert.NoError(t, err)
}

func TestAttempt_HandleNonExistentFile(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "invalid.dat")

	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Should be able to open a non-existent file
	err = att.Open(context.Background())
	assert.NoError(t, err)

	// Write to create the file
	err = att.Write(context.Background(), createTestStatus(status.Success))
	assert.NoError(t, err)

	// Verify the file was created with correct data
	dagRunStatus, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "test", dagRunStatus.DAGRunID)

	// Cleanup
	err = att.Close(context.Background())
	assert.NoError(t, err)
}

func TestAttempt_EmptyFile(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "empty.dat")

	// Create an empty file
	f, err := os.Create(file)
	require.NoError(t, err)
	_ = f.Close()

	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Reading an empty file should return ErrCorruptedStatusFile
	_, err = att.ReadStatus(context.Background())
	assert.ErrorIs(t, err, models.ErrCorruptedStatusFile)

	// Compacting an empty file should be safe
	err = att.Compact(context.Background())
	assert.NoError(t, err)
}

func TestAttempt_InvalidJSON(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "invalid.dat")

	// Create a file with valid JSOn
	validStatus := createTestStatus(status.Running)
	writeJSONToFile(t, file, validStatus)

	// Append invalid JSON
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	_, err = f.Write([]byte("invalid json\n"))
	require.NoError(t, err)

	att, err := NewAttempt(file, nil)
	require.NoError(t, err)

	// Should be able to read and get the valid entry
	dagRunStatus, err := att.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, status.Running.String(), dagRunStatus.Status.String())
}

func TestAttempt_CorruptedStatusFile(t *testing.T) {
	t.Run("EmptyFile", func(t *testing.T) {
		dir := createTempDir(t)
		file := filepath.Join(dir, "empty.jsonl")

		// Create empty file
		f, err := os.Create(file)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		att, err := NewAttempt(file, nil)
		require.NoError(t, err)

		// Should return ErrCorruptedStatusFile
		_, err = att.ReadStatus(context.Background())
		assert.ErrorIs(t, err, models.ErrCorruptedStatusFile)
	})

	t.Run("OnlyWhitespace", func(t *testing.T) {
		dir := createTempDir(t)
		file := filepath.Join(dir, "whitespace.jsonl")

		// Create file with only whitespace
		err := os.WriteFile(file, []byte("\n\n\n"), 0600)
		require.NoError(t, err)

		att, err := NewAttempt(file, nil)
		require.NoError(t, err)

		// Should return ErrCorruptedStatusFile
		_, err = att.ReadStatus(context.Background())
		assert.ErrorIs(t, err, models.ErrCorruptedStatusFile)
	})

	t.Run("NoValidJSON", func(t *testing.T) {
		dir := createTempDir(t)
		file := filepath.Join(dir, "novalid.jsonl")

		// Create file with only invalid JSON
		err := os.WriteFile(file, []byte("not json\nstill not json\n"), 0600)
		require.NoError(t, err)

		att, err := NewAttempt(file, nil)
		require.NoError(t, err)

		// Should return ErrCorruptedStatusFile
		_, err = att.ReadStatus(context.Background())
		assert.ErrorIs(t, err, models.ErrCorruptedStatusFile)
	})
}

func TestReadLineFrom(t *testing.T) {
	dir := createTempDir(t)
	file := filepath.Join(dir, "lines.txt")

	// Create a test file with multiple lines
	content := "line1\nline2\nline3\n"
	err := os.WriteFile(file, []byte(content), 0600)
	require.NoError(t, err)

	f, err := os.Open(file)
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	// Read first line
	line1, offset, err := readLineFrom(f, 0)
	assert.NoError(t, err)
	assert.Equal(t, "line1", string(line1))
	assert.Equal(t, int64(6), offset) // "line1\n" = 6 bytes

	// Read second line
	line2, offset, err := readLineFrom(f, offset)
	assert.NoError(t, err)
	assert.Equal(t, "line2", string(line2))
	assert.Equal(t, int64(12), offset) // offset 6 + "line2\n" = 12 bytes

	// Read third line
	line3, offset, err := readLineFrom(f, offset)
	assert.NoError(t, err)
	assert.Equal(t, "line3", string(line3))
	assert.Equal(t, int64(18), offset) // offset 12 + "line3\n" = 18 bytes

	// Try to read beyond EOF
	_, _, err = readLineFrom(f, offset)
	assert.ErrorIs(t, err, io.EOF)
}

func TestSafeRename(t *testing.T) {
	dir := createTempDir(t)
	sourceFile := filepath.Join(dir, "source.txt")
	targetFile := filepath.Join(dir, "target.txt")

	// Create source file
	err := os.WriteFile(sourceFile, []byte("test content"), 0600)
	require.NoError(t, err)

	// Test rename when target doesn't exist
	err = safeRename(sourceFile, targetFile)
	assert.NoError(t, err)
	assert.FileExists(t, targetFile)
	assert.NoFileExists(t, sourceFile)

	// Create source again
	err = os.WriteFile(sourceFile, []byte("new content"), 0600)
	require.NoError(t, err)

	// Test rename when target exists
	err = safeRename(sourceFile, targetFile)
	assert.NoError(t, err)
	assert.FileExists(t, targetFile)
	assert.NoFileExists(t, sourceFile)

	// Read target to verify content was updated
	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestAttempt_HideAndIsHidden(t *testing.T) {
	ctx := context.Background()

	// Create attempt
	dir := createTempDir(t)
	statusFile := filepath.Join(dir, "status.dat")
	att, err := NewAttempt(statusFile, nil)
	require.NoError(t, err)

	// Initially not hidden
	assert.False(t, att.Hidden())

	// Can't hide when open
	require.NoError(t, att.Open(ctx))
	assert.ErrorIs(t, att.Hide(ctx), ErrStatusFileOpen)

	// Can hide after close
	require.NoError(t, att.Close(ctx))
	require.NoError(t, att.Hide(ctx))
	assert.True(t, att.Hidden())

	// Idempotent
	require.NoError(t, att.Hide(ctx))
	assert.True(t, att.Hidden())
}

// createTempDir creates a temporary directory for testing
func createTempDir(t *testing.T) string {
	t.Helper()

	attemptID, err := genAttemptID()
	require.NoError(t, err)

	dir, err := os.MkdirTemp("", "attempt_"+formatAttemptTimestamp(models.NewUTC(time.Now()))+"_"+attemptID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	return dir
}

// createTestDAG creates a sample DAG for testing
func createTestDAG() *digraph.DAG {
	return &digraph.DAG{
		Name: "TestDAG",
		Steps: []digraph.Step{
			{
				Name:    "step1",
				Command: "echo 'step1'",
			},
			{
				Name:    "step2",
				Command: "echo 'step2'",
				Depends: []string{
					"step1",
				},
			},
		},
		HandlerOn: digraph.HandlerOn{
			Success: &digraph.Step{
				Name:    "on_success",
				Command: "echo 'success'",
			},
			Failure: &digraph.Step{
				Name:    "on_failure",
				Command: "echo 'failure'",
			},
		},
		Params: []string{"--param1=value1", "--param2=value2"},
	}
}

// createTestStatus creates a sample status for testing using StatusFactory
func createTestStatus(st status.Status) models.DAGRunStatus {
	dag := createTestDAG()

	return models.DAGRunStatus{
		Name:      dag.Name,
		DAGRunID:  "test",
		Status:    st,
		PID:       models.PID(12345),
		StartedAt: stringutil.FormatTime(time.Now()),
		Nodes:     models.NodesFromSteps(dag.Steps),
	}
}

// writeJSONToFile writes a JSON object to a file for testing
func writeJSONToFile(t *testing.T, file string, obj any) {
	t.Helper()
	data, err := json.Marshal(obj)
	require.NoError(t, err)

	err = os.WriteFile(file, append(data, '\n'), 0600)
	require.NoError(t, err)
}
