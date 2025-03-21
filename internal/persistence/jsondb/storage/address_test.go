package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	t.Run("NewAddress", func(t *testing.T) {
		t.Parallel()

		t.Run("BasicName", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test-dag"

			addr := NewAddress(baseDir, dagName)

			assert.Equal(t, dagName, addr.dagName, "dagName should be set correctly")
			assert.Equal(t, "test-dag", addr.prefix, "prefix should be set correctly")
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "executions"), addr.executionsDir, "path should be set correctly")
			assert.Equal(t, filepath.Join(baseDir, "test-dag", "executions", "2*", "*", "*", "exec_*", "status"+dataFileExtension), addr.globPattern, "globPattern should be set correctly")
		})

		t.Run("WithYAMLExtension", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test-dag.yaml"

			addr := NewAddress(baseDir, dagName)

			assert.Equal(t, dagName, addr.dagName, "dagName should be set correctly")
			assert.Equal(t, "test-dag", addr.prefix, "prefix should have extension removed")
		})

		t.Run("WithYMLExtension", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test-dag.yml"

			addr := NewAddress(baseDir, dagName)

			assert.Equal(t, dagName, addr.dagName, "dagName should be set correctly")
			assert.Equal(t, "test-dag", addr.prefix, "prefix should have extension removed")
		})

		t.Run("WithUnsafeName", func(t *testing.T) {
			baseDir := "/tmp"
			dagName := "test/dag with spaces.yaml"

			addr := NewAddress(baseDir, dagName)

			assert.Equal(t, dagName, addr.dagName, "dagName should be set correctly")

			// Check that the prefix is sanitized (doesn't contain unsafe characters)
			// The SafeName function converts to lowercase and replaces unsafe chars with _
			sanitizedPrefix := "dag_with_spaces"
			assert.True(t, strings.HasPrefix(addr.prefix, sanitizedPrefix), "prefix should be sanitized")

			// Check that there's a hash suffix
			hashSuffix := addr.prefix[len(sanitizedPrefix):]
			assert.True(t, len(hashSuffix) > 0, "prefix should include hash")

			// The hash length might vary based on implementation, so we just check it exists
			assert.True(t, len(hashSuffix) > 1, "hash suffix should be at least 2 characters")
		})
	})

	t.Run("FilePath", func(t *testing.T) {
		t.Parallel()

		baseDir := "/tmp"
		dagName := "test-dag"
		addr := NewAddress(baseDir, dagName)

		timestamp := NewUTC(time.Date(2023, 4, 15, 12, 30, 45, 0, time.UTC))
		reqID := "req123"

		path := addr.FilePath(timestamp, reqID)

		expected := filepath.Join(baseDir, "test-dag", "executions", "2023", "04", "15", "exec_20230415_123045_000Z_req123", "status.dat")
		assert.Equal(t, expected, path, "FilePath should generate the correct path")
	})

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		t.Run("DirectoryExists", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			// Create the directory
			err := os.MkdirAll(addr.executionsDir, 0755)
			require.NoError(t, err)

			assert.True(t, addr.Exists(), "Exists should return true when directory exists")
		})

		t.Run("DirectoryDoesNotExist", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "non-existent-dag")

			assert.False(t, addr.Exists(), "Exists should return false when directory does not exist")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			err := addr.Create()
			assert.NoError(t, err, "Create should not return error")
			assert.True(t, addr.Exists(), "Directory should exist after Create")
		})

		t.Run("AlreadyExists", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			// Create the directory first
			err := os.MkdirAll(addr.executionsDir, 0755)
			require.NoError(t, err)

			// Try to create again
			err = addr.Create()
			assert.NoError(t, err, "Create should not return error when directory already exists")
		})

	})

	t.Run("IsEmpty", func(t *testing.T) {
		t.Parallel()

		t.Run("EmptyDirectory", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			// Create the directory
			err := os.MkdirAll(addr.executionsDir, 0755)
			require.NoError(t, err)

			assert.True(t, addr.IsEmpty(), "IsEmpty should return true for empty directory")
		})

		t.Run("NonEmptyDirectory", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			// Create the directory
			err := os.MkdirAll(addr.executionsDir, 0755)
			require.NoError(t, err)

			// Create a file in the directory
			err = os.WriteFile(filepath.Join(addr.executionsDir, "test.txt"), []byte("test"), 0644)
			require.NoError(t, err)

			assert.False(t, addr.IsEmpty(), "IsEmpty should return false for non-empty directory")
		})

		t.Run("NonExistentDirectory", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "non-existent-dag")

			assert.True(t, addr.IsEmpty(), "IsEmpty should return true for non-existent directory")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "test-dag")

			// Create the directory
			err := os.MkdirAll(addr.executionsDir, 0755)
			require.NoError(t, err)

			err = addr.Remove()
			assert.NoError(t, err, "Remove should not return error")
			assert.False(t, addr.Exists(), "Directory should not exist after Remove")
		})

		t.Run("NonExistentDirectory", func(t *testing.T) {
			tmpDir := t.TempDir()
			addr := NewAddress(tmpDir, "non-existent-dag")

			err := addr.Remove()
			assert.NoError(t, err, "Remove should not return error for non-existent directory")
		})

	})
}
