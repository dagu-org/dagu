package resource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_AddAndGet(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)

	store.Add(10.0, 20.0, 30.0, 1.5)
	store.Add(15.0, 25.0, 35.0, 2.0)

	history := store.GetHistory(time.Hour)

	assert.Len(t, history.CPU, 2)
	assert.Len(t, history.Memory, 2)
	assert.Len(t, history.Disk, 2)
	assert.Len(t, history.Load, 2)

	assert.Equal(t, 10.0, history.CPU[0].Value)
	assert.Equal(t, 15.0, history.CPU[1].Value)
}

func TestMemoryStore_GetHistoryFiltering(t *testing.T) {
	t.Parallel()

	// Short retention for testing
	store := NewMemoryStore(100 * time.Millisecond)

	// Add old data
	store.Add(1.0, 1.0, 1.0, 1.0)

	// Wait for retention period and ensure timestamp change (Unix seconds)
	time.Sleep(1100 * time.Millisecond)

	// Add new data
	store.Add(2.0, 2.0, 2.0, 2.0)

	// Get history for a duration longer than retention - both points within the window
	history := store.GetHistory(time.Minute)
	assert.Len(t, history.CPU, 2)

	// Now test GetHistory with short duration - only recent data
	historyShort := store.GetHistory(50 * time.Millisecond)
	assert.Len(t, historyShort.CPU, 1)
	assert.Equal(t, 2.0, historyShort.CPU[0].Value)
}

func TestMemoryStore_Prune(t *testing.T) {
	t.Parallel()

	// Create store with short retention
	store := NewMemoryStore(50 * time.Millisecond)

	// Add initial data
	store.Add(1.0, 1.0, 1.0, 1.0)

	// Wait for timestamp to change (Unix seconds granularity)
	time.Sleep(1100 * time.Millisecond)

	// Force prune by setting lastPruned to past
	store.lastPruned = time.Now().Add(-2 * time.Minute)

	// Add new data - this should trigger pruning
	store.Add(2.0, 2.0, 2.0, 2.0)

	// The old data should be pruned from internal storage
	// We can verify by checking the internal slice lengths
	store.mu.RLock()
	cpuLen := len(store.cpu)
	store.mu.RUnlock()

	// Should only have the new data point after pruning
	assert.Equal(t, 1, cpuLen, "old data should be pruned")
}

func TestMemoryStore_EmptyHistory(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)

	history := store.GetHistory(time.Hour)

	assert.Nil(t, history.CPU)
	assert.Nil(t, history.Memory)
	assert.Nil(t, history.Disk)
	assert.Nil(t, history.Load)
}

func TestMemoryStore_GetHistoryReturnsCorrectMetrics(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)

	// Add distinct values to verify correct metric mapping
	store.Add(10.0, 20.0, 30.0, 40.0)

	history := store.GetHistory(time.Hour)

	require.Len(t, history.CPU, 1)
	require.Len(t, history.Memory, 1)
	require.Len(t, history.Disk, 1)
	require.Len(t, history.Load, 1)

	assert.Equal(t, 10.0, history.CPU[0].Value)
	assert.Equal(t, 20.0, history.Memory[0].Value)
	assert.Equal(t, 30.0, history.Disk[0].Value)
	assert.Equal(t, 40.0, history.Load[0].Value)
}

func TestMemoryStore_GetHistoryReturnsCopy(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Hour)
	store.Add(1.0, 1.0, 1.0, 1.0)

	history1 := store.GetHistory(time.Hour)
	history2 := store.GetHistory(time.Hour)

	// Modify history1
	if len(history1.CPU) > 0 {
		history1.CPU[0].Value = 999.0
	}

	// history2 should not be affected
	assert.Equal(t, 1.0, history2.CPU[0].Value)
}

func TestFilterPoints(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()
	points := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now - 10, Value: 3.0},
		{Timestamp: now, Value: 4.0},
	}

	// Filter with cutoff that keeps all points
	result := filterPoints(points, now-200, true)
	assert.Len(t, result, 4)

	// Filter with cutoff that keeps last 2 points
	result = filterPoints(points, now-30, true)
	assert.Len(t, result, 2)
	assert.Equal(t, 3.0, result[0].Value)
	assert.Equal(t, 4.0, result[1].Value)

	// Filter with cutoff that excludes all points
	result = filterPoints(points, now+10, true)
	assert.Nil(t, result)

	// Empty slice
	result = filterPoints(nil, now, true)
	assert.Nil(t, result)

	// No copy when idx == 0 and copySlice is false
	result = filterPoints(points, now-200, false)
	assert.Equal(t, points, result) // Should be same slice
}

func TestFilterPoints_SinglePoint(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()
	points := []MetricPoint{
		{Timestamp: now, Value: 1.0},
	}

	// Keep the single point
	result := filterPoints(points, now-10, true)
	assert.Len(t, result, 1)
	assert.Equal(t, 1.0, result[0].Value)

	// Exclude the single point
	result = filterPoints(points, now+10, true)
	assert.Nil(t, result)
}

func TestFilterPoints_ExactCutoff(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()
	points := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now, Value: 3.0},
	}

	// Cutoff exactly at middle point timestamp - should include it
	result := filterPoints(points, now-50, true)
	assert.Len(t, result, 2)
	assert.Equal(t, 2.0, result[0].Value)
	assert.Equal(t, 3.0, result[1].Value)
}

func TestFilterPoints_CopySliceBehavior(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()
	points := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now, Value: 3.0},
	}

	// With copySlice=true, should always return a new slice
	result := filterPoints(points, now-200, true)
	assert.Len(t, result, 3)

	// Modify original
	points[0].Value = 999.0

	// Result should not be affected
	assert.Equal(t, 1.0, result[0].Value)

	// With copySlice=false and idx > 0, should still copy (to allow GC)
	points2 := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now, Value: 3.0},
	}
	result2 := filterPoints(points2, now-30, false)
	assert.Len(t, result2, 1)
}
