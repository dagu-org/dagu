package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic(t *testing.T) {
	t.Parallel()

	t.Run("writes file successfully", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		content := []byte("hello world")

		err := WriteFileAtomic(filePath, content, 0600)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, content, data)

		// Verify temp file was cleaned up
		_, err = os.Stat(filePath + ".tmp")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")

		require.NoError(t, os.WriteFile(filePath, []byte("old"), 0600))

		err := WriteFileAtomic(filePath, []byte("new"), 0600)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, []byte("new"), data)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		t.Parallel()
		err := WriteFileAtomic("/nonexistent/dir/file.txt", []byte("data"), 0600)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write temp file")
	})

	t.Run("sets correct permissions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")

		err := WriteFileAtomic(filePath, []byte("data"), 0644)
		require.NoError(t, err)

		info, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})
}

func TestWriteJSONAtomic(t *testing.T) {
	t.Parallel()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	t.Run("writes JSON successfully", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")
		data := testData{Name: "test", Value: 42}

		err := WriteJSONAtomic(filePath, data, 0600)
		require.NoError(t, err)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		expected := "{\n  \"name\": \"test\",\n  \"value\": 42\n}"
		assert.Equal(t, expected, string(content))
	})

	t.Run("writes nil as null", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")

		err := WriteJSONAtomic(filePath, nil, 0600)
		require.NoError(t, err)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "null", string(content))
	})

	t.Run("returns error for unmarshalable type", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")

		err := WriteJSONAtomic(filePath, make(chan int), 0600)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal JSON")
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		t.Parallel()
		err := WriteJSONAtomic("/nonexistent/dir/file.json", "data", 0600)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write temp file")
	})
}
