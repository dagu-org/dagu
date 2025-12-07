package resource

import (
	"sort"
	"sync"
	"time"
)

// MetricPoint represents a single data point for a metric
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// ResourceHistory holds the history of resource usage
type ResourceHistory struct {
	CPU    []MetricPoint `json:"cpu"`
	Memory []MetricPoint `json:"memory"`
	Disk   []MetricPoint `json:"disk"`
	Load   []MetricPoint `json:"load"`
}

// Store defines the interface for storing resource metrics
type Store interface {
	Add(cpu, memory, disk, load float64)
	GetHistory(duration time.Duration) *ResourceHistory
}

// MemoryStore implements Store using in-memory slices
type MemoryStore struct {
	mu         sync.RWMutex
	cpu        []MetricPoint
	memory     []MetricPoint
	disk       []MetricPoint
	load       []MetricPoint
	retention  time.Duration
	lastPruned time.Time
}

// NewMemoryStore creates a new MemoryStore
func NewMemoryStore(retention time.Duration) *MemoryStore {
	return &MemoryStore{
		retention: retention,
	}
}

// Add adds a new data point for all metrics
func (s *MemoryStore) Add(cpu, memory, disk, load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ts := now.Unix()

	s.cpu = append(s.cpu, MetricPoint{Timestamp: ts, Value: cpu})
	s.memory = append(s.memory, MetricPoint{Timestamp: ts, Value: memory})
	s.disk = append(s.disk, MetricPoint{Timestamp: ts, Value: disk})
	s.load = append(s.load, MetricPoint{Timestamp: ts, Value: load})

	// Prune old data every minute to avoid excessive locking/scanning
	if now.Sub(s.lastPruned) > time.Minute {
		cutoff := now.Add(-s.retention).Unix()
		s.cpu = filterPoints(s.cpu, cutoff, false)
		s.memory = filterPoints(s.memory, cutoff, false)
		s.disk = filterPoints(s.disk, cutoff, false)
		s.load = filterPoints(s.load, cutoff, false)
		s.lastPruned = now
	}
}

// GetHistory returns the history of metrics for the specified duration
func (s *MemoryStore) GetHistory(duration time.Duration) *ResourceHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-duration).Unix()

	return &ResourceHistory{
		CPU:    filterPoints(s.cpu, cutoff, true),
		Memory: filterPoints(s.memory, cutoff, true),
		Disk:   filterPoints(s.disk, cutoff, true),
		Load:   filterPoints(s.load, cutoff, true),
	}
}

// filterPoints returns points with timestamp >= cutoff using binary search.
// If copy is true, returns a new slice to decouple from the original.
func filterPoints(points []MetricPoint, cutoff int64, copySlice bool) []MetricPoint {
	if len(points) == 0 {
		return nil
	}

	// Binary search for first point >= cutoff
	idx := sort.Search(len(points), func(i int) bool {
		return points[i].Timestamp >= cutoff
	})

	if idx == len(points) {
		return nil // All points are old
	}

	if !copySlice && idx == 0 {
		return points // No pruning needed, return original
	}

	// Return a copy to allow GC of old backing array or decouple from store
	result := make([]MetricPoint, len(points)-idx)
	copy(result, points[idx:])
	return result
}
