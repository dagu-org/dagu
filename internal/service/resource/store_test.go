package resource

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_AddAndGet(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)

	// Add distinct values to verify correct metric mapping
	store.Add(10.0, 20.0, 30.0, 1.5)
	store.Add(15.0, 25.0, 35.0, 2.0)

	history := store.GetHistory(time.Hour)

	require.Len(t, history.CPU, 2)
	require.Len(t, history.Memory, 2)
	require.Len(t, history.Disk, 2)
	require.Len(t, history.Load, 2)

	// Verify ordering
	assert.Equal(t, 10.0, history.CPU[0].Value)
	assert.Equal(t, 15.0, history.CPU[1].Value)

	// Verify correct metric assignment
	assert.Equal(t, 20.0, history.Memory[0].Value)
	assert.Equal(t, 30.0, history.Disk[0].Value)
	assert.Equal(t, 1.5, history.Load[0].Value)
}

func TestMemoryStore_GetHistoryFiltering(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStore(time.Hour)

		// Add old data
		store.Add(1.0, 1.0, 1.0, 1.0)

		// Wait 2+ seconds so old point is at least 2 seconds in the past
		time.Sleep(2100 * time.Millisecond)

		// Add new data
		store.Add(2.0, 2.0, 2.0, 2.0)

		// Long duration returns all points
		history := store.GetHistory(time.Minute)
		assert.Len(t, history.CPU, 2)

		// 1-second duration excludes old point (which is 2+ seconds old)
		historyShort := store.GetHistory(time.Second)
		require.Len(t, historyShort.CPU, 1)
		assert.Equal(t, 2.0, historyShort.CPU[0].Value)
	})
}

func TestMemoryStore_EmptyHistory(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)
	history := store.GetHistory(time.Hour)

	assert.Empty(t, history.CPU)
	assert.Empty(t, history.Memory)
	assert.Empty(t, history.Disk)
	assert.Empty(t, history.Load)
}

func TestMemoryStore_GetHistoryReturnsCopy(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)
	store.Add(1.0, 1.0, 1.0, 1.0)

	history1 := store.GetHistory(time.Hour)
	history2 := store.GetHistory(time.Hour)

	// Modify history1
	history1.CPU[0].Value = 999.0

	// history2 should not be affected
	assert.Equal(t, 1.0, history2.CPU[0].Value)
}

func TestNewMemoryStoreWithInterval_Capacity(t *testing.T) {
	t.Parallel()

	// 1 hour retention with 10s interval = 360 points + 10 buffer = 370
	store := NewMemoryStoreWithInterval(time.Hour, 10*time.Second)
	assert.Equal(t, 370, len(store.buffer.buffer))

	// Invalid interval defaults to 10s
	store2 := NewMemoryStoreWithInterval(time.Hour, 0)
	assert.Equal(t, 370, len(store2.buffer.buffer))
}
