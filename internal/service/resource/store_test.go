package resource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStore_AddAndGet(t *testing.T) {
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

func TestMemoryStore_Retention(t *testing.T) {
	// Short retention for testing
	store := NewMemoryStore(100 * time.Millisecond)

	// Add old data
	store.Add(1.0, 1.0, 1.0, 1.0)

	// Wait for retention period and ensure timestamp change (Unix seconds)
	time.Sleep(1100 * time.Millisecond)

	// Add new data
	store.Add(2.0, 2.0, 2.0, 2.0)

	// Get history for a duration longer than retention
	history := store.GetHistory(time.Minute)

	// Should only have the new data because GetHistory filters by duration from Now()
	// However, the store itself might still have the old data until prune is called.
	// But GetHistory logic: cutoff := time.Now().Add(-duration).Unix()
	// If duration is 1m, cutoff is 1m ago. Both points are within 1m.
	// Wait, the store retention is 100ms.
	// The prune logic runs every minute in Add(). So for this short test, prune won't run.
	// But GetHistory takes a duration. If we ask for 1m history, we should see both points?
	// Ah, the retention in NewMemoryStore is for pruning.
	// The duration in GetHistory is for filtering the view.

	// Let's test GetHistory filtering first.
	assert.Len(t, history.CPU, 2)

	// Now test GetHistory with short duration
	historyShort := store.GetHistory(50 * time.Millisecond)
	assert.Len(t, historyShort.CPU, 1)
	assert.Equal(t, 2.0, historyShort.CPU[0].Value)
}

func TestFilterPoints(t *testing.T) {
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
