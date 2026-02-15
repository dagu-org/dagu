package filebaseconfig

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("empty path returns error", func(t *testing.T) {
		_, err := New("")
		require.Error(t, err)
	})

	t.Run("creates parent directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "sub", "dir")
		filePath := filepath.Join(dir, "base.yaml")

		store, err := New(filePath)
		require.NoError(t, err)
		require.NotNil(t, store)

		info, err := os.Stat(dir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestGetSpec(t *testing.T) {
	t.Run("missing file returns empty string", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		store, err := New(filePath)
		require.NoError(t, err)

		spec, err := store.GetSpec(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "", spec)
	})

	t.Run("existing file returns content", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		content := "env:\n  - FOO=bar\n"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0600))

		store, err := New(filePath)
		require.NoError(t, err)

		spec, err := store.GetSpec(context.Background())
		require.NoError(t, err)
		assert.Equal(t, content, spec)
	})
}

func TestUpdateSpec(t *testing.T) {
	t.Run("creates file and writes content", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		store, err := New(filePath)
		require.NoError(t, err)

		content := []byte("env:\n  - FOO=bar\n")
		require.NoError(t, store.UpdateSpec(context.Background(), content))

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, content, data)
	})

	t.Run("round-trip get after update", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		store, err := New(filePath)
		require.NoError(t, err)

		content := []byte("timeout_sec: 300\n")
		require.NoError(t, store.UpdateSpec(context.Background(), content))

		spec, err := store.GetSpec(context.Background())
		require.NoError(t, err)
		assert.Equal(t, string(content), spec)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		require.NoError(t, os.WriteFile(filePath, []byte("old"), 0600))

		store, err := New(filePath)
		require.NoError(t, err)

		require.NoError(t, store.UpdateSpec(context.Background(), []byte("new")))

		spec, err := store.GetSpec(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "new", spec)
	})

	t.Run("file permissions are 0600", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "base.yaml")
		store, err := New(filePath)
		require.NoError(t, err)

		require.NoError(t, store.UpdateSpec(context.Background(), []byte("data")))

		info, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

func TestConcurrentAccess(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "base.yaml")
	store, err := New(filePath)
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			content := []byte("env:\n  - N=" + string(rune('0'+n)) + "\n")
			_ = store.UpdateSpec(ctx, content)
		}(i)
	}

	for range 10 {
		wg.Go(func() {
			_, _ = store.GetSpec(ctx)
		})
	}

	wg.Wait()

	// Verify the file is readable after concurrent access
	spec, err := store.GetSpec(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, spec)
}
