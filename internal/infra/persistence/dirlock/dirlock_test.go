package dirlock

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("ValidDirectory", func(t *testing.T) {
		lock := New("/tmp/test", nil)
		require.NotNil(t, lock)
	})

	t.Run("DefaultOptions", func(t *testing.T) {
		lock := New("/tmp/test", nil)

		dl := lock.(*dirLock)
		require.Equal(t, 30*time.Second, dl.opts.StaleThreshold)
		require.Equal(t, 50*time.Millisecond, dl.opts.RetryInterval)
	})

	t.Run("CustomOptions", func(t *testing.T) {
		opts := &LockOptions{
			StaleThreshold: 10 * time.Second,
			RetryInterval:  100 * time.Millisecond,
		}
		lock := New("/tmp/test", opts)

		dl := lock.(*dirLock)
		require.Equal(t, 10*time.Second, dl.opts.StaleThreshold)
		require.Equal(t, 100*time.Millisecond, dl.opts.RetryInterval)
	})
}

func TestTryLock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("AcquireLockSuccessfully", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.TryLock()
		require.NoError(t, err)
		require.True(t, lock.IsHeldByMe())
		require.True(t, lock.IsLocked())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("LockConflict", func(t *testing.T) {
		lock1 := New(tmpDir, nil)
		lock2 := New(tmpDir, nil)

		// First lock succeeds
		err := lock1.TryLock()
		require.NoError(t, err)

		// Second lock fails
		err = lock2.TryLock()
		require.ErrorIs(t, err, ErrLockConflict)
		require.False(t, lock2.IsHeldByMe())

		// Cleanup
		err = lock1.Unlock()
		require.NoError(t, err)
	})

	t.Run("ReacquireAfterUnlock", func(t *testing.T) {
		lock := New(tmpDir, nil)

		// Acquire
		err := lock.TryLock()
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

	t.Run("AcquireImmediately", func(t *testing.T) {
		lock := New(tmpDir, nil)

		ctx := context.Background()
		err := lock.Lock(ctx)
		require.NoError(t, err)
		require.True(t, lock.IsHeldByMe())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("WaitForLock", func(t *testing.T) {
		lock1 := New(tmpDir, &LockOptions{
			RetryInterval: 10 * time.Millisecond,
		})

		lock2 := New(tmpDir, &LockOptions{
			RetryInterval: 10 * time.Millisecond,
		})

		// First lock acquired
		err := lock1.TryLock()
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

	t.Run("ContextCancellation", func(t *testing.T) {
		lock1 := New(tmpDir, nil)
		lock2 := New(tmpDir, nil)

		// First lock acquired
		err := lock1.TryLock()
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

	t.Run("UnlockHeldLock", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.TryLock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)
		require.False(t, lock.IsHeldByMe())
		require.False(t, lock.IsLocked())
	})

	t.Run("UnlockNotHeld", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("DoubleUnlock", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.TryLock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)
	})
}

func TestIsLocked(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("NoLock", func(t *testing.T) {
		lock := New(tmpDir, nil)
		require.False(t, lock.IsLocked())
	})

	t.Run("WithLock", func(t *testing.T) {
		lock1 := New(tmpDir, nil)
		lock2 := New(tmpDir, nil)

		err := lock1.TryLock()
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

	t.Run("CleanStaleLock", func(t *testing.T) {
		// Create a stale lock manually
		lockPath := filepath.Join(tmpDir, ".dagu_lock")
		err := os.Mkdir(lockPath, 0700)
		require.NoError(t, err)

		// Set modification time to past
		pastTime := time.Now().Add(-60 * time.Second)
		err = os.Chtimes(lockPath, pastTime, pastTime)
		require.NoError(t, err)

		lock := New(tmpDir, &LockOptions{
			StaleThreshold: 30 * time.Second,
		})

		// TryLock should clean up stale lock and succeed
		err = lock.TryLock()
		require.NoError(t, err)

		// Verify lock is held
		require.True(t, lock.IsHeldByMe())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})
}

func TestForceUnlock(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("ForceUnlockExistingLock", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.TryLock()
		require.NoError(t, err)
		require.True(t, lock.IsLocked())

		err = ForceUnlock(tmpDir)
		require.NoError(t, err)
		require.False(t, lock.IsLocked())
	})

	t.Run("ForceUnlockEmptyDirectory", func(t *testing.T) {
		err := ForceUnlock(tmpDir)
		require.NoError(t, err)
	})
}

func TestConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("MultipleGoroutinesCompeting", func(t *testing.T) {
		const numGoroutines = 10
		const numIterations = 5

		var wg sync.WaitGroup
		successCount := make([]int, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				lock := New(tmpDir, &LockOptions{
					RetryInterval: 5 * time.Millisecond,
				})

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

func TestHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("HeartbeatUpdatesLockTimestamp", func(t *testing.T) {
		lock := New(tmpDir, nil)

		// Acquire lock
		err := lock.TryLock()
		require.NoError(t, err)

		// Get initial lock info
		info1, err := lock.Info()
		require.NoError(t, err)
		require.NotNil(t, info1)
		initialTime := info1.AcquiredAt

		// Wait a bit to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Heartbeat
		err = lock.Heartbeat(context.Background())
		require.NoError(t, err)

		// Get updated lock info
		info2, err := lock.Info()
		require.NoError(t, err)
		require.NotNil(t, info2)

		// Verify timestamp was updated
		require.True(t, info2.AcquiredAt.After(initialTime))

		// Verify lock is still held
		require.True(t, lock.IsHeldByMe())

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("HeartbeatWithoutLockFails", func(t *testing.T) {
		lock := New(tmpDir, nil)

		err := lock.Heartbeat(context.Background())
		require.ErrorIs(t, err, ErrNotLocked)
	})

	t.Run("ConcurrentHeartbeatAndCheck", func(t *testing.T) {
		// Use a different temp dir to avoid conflicts
		isolatedDir := t.TempDir()
		lock := New(isolatedDir, nil)

		err := lock.TryLock()
		require.NoError(t, err)

		// Run heartbeat and checks concurrently
		done := make(chan bool)
		errCh := make(chan error, 1)
		go func() {
			for i := 0; i < 5; i++ {
				err := lock.Heartbeat(context.Background())
				if err != nil {
					errCh <- err
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			done <- true
		}()

		// Check lock status while heartbeat is running
		for i := 0; i < 5; i++ {
			require.True(t, lock.IsLocked())
			require.True(t, lock.IsHeldByMe())
			time.Sleep(5 * time.Millisecond)
		}

		select {
		case err := <-errCh:
			t.Fatalf("Heartbeat failed: %v", err)
		case <-done:
			// Success
		}

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("NonExistentDirectory", func(t *testing.T) {
		nonExistentDir := filepath.Join(t.TempDir(), "non-existent")
		lock := New(nonExistentDir, nil)

		// Should succeed and create the directory
		err := lock.TryLock()
		require.NoError(t, err)

		// Verify directory was created
		_, err = os.Stat(nonExistentDir)
		require.NoError(t, err)

		err = lock.Unlock()
		require.NoError(t, err)
	})

	t.Run("InfoReturnsCorrectData", func(t *testing.T) {
		tmpDir := t.TempDir()
		lock := New(tmpDir, nil)

		// No lock initially
		info, err := lock.Info()
		require.NoError(t, err)
		require.Nil(t, info)

		// Acquire lock
		err = lock.TryLock()
		require.NoError(t, err)

		// Get info
		info, err = lock.Info()
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, ".dagu_lock", info.LockDirName)
		require.WithinDuration(t, time.Now(), info.AcquiredAt, 1*time.Second)

		// Cleanup
		err = lock.Unlock()
		require.NoError(t, err)

		// No lock after unlock
		info, err = lock.Info()
		require.NoError(t, err)
		require.Nil(t, info)
	})
}
