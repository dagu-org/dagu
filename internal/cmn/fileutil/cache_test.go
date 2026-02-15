package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	assert.Equal(t, "test", cache.Name())
	assert.Equal(t, 0, cache.Size())
}

func TestCache_StoreAndLoad(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	// Create a temp file for testing
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	// Store and verify
	cache.Store(filePath, "test-data", fi)
	assert.Equal(t, 1, cache.Size())

	// Load and verify
	data, ok := cache.Load(filePath)
	assert.True(t, ok)
	assert.Equal(t, "test-data", data)

	// Load non-existent
	_, ok = cache.Load("non-existent")
	assert.False(t, ok)
}

func TestCache_Invalidate(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	cache.Store(filePath, "test-data", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size())

	_, ok := cache.Load(filePath)
	assert.False(t, ok)
}

func TestCache_CapacityLimit(t *testing.T) {
	t.Parallel()

	// Create cache with capacity of 5
	cache := NewCache[string]("test", 5, time.Hour)

	tmpDir := t.TempDir()

	// Add 10 items — LRU enforces capacity immediately on Add
	for i := range 10 {
		filePath := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		fi, err := os.Stat(filePath)
		require.NoError(t, err)

		cache.Store(filePath, "data", fi)
	}

	// LRU enforces capacity on Add, so size is capped at 5
	assert.Equal(t, 5, cache.Size())
}

func TestCache_TTLExpiration(t *testing.T) {
	t.Parallel()

	// Use a very short TTL
	cache := NewCache[string]("test", 100, 100*time.Millisecond)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	cache.Store(filePath, "test-data", fi)
	assert.Equal(t, 1, cache.Size())

	// Verify the entry is accessible before expiration
	data, ok := cache.Load(filePath)
	assert.True(t, ok)
	assert.Equal(t, "test-data", data)

	// Wait for TTL to expire (LRU sweeps every ttl/100 = 1ms)
	time.Sleep(200 * time.Millisecond)

	// Entry should be expired
	_, ok = cache.Load(filePath)
	assert.False(t, ok)
	assert.Equal(t, 0, cache.Size())
}

func TestCache_IsStale(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	cache.Store(filePath, "test-data", fi)

	t.Run("NotStaleWhenUnchanged", func(t *testing.T) {
		stale, _, err := cache.IsStale(filePath)
		require.NoError(t, err)
		assert.False(t, stale)
	})

	t.Run("StaleWhenModified", func(t *testing.T) {
		// Wait a bit and modify the file
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, os.WriteFile(filePath, []byte("new content"), 0644))

		stale, _, err := cache.IsStale(filePath)
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("StaleWhenSizeChanged", func(t *testing.T) {
		// Store with current state
		fi2, _ := os.Stat(filePath)
		cache.Store(filePath, "data", fi2)

		// Change file size
		require.NoError(t, os.WriteFile(filePath, []byte("much longer content here"), 0644))

		stale, _, err := cache.IsStale(filePath)
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("StaleWhenNotCached", func(t *testing.T) {
		otherPath := filepath.Join(tmpDir, "other.txt")
		require.NoError(t, os.WriteFile(otherPath, []byte("content"), 0644))

		stale, _, err := cache.IsStale(otherPath)
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("ErrorWhenFileNotExist", func(t *testing.T) {
		stale, _, err := cache.IsStale(filepath.Join(tmpDir, "nonexistent.txt"))
		assert.True(t, stale)
		assert.Error(t, err)
	})
}

func TestCache_LoadLatest(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	loadCount := 0
	loader := func() (string, error) {
		loadCount++
		return "loaded-data", nil
	}

	// First call should invoke loader
	data, err := cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 1, loadCount)

	// Second call should use cache (file unchanged)
	data, err = cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 1, loadCount) // Loader not called again

	// Modify file
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(filePath, []byte("new content"), 0644))

	// Third call should invoke loader again
	data, err = cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 2, loadCount)
}

func TestCache_LoadLatest_FileNotFound(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	loader := func() (string, error) {
		return "data", nil
	}

	// LoadLatest on a non-existent file should return an error
	_, err := cache.LoadLatest("/nonexistent/path/file.txt", loader)
	assert.Error(t, err)
}

func TestCache_LoadLatest_LoaderError(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	loaderErr := fmt.Errorf("loader failed")
	loader := func() (string, error) {
		return "", loaderErr
	}

	// LoadLatest should propagate the loader error
	_, err := cache.LoadLatest(filePath, loader)
	assert.ErrorIs(t, err, loaderErr)

	// Cache should remain empty after loader failure
	assert.Equal(t, 0, cache.Size())
}

func TestCache_StartEviction(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	ctx := t.Context()

	// StartEviction is a no-op but should not panic
	cache.StartEviction(ctx)
}

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cache := NewCache[int]("test", 1000, time.Hour)

	tmpDir := t.TempDir()

	// Create test files
	const numFiles = 100
	files := make([]string, numFiles)
	for i := range numFiles {
		files[i] = filepath.Join(tmpDir, "test"+string(rune('a'+i%26))+string(rune('0'+i/26))+".txt")
		require.NoError(t, os.WriteFile(files[i], []byte("content"), 0644))
	}

	// Concurrent writes
	done := make(chan bool)
	for i := range numFiles {
		go func(idx int) {
			fi, _ := os.Stat(files[idx])
			cache.Store(files[idx], idx, fi)
			done <- true
		}(i)
	}

	for range numFiles {
		<-done
	}

	assert.Equal(t, numFiles, cache.Size())

	// Concurrent reads
	for i := range numFiles {
		go func(idx int) {
			_, _ = cache.Load(files[idx])
			done <- true
		}(i)
	}

	for range numFiles {
		<-done
	}
}

func TestCache_ZeroCapacityMeansUnlimited(t *testing.T) {
	t.Parallel()

	// Create cache with zero capacity (unlimited)
	cache := NewCache[string]("test", 0, time.Hour*24)

	tmpDir := t.TempDir()

	// Add many items
	for i := range 100 {
		filePath := filepath.Join(tmpDir, "file"+string(rune('0'+i/10))+string(rune('0'+i%10))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		fi, err := os.Stat(filePath)
		require.NoError(t, err)

		cache.Store(filePath, "data", fi)
	}

	// All items should be stored (no capacity eviction)
	assert.Equal(t, 100, cache.Size())
}

func TestCacheMetrics_Interface(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("metrics-test", 50, time.Hour)

	// Verify it implements CacheMetrics interface
	var metrics CacheMetrics = cache

	assert.Equal(t, "metrics-test", metrics.Name())
	assert.Equal(t, 0, metrics.Size())

	// Add an item
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
	fi, _ := os.Stat(filePath)
	cache.Store(filePath, "data", fi)

	assert.Equal(t, 1, metrics.Size())
}

func TestCache_StoreSameKeyTwice(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	// Store same key twice
	cache.Store(filePath, "first", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Store(filePath, "second", fi)
	assert.Equal(t, 1, cache.Size(), "items counter should not increment for existing key")

	// Verify the value was updated
	data, ok := cache.Load(filePath)
	assert.True(t, ok)
	assert.Equal(t, "second", data)
}

func TestCache_InvalidateNonExistent(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	// Invalidate a key that doesn't exist
	cache.Invalidate("non-existent")

	assert.Equal(t, 0, cache.Size(), "items counter should stay at 0")

	// Add one item, then invalidate twice
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
	fi, _ := os.Stat(filePath)

	cache.Store(filePath, "data", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size())

	// Invalidate same key again
	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size(), "items counter should not go negative")
}

func TestCache_MixedExpirationAndCapacity(t *testing.T) {
	t.Parallel()

	// Cache with capacity of 3 and short TTL
	cache := NewCache[string]("test", 3, 100*time.Millisecond)

	tmpDir := t.TempDir()

	// Add 2 entries that will expire
	for i := range 2 {
		filePath := filepath.Join(tmpDir, "expire"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
		fi, _ := os.Stat(filePath)
		cache.Store(filePath, "expires", fi)
	}
	assert.Equal(t, 2, cache.Size())

	// Wait for them to expire
	time.Sleep(200 * time.Millisecond)

	// Add 4 more entries (capacity is 3, but 2 expired so should fit 3)
	for i := range 4 {
		filePath := filepath.Join(tmpDir, "valid"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
		fi, _ := os.Stat(filePath)
		cache.Store(filePath, "valid", fi)
	}

	// Should be at most capacity (3)
	assert.LessOrEqual(t, cache.Size(), 3)
}

func TestCache_LRUEvictionOrder(t *testing.T) {
	t.Parallel()

	// Capacity of 3
	cache := NewCache[string]("test", 3, time.Hour)

	tmpDir := t.TempDir()

	// Create 4 files
	var files []string
	for i := range 4 {
		fp := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(fp, []byte("content"), 0644))
		files = append(files, fp)
	}

	// Add first 3 entries
	for i := range 3 {
		fi, _ := os.Stat(files[i])
		cache.Store(files[i], "data-"+string(rune('0'+i)), fi)
	}
	assert.Equal(t, 3, cache.Size())

	// Access file0 to make it recently used
	_, _ = cache.Load(files[0])

	// Add file3 — should evict file1 (least recently used, since file0 was accessed)
	fi, _ := os.Stat(files[3])
	cache.Store(files[3], "data-3", fi)

	assert.Equal(t, 3, cache.Size())

	// file0 should still be present (was accessed recently)
	_, ok := cache.Load(files[0])
	assert.True(t, ok, "file0 should still be cached (recently accessed)")

	// file1 should have been evicted (least recently used)
	_, ok = cache.Load(files[1])
	assert.False(t, ok, "file1 should have been evicted (LRU)")

	// file2 should still be present
	_, ok = cache.Load(files[2])
	assert.True(t, ok, "file2 should still be cached")

	// file3 should be present
	_, ok = cache.Load(files[3])
	assert.True(t, ok, "file3 should be cached (just added)")
}

func TestCache_TTLEvictionAutomatic(t *testing.T) {
	t.Parallel()

	// Short TTL - the LRU sweeps every ttl/100
	cache := NewCache[string]("test", 100, 100*time.Millisecond)

	tmpDir := t.TempDir()

	// Add entries
	for i := range 5 {
		fp := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(fp, []byte("content"), 0644))
		fi, _ := os.Stat(fp)
		cache.Store(fp, "data", fi)
	}
	assert.Equal(t, 5, cache.Size())

	// Wait for TTL expiration + cleanup sweep
	time.Sleep(200 * time.Millisecond)

	// Entries should have been automatically evicted (no manual trigger needed)
	assert.Equal(t, 0, cache.Size())
}
