package resource

import (
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
		s.cpu = prune(s.cpu, cutoff)
		s.memory = prune(s.memory, cutoff)
		s.disk = prune(s.disk, cutoff)
		s.load = prune(s.load, cutoff)
		s.lastPruned = now
	}
}

// GetHistory returns the history of metrics for the specified duration
func (s *MemoryStore) GetHistory(duration time.Duration) *ResourceHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-duration).Unix()

	return &ResourceHistory{
		CPU:    filter(s.cpu, cutoff),
		Memory: filter(s.memory, cutoff),
		Disk:   filter(s.disk, cutoff),
		Load:   filter(s.load, cutoff),
	}
}

func prune(points []MetricPoint, cutoff int64) []MetricPoint {
	// Find the first index where timestamp >= cutoff
	idx := 0
	for i, p := range points {
		if p.Timestamp >= cutoff {
			idx = i
			break
		}
	}
	// If all points are old, return empty slice, but keep capacity if possible?
	// Actually, just re-slicing is fine.
	if idx >= len(points) {
		return []MetricPoint{}
	}
	// Create new slice to allow GC of old array backing if it gets too large?
	// For simplicity, just re-slice for now.
	return points[idx:]
}

func filter(points []MetricPoint, cutoff int64) []MetricPoint {
	// Find start index
	start := 0
	for i, p := range points {
		if p.Timestamp >= cutoff {
			start = i
			break
		}
	}

	if start >= len(points) {
		return []MetricPoint{}
	}

	// Return a copy to avoid race conditions if the caller modifies it (though they shouldn't)
	// and to decouple from the store's internal slice
	result := make([]MetricPoint, len(points)-start)
	copy(result, points[start:])
	return result
}
