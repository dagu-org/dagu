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

	store := NewMemoryStore(time.Hour)

	// Add old data
	store.Add(1.0, 1.0, 1.0, 1.0)

	// Wait for timestamp change (Unix seconds granularity)
	time.Sleep(1100 * time.Millisecond)

	// Add new data
	store.Add(2.0, 2.0, 2.0, 2.0)

	// Long duration returns all points
	history := store.GetHistory(time.Minute)
	assert.Len(t, history.CPU, 2)

	// Short duration returns only recent data
	historyShort := store.GetHistory(500 * time.Millisecond)
	require.Len(t, historyShort.CPU, 1)
	assert.Equal(t, 2.0, historyShort.CPU[0].Value)
}

func TestMemoryStore_Prune(t *testing.T) {
	t.Parallel()

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

func TestFilterPoints(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()
	points := []MetricPoint{
		{Timestamp: now - 100, Value: 1.0},
		{Timestamp: now - 50, Value: 2.0},
		{Timestamp: now - 10, Value: 3.0},
		{Timestamp: now, Value: 4.0},
	}

	tests := []struct {
		name      string
		cutoff    int64
		copySlice bool
		wantLen   int
		wantFirst float64
	}{
		{"keeps all points", now - 200, true, 4, 1.0},
		{"keeps last 2 points", now - 30, true, 2, 3.0},
		{"exact cutoff includes boundary", now - 50, true, 3, 2.0},
		{"excludes all points", now + 10, true, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterPoints(points, tt.cutoff, tt.copySlice)
			if tt.wantLen == 0 {
				assert.Nil(t, result)
			} else {
				assert.Len(t, result, tt.wantLen)
				assert.Equal(t, tt.wantFirst, result[0].Value)
			}
		})
	}

	// Empty slice
	assert.Nil(t, filterPoints(nil, now, true))

	// No copy when idx == 0 and copySlice is false
	result := filterPoints(points, now-200, false)
	assert.Same(t, &points[0], &result[0], "should return same slice")

	// Copy when idx > 0 even with copySlice=false
	result = filterPoints(points, now-30, false)
	assert.Len(t, result, 2)
}
