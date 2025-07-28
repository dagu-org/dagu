package dirlock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		lock, err := New("/tmp/test", nil)
		require.NoError(t, err)
		require.NotNil(t, lock)
	})

	t.Run("empty directory", func(t *testing.T) {
		lock, err := New("", nil)
		require.Error(t, err)
		require.Nil(t, lock)
	})

	t.Run("default options", func(t *testing.T) {
		lock, err := New("/tmp/test", nil)
		require.NoError(t, err)

		dl := lock.(*dirLock)
		require.Equal(t, 30*time.Second, dl.opts.StaleThreshold)
		require.Equal(t, 50*time.Millisecond, dl.opts.RetryInterval)
	})

	t.Run("custom options", func(t *testing.T) {
		opts := &LockOptions{
			StaleThreshold: 10 * time.Second,
			RetryInterval:  100 * time.Millisecond,
		}
		lock, err := New("/tmp/test", opts)
		require.NoError(t, err)

		dl := lock.(*dirLock)
		require.Equal(t, 10*time.Second, dl.opts.StaleThreshold)
		require.Equal(t, 100*time.Millisecond, dl.opts.RetryInterval)
	})
}

func TestTryLock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("acquire lock successfully", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock.TryLock()
		require.NoError(t, err)
		require.True(t, lock.IsHeldByMe())
		require.True(t, lock.IsLocked())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("lock conflict", func(t *testing.T) {
		lock1, err := New(tmpDir, nil)
		require.NoError(t, err)

		lock2, err := New(tmpDir, nil)
		require.NoError(t, err)

		// First lock succeeds
		err = lock1.TryLock()
		require.NoError(t, err)

		// Second lock fails
		err = lock2.TryLock()
		require.ErrorIs(t, err, ErrLockConflict)
		require.False(t, lock2.IsHeldByMe())

		// Cleanup
		err = lock1.Unlock()
		require.NoError(t, err)
	})

	t.Run("reacquire after unlock", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		// Acquire
		err = lock.TryLock()
		require.NoError(t, err)

		// Release
		err = lock.Unlock()
		require.NoError(t, err)

		// Reacquire
		err = lock.TryLock()
		require.NoError(t, err)

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})
}

func TestLock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("acquire immediately", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		ctx := context.Background()
		err = lock.Lock(ctx)
		require.NoError(t, err)
		require.True(t, lock.IsHeldByMe())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("wait for lock", func(t *testing.T) {
		lock1, err := New(tmpDir, &LockOptions{
			RetryInterval: 10 * time.Millisecond,
		})
		require.NoError(t, err)

		lock2, err := New(tmpDir, &LockOptions{
			RetryInterval: 10 * time.Millisecond,
		})
		require.NoError(t, err)

		// First lock acquired
		err = lock1.TryLock()
		require.NoError(t, err)

		// Start goroutine to release lock after delay
		released := make(chan bool)
		go func() {
			time.Sleep(30 * time.Millisecond)
			_ = lock1.Unlock()
			released <- true
		}()

		// Second lock should wait and then acquire
		ctx := context.Background()
		err = lock2.Lock(ctx)
		require.NoError(t, err)

		// Verify the lock was released before we acquired it
		select {
		case <-released:
			// Good, lock was released
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Lock was not released in time")
		}

		// Cleanup
		err = lock2.Unlock()
		require.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		lock1, err := New(tmpDir, nil)
		require.NoError(t, err)

		lock2, err := New(tmpDir, nil)
		require.NoError(t, err)

		// First lock acquired
		err = lock1.TryLock()
		require.NoError(t, err)

		// Try to acquire with context that gets cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err = lock2.Lock(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "context deadline exceeded")
		require.False(t, lock2.IsHeldByMe())

		// Cleanup
		err = lock1.Unlock()
		require.NoError(t, err)
	})
}

func TestUnlock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("unlock held lock", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock.TryLock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)
		require.False(t, lock.IsHeldByMe())
		require.False(t, lock.IsLocked())
	})

	t.Run("unlock not held", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock.Unlock()
		require.ErrorIs(t, err, ErrNotLocked)
	})

	t.Run("double unlock", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock.TryLock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.ErrorIs(t, err, ErrNotLocked)
	})
}

func TestIsLocked(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("no lock", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)
		require.False(t, lock.IsLocked())
	})

	t.Run("with lock", func(t *testing.T) {
		lock1, err := New(tmpDir, nil)
		require.NoError(t, err)

		lock2, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock1.TryLock()
		require.NoError(t, err)

		require.True(t, lock1.IsLocked())
		require.True(t, lock2.IsLocked())

		err = lock1.Unlock()
		require.NoError(t, err)

		require.False(t, lock1.IsLocked())
		require.False(t, lock2.IsLocked())
	})
}

func TestStaleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("clean stale lock", func(t *testing.T) {
		// Create a stale lock manually
		staleLockName := fmt.Sprintf(".dagu_lock.%d", time.Now().Add(-60*time.Second).UnixNano())
		staleLockPath := filepath.Join(tmpDir, staleLockName)
		err := os.Mkdir(staleLockPath, 0700)
		require.NoError(t, err)

		lock, err := New(tmpDir, &LockOptions{
			StaleThreshold: 30 * time.Second,
		})
		require.NoError(t, err)

		// TryLock should clean up stale lock and succeed
		err = lock.TryLock()
		require.NoError(t, err)

		// Verify stale lock was removed
		_, err = os.Stat(staleLockPath)
		require.True(t, os.IsNotExist(err))

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})
}

func TestForceUnlock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("force unlock existing lock", func(t *testing.T) {
		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		err = lock.TryLock()
		require.NoError(t, err)
		require.True(t, lock.IsLocked())

		err = ForceUnlock(tmpDir)
		require.NoError(t, err)
		require.False(t, lock.IsLocked())
	})

	t.Run("force unlock empty directory", func(t *testing.T) {
		err := ForceUnlock(tmpDir)
		require.NoError(t, err)
	})
}

func TestConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("multiple goroutines competing", func(t *testing.T) {
		const numGoroutines = 10
		const numIterations = 5

		var wg sync.WaitGroup
		successCount := make([]int, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				lock, err := New(tmpDir, &LockOptions{
					RetryInterval: 5 * time.Millisecond,
				})
				require.NoError(t, err)

				for j := 0; j < numIterations; j++ {
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					if err := lock.Lock(ctx); err == nil {
						successCount[id]++
						time.Sleep(2 * time.Millisecond) // Simulate work
						_ = lock.Unlock()
					}
					cancel()
				}
			}(i)
		}

		wg.Wait()

		// Verify that locks were acquired
		totalSuccess := 0
		for _, count := range successCount {
			totalSuccess += count
		}
		require.Greater(t, totalSuccess, 0)
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("non-existent directory", func(t *testing.T) {
		nonExistentDir := filepath.Join(t.TempDir(), "non-existent")
		lock, err := New(nonExistentDir, nil)
		require.NoError(t, err)

		// Should succeed and create the directory
		err = lock.TryLock()
		require.NoError(t, err)

		// Verify directory was created
		_, err = os.Stat(nonExistentDir)
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("invalid lock directory format", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create invalid lock directories
		invalidNames := []string{
			".dagu_lock",
			".dagu_lock.",
			".dagu_lock.abc",
			".dagu_lock.123.456",
		}

		for _, name := range invalidNames {
			err := os.Mkdir(filepath.Join(tmpDir, name), 0700)
			require.NoError(t, err)
		}

		lock, err := New(tmpDir, nil)
		require.NoError(t, err)

		// Should clean up invalid locks and succeed
		err = lock.TryLock()
		require.NoError(t, err)

		// Verify all invalid locks were removed
		entries, err := os.ReadDir(tmpDir)
		require.NoError(t, err)

		validLockCount := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".dagu_lock.") {
				validLockCount++
			}
		}
		require.Equal(t, 1, validLockCount)

		err = lock.Unlock()
		require.NoError(t, err)
	})
}
