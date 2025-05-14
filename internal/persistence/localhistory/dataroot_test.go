package localhistory

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataRoot(t *testing.T) {
	t.Run("NewDataRoot", func(t *testing.T) {
		t.Parallel()

		t.Run("BasicName", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test-dag"

			dr := NewDataRoot(baseDir, dagName)

			assert.Equal(t, "test-dag", dr.prefix, "prefix should be set correctly")
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "executions"), dr.executionsDir, "path should be set correctly")
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "executions", "*", "*", "*", WorkflowDirPrefix+"*"), dr.globPattern, "globPattern should be set correctly")
		})

		t.Run("WithYAMLExtension", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test-dag.yaml"

			dr := NewDataRoot(baseDir, dagName)

			assert.Equal(t, "test-dag", dr.prefix, "prefix should have extension removed")
		})

		t.Run("WithUnsafeName", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test/dag with spaces.yaml"

			dr := NewDataRoot(baseDir, dagName)

			// Check that the prefix is sanitized (doesn't contain unsafe characters)
			// The SafeName function converts to lowercase and replaces unsafe chars with _
			sanitizedPrefix := "dag_with_spaces"
			assert.True(t, strings.HasPrefix(dr.prefix, sanitizedPrefix), "prefix should be sanitized")

			// Check that there's a hash suffix
			hashSuffix := dr.prefix[len(sanitizedPrefix):]
			assert.True(t, len(hashSuffix) > 0, "prefix should include hash")

			// The hash length might vary based on implementation, so we just check it exists
			assert.True(t, len(hashSuffix) > 1, "hash suffix should be at least 2 characters")
		})
	})
}

func TestDataRootRuns(t *testing.T) {
	t.Parallel()

	t.Run("FindByWorkflowID", func(t *testing.T) {
		ts := models.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ctx := context.Background()

		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-id1", ts)
		_ = root.CreateTestExecution(t, "test-id2", ts)

		actual, err := root.FindByWorkflowID(ctx, "test-id1")
		require.NoError(t, err)

		assert.Equal(t, exec.Workflow, actual, "FindByWorkflowID should return the correct run")
	})

	t.Run("Latest", func(t *testing.T) {
		root := setupTestDataRoot(t)

		ts1 := models.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := models.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := models.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))

		_ = root.CreateTestExecution(t, "test-id1", ts1)
		_ = root.CreateTestExecution(t, "test-id2", ts2)
		_ = root.CreateTestExecution(t, "test-id3", ts3)

		runs := root.Latest(context.Background(), 2)
		require.Len(t, runs, 2)

		assert.Equal(t, "test-id3", runs[0].workflowID, "Latest should return the most recent runs")
	})

	t.Run("LatestAfter", func(t *testing.T) {
		root := setupTestDataRoot(t)

		ts1 := models.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := models.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := models.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))
		ts4 := models.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 1, time.UTC))

		_ = root.CreateTestExecution(t, "test-id1", ts1)
		_ = root.CreateTestExecution(t, "test-id2", ts2)
		latest := root.CreateTestExecution(t, "test-id3", ts3)

		_, err := root.LatestAfter(context.Background(), ts4)
		require.ErrorIs(t, err, models.ErrNoStatusData, "LatestAfter should return ErrNoStatusData when no runs are found")

		run, err := root.LatestAfter(context.Background(), ts3)
		require.NoError(t, err)

		assert.Equal(t, *latest.Workflow, *run, "LatestAfter should return the most recent run after the given timestamp")
	})

	t.Run("ListInRange", func(t *testing.T) {
		root := setupTestDataRoot(t)

		for date := 1; date <= 31; date++ {
			for hour := 0; hour < 24; hour++ {
				ts := models.NewUTC(time.Date(2021, 1, date, hour, 0, 0, 0, time.UTC))
				_ = root.CreateTestExecution(t, fmt.Sprintf("test-id-%d-%d", date, hour), ts)
			}
		}

		// list between 2021-01-01 05:00 and 2021-01-02 02:00
		start := models.NewUTC(time.Date(2021, 1, 1, 5, 0, 0, 0, time.UTC))
		end := models.NewUTC(time.Date(2021, 1, 2, 2, 0, 0, 0, time.UTC))

		result := root.listInRange(context.Background(), start, end, nil)
		require.Len(t, result, 21, "ListInRange should return the correct")

		// Check the first and last timestamps
		first := result[0]
		assert.Equal(t, "2021-01-02 01:00", first.timestamp.Format("2006-01-02 15:04"))

		last := result[len(result)-1]
		assert.Equal(t, "2021-01-01 05:00", last.timestamp.Format("2006-01-02 15:04"))
	})
}

func TestDataRootRemoveOld(_ *testing.T) {
	// TODO: Implement
}

func TestDataRootRename(t *testing.T) {
	root := setupTestDataRoot(t)

	for date := 1; date <= 3; date++ {
		ts := models.NewUTC(time.Date(2021, 1, date, 0, 0, 0, 0, time.UTC))
		_ = root.CreateTestExecution(t, fmt.Sprintf("test-id-%d", date), ts)
	}

	newRoot := NewDataRoot(root.baseDir, "new-dag")
	err := root.Rename(root.Context, newRoot)
	require.NoError(t, err)

	// Check that the old directory is removed
	assert.False(t, root.Exists(), "Old directory should be removed")
	// Check files are moved to the new directory
	assert.True(t, newRoot.Exists(), "New directory should exist")

	matches, err := filepath.Glob(newRoot.globPattern)
	require.NoError(t, err)
	assert.Len(t, matches, 3, "All files should be moved to the new directory")
}

func TestDataRootUtils(t *testing.T) {
	t.Parallel()

	root := setupTestDataRoot(t)

	// Directory does not exist
	assert.False(t, root.Exists(), "Exists should return false when directory does not exist")

	// Create the directory
	err := root.Create()
	require.NoError(t, err)

	// Directory exists
	exists := root.Exists()
	assert.True(t, exists, "Exists should return true when directory exists")

	// IsEmpty should return true for empty directory
	isEmpty := root.IsEmpty()
	assert.True(t, isEmpty, "IsEmpty should return true for empty directory")

	// Add a file to the directory
	root.CreateTestExecution(t, "test-id", models.NewUTC(time.Now()))
	require.NoError(t, err)

	// IsEmpty should return false for non-empty directory
	isEmpty = root.IsEmpty()
	assert.False(t, isEmpty, "IsEmpty should return false for non-empty directory")

	// Remove the directory
	err = root.Remove()
	require.NoError(t, err)

	// Directory does not exist
	assert.False(t, root.Exists(), "Exists should return false when directory does not exist")
}

func setupTestDataRoot(t *testing.T) *DataRootTest {
	t.Helper()

	tmpDir := t.TempDir()
	root := NewDataRoot(tmpDir, "test-dag")
	return &DataRootTest{DataRoot: root, TB: t, Context: context.Background()}
}

type DataRootTest struct {
	DataRoot
	TB      testing.TB
	Context context.Context
}

func (drt *DataRootTest) CreateTestExecution(t *testing.T, workflowID string, ts models.TimeInUTC) ExecutionTest {
	t.Helper()

	err := drt.Create()
	require.NoError(t, err)

	run, err := drt.CreateWorkflow(ts, workflowID)
	require.NoError(t, err)

	return ExecutionTest{
		DataRootTest: *drt,
		Workflow:     run,
		TB:           t}
}
