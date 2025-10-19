package filedagrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "dag-runs"), dr.dagRunsDir, "path should be set correctly")
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "dag-runs", "*", "*", "*", DAGRunDirPrefix+"*"), dr.globPattern, "globPattern should be set correctly")
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

	t.Run("FindByDAGRunID", func(t *testing.T) {
		ts := execution.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ctx := context.Background()

		dr := setupTestDataRoot(t)
		dagRun := dr.CreateTestDAGRun(t, "test-id1", ts)
		_ = dr.CreateTestDAGRun(t, "test-id2", ts)

		actual, err := dr.FindByDAGRunID(ctx, "test-id1")
		require.NoError(t, err)

		assert.Equal(t, dagRun.DAGRun, actual, "FindByDAGRunID should return the correct run")
	})

	t.Run("Latest", func(t *testing.T) {
		root := setupTestDataRoot(t)

		ts1 := execution.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := execution.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := execution.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))

		_ = root.CreateTestDAGRun(t, "test-id1", ts1)
		_ = root.CreateTestDAGRun(t, "test-id2", ts2)
		_ = root.CreateTestDAGRun(t, "test-id3", ts3)

		runs := root.Latest(context.Background(), 2)
		require.Len(t, runs, 2)

		assert.Equal(t, "test-id3", runs[0].dagRunID, "Latest should return the most recent runs")
	})

	t.Run("LatestAfter", func(t *testing.T) {
		root := setupTestDataRoot(t)

		ts1 := execution.NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := execution.NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := execution.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))
		ts4 := execution.NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 1, time.UTC))

		_ = root.CreateTestDAGRun(t, "test-id1", ts1)
		_ = root.CreateTestDAGRun(t, "test-id2", ts2)
		latest := root.CreateTestDAGRun(t, "test-id3", ts3)

		_, err := root.LatestAfter(context.Background(), ts4)
		require.ErrorIs(t, err, execution.ErrNoStatusData, "LatestAfter should return ErrNoStatusData when no runs are found")

		run, err := root.LatestAfter(context.Background(), ts3)
		require.NoError(t, err)

		assert.Equal(t, *latest.DAGRun, *run, "LatestAfter should return the most recent run after the given timestamp")
	})

	t.Run("ListInRange", func(t *testing.T) {
		root := setupTestDataRoot(t)

		for date := 1; date <= 31; date++ {
			for hour := 0; hour < 24; hour++ {
				ts := execution.NewUTC(time.Date(2021, 1, date, hour, 0, 0, 0, time.UTC))
				_ = root.CreateTestDAGRun(t, fmt.Sprintf("test-id-%d-%d", date, hour), ts)
			}
		}

		// list between 2021-01-01 05:00 and 2021-01-02 02:00
		start := execution.NewUTC(time.Date(2021, 1, 1, 5, 0, 0, 0, time.UTC))
		end := execution.NewUTC(time.Date(2021, 1, 2, 2, 0, 0, 0, time.UTC))

		result := root.listDAGRunsInRange(context.Background(), start, end, &listDAGRunsInRangeOpts{})
		require.Len(t, result, 21, "ListInRange should return the correct")

		// Check the first and last timestamps
		first := result[0]
		assert.Equal(t, "2021-01-02 01:00", first.timestamp.Format("2006-01-02 15:04"))

		last := result[len(result)-1]
		assert.Equal(t, "2021-01-01 05:00", last.timestamp.Format("2006-01-02 15:04"))
	})
}

func TestDataRootRemoveOld(t *testing.T) {
	t.Run("RemoveAllWhenRetentionIsZero", func(t *testing.T) {
		root := setupTestDataRoot(t)

		// Use old timestamps like the working store_test.go
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)

		// Create dag-runs with old timestamps
		dagRun1 := root.CreateTestDAGRun(t, "dag-run-1", execution.NewUTC(ts1))
		dagRun2 := root.CreateTestDAGRun(t, "dag-run-2", execution.NewUTC(ts2))

		// Create actual attempts with status data using old timestamps
		createAttemptWithStatus := func(dagRunTest DAGRunTest, ts time.Time) *Attempt {
			attempt, err := dagRunTest.CreateAttempt(root.Context, execution.NewUTC(ts), nil)
			require.NoError(t, err)
			require.NoError(t, attempt.Open(root.Context))
			status := execution.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: dagRunTest.dagRunID,
				Status:   core.Succeeded,
			}
			require.NoError(t, attempt.Write(root.Context, status))
			require.NoError(t, attempt.Close(root.Context))

			// Set the file modification time to match the old timestamp
			err = os.Chtimes(attempt.file, ts, ts)
			require.NoError(t, err)

			return attempt
		}

		createAttemptWithStatus(dagRun1, ts1)
		createAttemptWithStatus(dagRun2, ts2)

		// Verify dag-runs exist
		assert.True(t, fileutil.FileExists(dagRun1.baseDir), "dag-run 1 should exist before cleanup")
		assert.True(t, fileutil.FileExists(dagRun2.baseDir), "dag-run 2 should exist before cleanup")

		// Remove all dag-runs (retention = 0)
		err := root.RemoveOld(root.Context, 0)
		require.NoError(t, err)

		// Verify all dag-runs are removed
		assert.False(t, fileutil.FileExists(dagRun1.baseDir), "dag-run 1 should be removed")
		assert.False(t, fileutil.FileExists(dagRun2.baseDir), "dag-run 2 should be removed")
	})

	t.Run("KeepRecentWhenRetentionIsPositive", func(t *testing.T) {
		root := setupTestDataRoot(t)

		// Create dag-runs: one old and one recent
		oldTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		recentTime := time.Now().AddDate(0, 0, -1) // 1 day ago

		dagRun1 := root.CreateTestDAGRun(t, "old-dag-run", execution.NewUTC(oldTime))
		dagRun2 := root.CreateTestDAGRun(t, "recent-dag-run", execution.NewUTC(recentTime))

		// Create actual attempts with status data
		createAttemptWithStatus := func(dagRunTest DAGRunTest, ts time.Time) *Attempt {
			attempt, err := dagRunTest.CreateAttempt(root.Context, execution.NewUTC(ts), nil)
			require.NoError(t, err)
			require.NoError(t, attempt.Open(root.Context))
			status := execution.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: dagRunTest.dagRunID,
				Status:   core.Succeeded,
			}
			require.NoError(t, attempt.Write(root.Context, status))
			require.NoError(t, attempt.Close(root.Context))

			// Set the file modification time to match the timestamp
			err = os.Chtimes(attempt.file, ts, ts)
			require.NoError(t, err)

			return attempt
		}

		createAttemptWithStatus(dagRun1, oldTime)
		createAttemptWithStatus(dagRun2, recentTime)

		// Verify dag-runs exist
		assert.True(t, fileutil.FileExists(dagRun1.baseDir), "Old dag-run should exist before cleanup")
		assert.True(t, fileutil.FileExists(dagRun2.baseDir), "Recent dag-run should exist before cleanup")

		// Remove dag-runs older than 7 days (should remove old but keep recent)
		err := root.RemoveOld(root.Context, 7)
		require.NoError(t, err)

		// Verify old dag-run is removed but recent one is kept
		assert.False(t, fileutil.FileExists(dagRun1.baseDir), "Old dag-run should be removed")
		assert.True(t, fileutil.FileExists(dagRun2.baseDir), "Recent dag-run should be kept")
	})

	t.Run("RemoveEmptyDirectories", func(t *testing.T) {
		root := setupTestDataRoot(t)

		// Create dag-runs in different date directories
		date1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		date2 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

		dagRun1 := root.CreateTestDAGRun(t, "dag-run-1", execution.NewUTC(date1))
		dagRun2 := root.CreateTestDAGRun(t, "dag-run-2", execution.NewUTC(date2))

		// Create actual attempts with status data
		createAttemptWithStatus := func(dagRunTest DAGRunTest, ts time.Time) *Attempt {
			attempt, err := dagRunTest.CreateAttempt(root.Context, execution.NewUTC(ts), nil)
			require.NoError(t, err)
			require.NoError(t, attempt.Open(root.Context))
			status := execution.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: dagRunTest.dagRunID,
				Status:   core.Succeeded,
			}
			require.NoError(t, attempt.Write(root.Context, status))
			require.NoError(t, attempt.Close(root.Context))

			// Set the file modification time to match the old timestamp
			err = os.Chtimes(attempt.file, ts, ts)
			require.NoError(t, err)

			return attempt
		}

		createAttemptWithStatus(dagRun1, date1)
		createAttemptWithStatus(dagRun2, date2)

		// Verify directory structure exists
		assert.True(t, fileutil.FileExists(dagRun1.baseDir), "dag-run 1 should exist")
		assert.True(t, fileutil.FileExists(dagRun2.baseDir), "dag-run 2 should exist")

		// Remove all old dag-runs (retention = 0)
		err := root.RemoveOld(root.Context, 0)
		require.NoError(t, err)

		// Verify dag-runs are removed
		assert.False(t, fileutil.FileExists(dagRun1.baseDir), "dag-run 1 should be removed")
		assert.False(t, fileutil.FileExists(dagRun2.baseDir), "dag-run 2 should be removed")

		// Verify that the cleanup also removes empty directories
		// The method should clean up empty year/month/day directories
		assert.True(t, root.IsEmpty(), "Root should be empty after cleanup")
	})
}

func TestDataRootRename(t *testing.T) {
	root := setupTestDataRoot(t)

	for date := 1; date <= 3; date++ {
		ts := execution.NewUTC(time.Date(2021, 1, date, 0, 0, 0, 0, time.UTC))
		_ = root.CreateTestDAGRun(t, fmt.Sprintf("test-id-%d", date), ts)
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
	root.CreateTestDAGRun(t, "test-id", execution.NewUTC(time.Now()))
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

// setupTestDataRoot creates a DataRootTest instance for testing purposes.
func setupTestDataRoot(t *testing.T) *DataRootTest {
	t.Helper()

	tmpDir := t.TempDir()
	root := NewDataRoot(tmpDir, "test-dag")
	return &DataRootTest{DataRoot: root, TB: t, Context: context.Background()}
}

// DataRootTest extends DataRoot with testing utilities and context.
type DataRootTest struct {
	DataRoot
	TB      testing.TB
	Context context.Context
}

// CreateTestDAGRun creates a test dag-run with the specified ID and timestamp.
// It ensures the DataRoot directory exists before creating the dag-run.
func (drt *DataRootTest) CreateTestDAGRun(t *testing.T, dagRunID string, ts execution.TimeInUTC) DAGRunTest {
	t.Helper()

	err := drt.Create()
	require.NoError(t, err)

	run, err := drt.CreateDAGRun(ts, dagRunID)
	require.NoError(t, err)

	return DAGRunTest{
		DataRootTest: *drt,
		DAGRun:       run,
		TB:           t,
	}
}
