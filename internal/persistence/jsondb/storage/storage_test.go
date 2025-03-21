package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	// Storage tests
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := New()
		assert.NotNil(t, s, "New() should return a non-nil Storage")
		_, ok := s.(*storage)
		assert.True(t, ok, "New() should return a *storage")
	})

	t.Run("TimeInUTC", func(t *testing.T) {
		t.Parallel()

		t.Run("NewUTC", func(t *testing.T) {
			t.Parallel()
			now := time.Now()
			utc := NewUTC(now)
			assert.Equal(t, now.UTC(), utc.Time, "NewUTC should convert time to UTC")
		})

		t.Run("Format", func(t *testing.T) {
			t.Parallel()
			// Create a fixed time for testing
			fixedTime := time.Date(2023, 4, 15, 12, 30, 45, 123000000, time.UTC)
			utc := NewUTC(fixedTime)

			formatted := utc.Format(dateTimeFormatUTC)
			assert.Equal(t, "20230415_123045_000Z", formatted, "Format should format time correctly")
		})
	})

	t.Run("GenerateFilePath", func(t *testing.T) {
		t.Parallel()
		s := New()
		ctx := context.Background()

		tmpDir := t.TempDir()
		a := NewAddress(tmpDir, "test-dag")

		timestamp := NewUTC(time.Date(2023, 4, 15, 12, 30, 45, 0, time.UTC))
		reqID := "req123"

		path := s.GenerateFilePath(ctx, a, timestamp, reqID)

		// Verify the path format
		expected := filepath.Join(tmpDir, "test-dag", "executions", "2023", "04", "15", "exec_20230415_123045_000Z_req123", "status.dat")
		assert.Equal(t, expected, path, "GenerateFilePath should generate the correct path")
	})

	t.Run("Latest", func(t *testing.T) {
		t.Parallel()

		// Setup test directory with files
		tmpDir := t.TempDir()
		setupTestFiles(t, tmpDir)

		s := New()
		ctx := context.Background()
		a := NewAddress(tmpDir, "test-dag")

		t.Run("NoFiles", func(t *testing.T) {
			t.Parallel()
			emptyDir := t.TempDir()
			emptyAddr := NewAddress(emptyDir, "empty-dag")

			files := s.Latest(ctx, emptyAddr, 10)
			assert.Empty(t, files, "Latest should return empty slice when no files exist")
		})

		t.Run("WithLimit", func(t *testing.T) {
			files := s.Latest(ctx, a, 2)
			require.Len(t, files, 2, "Latest should return at most itemLimit files")

			// Verify files are sorted by timestamp (most recent first)
			assert.Contains(t, files[0], "20230415_123045")
			assert.Contains(t, files[1], "20230415_123030")
		})

		t.Run("InvalidGlobPattern", func(t *testing.T) {
			// Create an address with an invalid glob pattern
			invalidAddr := Address{
				dagName:       "invalid",
				prefix:        "invalid",
				executionsDir: filepath.Join(tmpDir, "invalid"),
				globPattern:   "[", // Invalid glob pattern
			}

			files := s.Latest(ctx, invalidAddr, 10)
			assert.Empty(t, files, "Latest should handle invalid glob patterns gracefully")
		})
	})

	t.Run("LatestAfter", func(t *testing.T) {
		t.Parallel()

		// Setup test directory with files
		tmpDir := t.TempDir()
		setupTestFiles(t, tmpDir)

		s := New()
		ctx := context.Background()
		a := NewAddress(tmpDir, "test-dag")

		t.Run("NoFiles", func(t *testing.T) {
			t.Parallel()
			emptyDir := t.TempDir()
			emptyAddr := NewAddress(emptyDir, "empty-dag")

			_, err := s.LatestAfter(ctx, emptyAddr, TimeInUTC{})
			assert.ErrorIs(t, err, persistence.ErrNoStatusData, "LatestAfter should return ErrNoStatusData when no files exist")
		})

		t.Run("WithZeroCutoff", func(t *testing.T) {
			file, err := s.LatestAfter(ctx, a, TimeInUTC{})
			assert.NoError(t, err, "LatestAfter should not return error with zero cutoff")
			assert.Contains(t, file, "20230415_123045", "LatestAfter should return the most recent file")
		})

		t.Run("WithCutoffBefore", func(t *testing.T) {
			cutoff := NewUTC(time.Date(2023, 4, 15, 12, 20, 0, 0, time.UTC))
			file, err := s.LatestAfter(ctx, a, cutoff)
			assert.NoError(t, err, "LatestAfter should not return error when cutoff is before latest file")
			assert.Contains(t, file, "20230415_123045", "LatestAfter should return the most recent file")
		})

		t.Run("WithCutoffAfter", func(t *testing.T) {
			cutoff := NewUTC(time.Date(2023, 4, 15, 13, 0, 0, 0, time.UTC))
			_, err := s.LatestAfter(ctx, a, cutoff)
			assert.ErrorIs(t, err, persistence.ErrNoStatusData, "LatestAfter should return ErrNoStatusData when cutoff is after latest file")
		})

		t.Run("InvalidGlobPattern", func(t *testing.T) {
			// Create an address with an invalid glob pattern
			invalidAddr := Address{
				dagName:       "invalid",
				prefix:        "invalid",
				executionsDir: filepath.Join(tmpDir, "invalid"),
				globPattern:   "[", // Invalid glob pattern
			}

			_, err := s.LatestAfter(ctx, invalidAddr, TimeInUTC{})
			assert.ErrorIs(t, err, persistence.ErrNoStatusData, "LatestAfter should handle invalid glob patterns gracefully")
		})
	})

	t.Run("FindByRequestID", func(t *testing.T) {
		t.Parallel()

		// Setup test directory with files
		tmpDir := t.TempDir()
		setupTestFiles(t, tmpDir)

		s := New()
		ctx := context.Background()
		a := NewAddress(tmpDir, "test-dag")

		t.Run("NoFiles", func(t *testing.T) {
			t.Parallel()
			emptyDir := t.TempDir()
			emptyAddr := NewAddress(emptyDir, "empty-dag")

			_, err := s.FindByRequestID(ctx, emptyAddr, "req123")
			assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound, "FindByRequestID should return ErrRequestIDNotFound when no files exist")
		})

		t.Run("FileExists", func(t *testing.T) {
			file, err := s.FindByRequestID(ctx, a, "req123")
			assert.NoError(t, err, "FindByRequestID should not return error when file exists")
			assert.Contains(t, file, "req123", "FindByRequestID should return file with matching request ID")
		})

		t.Run("InvalidGlobPattern", func(t *testing.T) {
			// Create an address with an invalid glob pattern
			invalidAddr := Address{
				dagName:       "invalid",
				prefix:        "invalid",
				executionsDir: filepath.Join(tmpDir, "invalid"),
				globPattern:   "[", // Invalid glob pattern
			}

			_, err := s.FindByRequestID(ctx, invalidAddr, "req123")
			assert.Error(t, err, "FindByRequestID should handle invalid glob patterns gracefully")
		})
	})

	t.Run("RemoveOld", func(t *testing.T) {
		t.Parallel()

		// Setup test directory with files
		tmpDir := t.TempDir()
		setupTestFiles(t, tmpDir)

		s := New()
		ctx := context.Background()
		a := NewAddress(tmpDir, "test-dag")

		t.Run("NoFiles", func(t *testing.T) {
			t.Parallel()
			emptyDir := t.TempDir()
			emptyAddr := NewAddress(emptyDir, "empty-dag")

			err := s.RemoveOld(ctx, emptyAddr, 7)
			assert.NoError(t, err, "RemoveOld should not return error when no files exist")
		})

		t.Run("RemoveOldFiles", func(t *testing.T) {
			// Set file modification times to be old
			setOldFileModTime(t, tmpDir)

			err := s.RemoveOld(ctx, a, 1) // Remove files older than 1 day
			assert.NoError(t, err, "RemoveOld should not return error")

			// Verify files were removed
			files, err := filepath.Glob(a.globPattern)
			assert.NoError(t, err)
			assert.Empty(t, files, "RemoveOld should remove old files")
		})

		t.Run("InvalidGlobPattern", func(t *testing.T) {
			// Create an address with an invalid glob pattern
			invalidAddr := Address{
				dagName:       "invalid",
				prefix:        "invalid",
				executionsDir: filepath.Join(tmpDir, "invalid"),
				globPattern:   "[", // Invalid glob pattern
			}

			err := s.RemoveOld(ctx, invalidAddr, 7)
			assert.Error(t, err, "RemoveOld should handle invalid glob patterns gracefully")
		})

		t.Run("ContextCancellation", func(t *testing.T) {
			// Create a canceled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Setup many files to ensure the context cancellation is triggered
			manyFilesDir := t.TempDir()
			setupManyTestFiles(t, manyFilesDir, 100)
			manyFilesAddr := NewAddress(manyFilesDir, "many-files-dag")

			err := s.RemoveOld(ctx, manyFilesAddr, 7)
			require.Error(t, err, "RemoveOld should handle context cancellation")
			assert.Contains(t, err.Error(), "operation canceled", "Error should indicate operation was canceled")
		})
	})

	t.Run("Rename", func(t *testing.T) {
		t.Parallel()

		// Setup test directory with files
		tmpDir := t.TempDir()
		setupTestFiles(t, tmpDir)

		s := New()
		ctx := context.Background()
		oldAddr := NewAddress(tmpDir, "test-dag")
		newAddr := NewAddress(tmpDir, "new-dag")

		t.Run("NoSourceFiles", func(t *testing.T) {
			t.Parallel()
			emptyDir := t.TempDir()
			emptyAddr := NewAddress(emptyDir, "empty-dag")
			newEmptyAddr := NewAddress(emptyDir, "new-empty-dag")

			err := s.Rename(ctx, emptyAddr, newEmptyAddr)
			assert.NoError(t, err, "Rename should not return error when source has no files")
		})

		t.Run("SourceDoesNotExist", func(t *testing.T) {
			t.Parallel()
			nonExistentDir := filepath.Join(t.TempDir(), "non-existent")
			nonExistentAddr := NewAddress(nonExistentDir, "non-existent-dag")
			targetAddr := NewAddress(t.TempDir(), "target-dag")

			err := s.Rename(ctx, nonExistentAddr, targetAddr)
			assert.NoError(t, err, "Rename should not return error when source does not exist")
		})

		t.Run("SuccessfulRename", func(t *testing.T) {
			err := s.Rename(ctx, oldAddr, newAddr)
			assert.NoError(t, err, "Rename should not return error")

			// Verify old files were moved
			oldFiles, err := filepath.Glob(oldAddr.globPattern)
			assert.NoError(t, err)
			assert.Empty(t, oldFiles, "Old files should be removed after rename")

			// Verify new files exist
			newFiles, err := filepath.Glob(newAddr.globPattern)
			assert.NoError(t, err)
			assert.NotEmpty(t, newFiles, "New files should exist after rename")
		})

		t.Run("InvalidGlobPattern", func(t *testing.T) {
			// Create an address with an invalid glob pattern
			invalidAddr := Address{
				dagName:       "invalid",
				prefix:        "invalid",
				executionsDir: filepath.Join(tmpDir, "invalid"),
				globPattern:   "[", // Invalid glob pattern
			}

			// Create the directory to ensure it exists
			err := os.MkdirAll(invalidAddr.executionsDir, 0755)
			require.NoError(t, err)

			err = s.Rename(ctx, invalidAddr, newAddr)
			assert.Error(t, err, "Rename should handle invalid glob patterns gracefully")
		})

		t.Run("ContextCancellation", func(t *testing.T) {
			// Create a canceled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Setup many files to ensure the context cancellation is triggered
			manyFilesDir := t.TempDir()
			setupManyTestFiles(t, manyFilesDir, 100)
			manyFilesAddr := NewAddress(manyFilesDir, "many-files-dag")
			newManyFilesAddr := NewAddress(manyFilesDir, "new-many-files-dag")

			err := s.Rename(ctx, manyFilesAddr, newManyFilesAddr)
			assert.Error(t, err, "Rename should handle context cancellation")
			assert.Contains(t, err.Error(), "operation canceled", "Error should indicate operation was canceled")
		})
	})

	// Helper function tests
	t.Run("parseFileTimestamp", func(t *testing.T) {
		t.Parallel()

		t.Run("ValidTimestamp", func(t *testing.T) {
			file := "/path/to/test-dag_20230415_123045_000Z_req123/status.dat"
			timestamp, err := parseFileTimestamp(file)
			assert.NoError(t, err, "parseFileTimestamp should not return error for valid timestamp")

			expected := time.Date(2023, 4, 15, 12, 30, 45, 0, time.UTC)
			assert.Equal(t, expected, timestamp, "parseFileTimestamp should parse timestamp correctly")
		})

		t.Run("NoTimestamp", func(t *testing.T) {
			file := "/path/to/test-dag_invalid_timestamp/status.dat"
			_, err := parseFileTimestamp(file)
			assert.Error(t, err, "parseFileTimestamp should return error when no timestamp is found")
			assert.Contains(t, err.Error(), "no timestamp found", "Error should indicate no timestamp was found")
		})

		t.Run("InvalidTimestamp", func(t *testing.T) {
			// This matches the regex but is not a valid timestamp
			file := "/path/to/test-dag_20230415_999999_999Z_req123/status.dat"
			_, err := parseFileTimestamp(file)
			assert.Error(t, err, "parseFileTimestamp should return error for invalid timestamp")
			assert.Contains(t, err.Error(), "failed to parse UTC timestamp", "Error should indicate timestamp parsing failed")
		})
	})

	t.Run("filterLatest", func(t *testing.T) {
		t.Parallel()

		t.Run("EmptyFiles", func(t *testing.T) {
			result := filterLatest(nil, 10, 1)
			assert.Nil(t, result, "filterLatest should return nil for empty files")
		})

		t.Run("LimitExceedsFileCount", func(t *testing.T) {
			// Create temporary files with valid timestamps
			tmpDir := t.TempDir()
			setupTestFiles(t, tmpDir)

			// Get the actual files
			pattern := filepath.Join(tmpDir, "test-dag", "executions", "2*", "*", "*", "*", "status.dat")
			files, err := filepath.Glob(pattern)
			require.NoError(t, err)
			require.NotEmpty(t, files)

			result := filterLatest(files, 5, 1)
			assert.Len(t, result, 3, "filterLatest should return all files when limit exceeds file count")
		})

		t.Run("LimitLessThanFileCount", func(t *testing.T) {
			// Create temporary files with valid timestamps
			tmpDir := t.TempDir()
			setupTestFiles(t, tmpDir)

			// Get the actual files
			pattern := filepath.Join(tmpDir, "test-dag", "executions", "20*", "*", "*", "*", "status.dat")
			files, err := filepath.Glob(pattern)
			require.NoError(t, err)
			require.NotEmpty(t, files)

			result := filterLatest(files, 2, 1)
			assert.Len(t, result, 2, "filterLatest should return at most itemLimit files")

			// Verify files are sorted by timestamp (most recent first)
			assert.Contains(t, result[0], "20230415_123045")
			assert.Contains(t, result[1], "20230415_123030")
		})

		t.Run("InvalidTimestamps", func(t *testing.T) {
			// Create a mix of valid and invalid files
			tmpDir := t.TempDir()
			setupTestFiles(t, tmpDir)

			// Create an invalid file
			invalidDir := filepath.Join(tmpDir, "test-dag", "test-dag_invalid_timestamp_req999")
			err := os.MkdirAll(invalidDir, 0755)
			require.NoError(t, err)

			invalidPath := filepath.Join(invalidDir, "status.dat")
			err = os.WriteFile(invalidPath, []byte(`{"Status": "error"}`), 0644)
			require.NoError(t, err)

			// Get all files including the invalid one
			pattern := filepath.Join(tmpDir, "test-dag", "executions", "20*", "*", "*", "*", "status.dat")
			files, err := filepath.Glob(pattern)
			require.NoError(t, err)
			require.NotEmpty(t, files)

			result := filterLatest(files, 4, 1)
			assert.Len(t, result, 3, "filterLatest should skip files with invalid timestamps")

			// Verify only valid files are included
			for _, file := range result {
				assert.True(t, strings.Contains(file, "20230415_"), "Only files with valid timestamps should be included")
			}
		})

		t.Run("DefaultMaxWorkers", func(t *testing.T) {
			// Create temporary files with valid timestamps
			tmpDir := t.TempDir()
			setupTestFiles(t, tmpDir)

			// Get the actual files
			pattern := filepath.Join(tmpDir, "test-dag", "executions", "20*", "*", "*", "*", "status.dat")
			files, err := filepath.Glob(pattern)
			require.NoError(t, err)
			require.NotEmpty(t, files)

			result := filterLatest(files, 2, 0) // 0 should use default (NumCPU)
			assert.Len(t, result, 2, "filterLatest should use default maxWorkers when 0 is provided")
		})
	})

	t.Run("processFilesParallel", func(t *testing.T) {
		t.Parallel()

		t.Run("EmptyFiles", func(t *testing.T) {
			ctx := context.Background()
			errs := processFilesParallel(ctx, nil, func(string) error {
				return nil
			})
			assert.Empty(t, errs, "processFilesParallel should return empty errors for empty files")
		})

		t.Run("NoErrors", func(t *testing.T) {
			ctx := context.Background()
			files := []string{"file1", "file2", "file3"}

			errs := processFilesParallel(ctx, files, func(string) error {
				return nil
			})
			assert.Empty(t, errs, "processFilesParallel should return empty errors when no errors occur")
		})

		t.Run("WithErrors", func(t *testing.T) {
			ctx := context.Background()
			files := []string{"file1", "file2", "file3"}

			errs := processFilesParallel(ctx, files, func(file string) error {
				if file == "file2" {
					return errors.New("test error")
				}
				return nil
			})
			assert.Len(t, errs, 1, "processFilesParallel should return errors that occur")
			assert.Contains(t, errs[0].Error(), "test error", "Error message should be preserved")
		})

		t.Run("ContextCancellation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			files := []string{"file1", "file2", "file3"}

			errs := processFilesParallel(ctx, files, func(string) error {
				return nil
			})
			assert.Len(t, errs, 1, "processFilesParallel should return error for canceled context")
			assert.Contains(t, errs[0].Error(), "operation canceled", "Error should indicate operation was canceled")
		})
	})
}

// Helper functions for tests

// setupTestFiles creates test files for testing
func setupTestFiles(t *testing.T, dir string) {
	t.Helper()

	// Create test-dag directory
	baseDir := filepath.Join(dir, "test-dag")
	err := os.MkdirAll(baseDir, 0755)
	require.NoError(t, err)

	// Create test files with different timestamps
	timestamps := []struct {
		time   string
		reqID  string
		status string
	}{
		{"20230415_123045_000Z", "req123", "running"},
		{"20230415_123030_000Z", "req456", "success"},
		{"20230415_123015_000Z", "req789", "error"},
	}

	for _, ts := range timestamps {
		timestamp, err := time.Parse(dateTimeFormatUTC, ts.time)
		require.NoError(t, err)
		fileDir := filepath.Join(baseDir,
			"executions",
			timestamp.Format("2006"),
			timestamp.Format("01"),
			timestamp.Format("02"),
			fmt.Sprintf("exec_%s_%s", ts.time, ts.reqID))
		err = os.MkdirAll(fileDir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(fileDir, "status.dat")
		err = os.WriteFile(filePath, []byte(fmt.Sprintf(`{"Status": "%s"}`, ts.status)), 0644)
		require.NoError(t, err)
	}
}

// setupManyTestFiles creates many test files for testing parallel processing
func setupManyTestFiles(t *testing.T, dir string, count int) {
	t.Helper()

	// Create test-dag directory
	baseDir := filepath.Join(dir, "many-files-dag")
	err := os.MkdirAll(baseDir, 0755)
	require.NoError(t, err)

	// Create test files with different timestamps
	for i := 0; i < count; i++ {
		ts := time.Now().Add(time.Duration(-i) * time.Minute).UTC()
		tsStr := ts.Format(dateTimeFormatUTC)
		reqID := fmt.Sprintf("req%d", i)

		fileDir := filepath.Join(baseDir,
			"executions",
			ts.Format("2006"),
			ts.Format("01"),
			ts.Format("02"),
			fmt.Sprintf("exec_%s_%s", tsStr, reqID))
		err := os.MkdirAll(fileDir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(fileDir, "status.dat")
		err = os.WriteFile(filePath, []byte(fmt.Sprintf(`{"Status": "running", "RequestId": "%s"}`, reqID)), 0644)
		require.NoError(t, err)
	}
}

// setOldFileModTime sets file modification times to be old
func setOldFileModTime(t *testing.T, dir string) {
	t.Helper()

	// Find all status.dat files
	pattern := filepath.Join(dir, "test-dag", "executions", "2*", "*", "*", "*", "status.dat")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)

	// Set modification time to be old
	oldTime := time.Now().AddDate(0, 0, -30) // 30 days old
	for _, file := range files {
		err := os.Chtimes(file, oldTime, oldTime)
		require.NoError(t, err)

		// Also set the directory time
		dir := filepath.Dir(file)
		err = os.Chtimes(dir, oldTime, oldTime)
		require.NoError(t, err)
	}
}
