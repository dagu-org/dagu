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

func TestMemoryStore_Prune(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStore(50 * time.Millisecond)

		store.Add(1.0, 1.0, 1.0, 1.0)

		// Wait for timestamp change
		time.Sleep(1100 * time.Millisecond)

		// Force prune by setting lastPruned to past
		store.lastPruned = time.Now().Add(-2 * time.Minute)

		// This Add triggers pruning
		store.Add(2.0, 2.0, 2.0, 2.0)

		store.mu.RLock()
		cpuLen := len(store.cpu)
		store.mu.RUnlock()

		assert.Equal(t, 1, cpuLen, "old data should be pruned")
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

func TestShiftSlice(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	// Shift first 2 elements
	points := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now - 10, Value: 3.0},
		{Timestamp: now, Value: 4.0},
	}
	result := shiftSlice(points, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, 3.0, result[0].Value)
	assert.Equal(t, 4.0, result[1].Value)

	// Shift all (idx = len)
	points2 := []MetricPoint{
		{Timestamp: now, Value: 1.0},
	}
	result = shiftSlice(points2, 1)
	assert.Len(t, result, 0)

	// Shift none (idx = 0)
	points3 := []MetricPoint{
		{Timestamp: now - 10, Value: 1.0},
		{Timestamp: now, Value: 2.0},
	}
	result = shiftSlice(points3, 0)
	assert.Len(t, result, 2)
	assert.Equal(t, 1.0, result[0].Value)
}
