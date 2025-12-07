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

// MemoryStore implements Store using in-memory storage
// Data is stored in the same format as the output to avoid transformation overhead
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

// Add appends a new data point for all metrics
func (s *MemoryStore) Add(cpu, memory, disk, load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ts := now.Unix()

	s.cpu = append(s.cpu, MetricPoint{Timestamp: ts, Value: cpu})
	s.memory = append(s.memory, MetricPoint{Timestamp: ts, Value: memory})
	s.disk = append(s.disk, MetricPoint{Timestamp: ts, Value: disk})
	s.load = append(s.load, MetricPoint{Timestamp: ts, Value: load})

	// Prune old data periodically to bound memory usage
	if now.Sub(s.lastPruned) > time.Minute {
		s.prune(now.Add(-s.retention).Unix())
		s.lastPruned = now
	}
}

// prune removes data points older than cutoff
func (s *MemoryStore) prune(cutoff int64) {
	if len(s.cpu) == 0 {
		return
	}

	// Single binary search - timestamps are identical across all slices
	idx := sort.Search(len(s.cpu), func(i int) bool {
		return s.cpu[i].Timestamp >= cutoff
	})

	if idx == 0 {
		return // Nothing to prune
	}

	// Shift remaining data to front to allow GC of backing array space
	s.cpu = shiftSlice(s.cpu, idx)
	s.memory = shiftSlice(s.memory, idx)
	s.disk = shiftSlice(s.disk, idx)
	s.load = shiftSlice(s.load, idx)
}

// shiftSlice moves elements starting at idx to the front
func shiftSlice(s []MetricPoint, idx int) []MetricPoint {
	n := copy(s, s[idx:])
	return s[:n]
}

// GetHistory returns metrics for the specified duration
func (s *MemoryStore) GetHistory(duration time.Duration) *ResourceHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cpu) == 0 {
		return &ResourceHistory{}
	}

	cutoff := time.Now().Add(-duration).Unix()

	// Single binary search - timestamps are identical across all slices
	idx := sort.Search(len(s.cpu), func(i int) bool {
		return s.cpu[i].Timestamp >= cutoff
	})

	if idx >= len(s.cpu) {
		return &ResourceHistory{}
	}

	n := len(s.cpu) - idx

	// Single allocation for all 4 result slices - better cache locality
	all := make([]MetricPoint, n*4)

	// Use optimized builtin copy (SIMD on most platforms)
	copy(all[:n], s.cpu[idx:])
	copy(all[n:2*n], s.memory[idx:])
	copy(all[2*n:3*n], s.disk[idx:])
	copy(all[3*n:], s.load[idx:])

	return &ResourceHistory{
		CPU:    all[:n:n],           // cap=n prevents accidental append corruption
		Memory: all[n : 2*n : 2*n],
		Disk:   all[2*n : 3*n : 3*n],
		Load:   all[3*n : 4*n : 4*n],
	}
}
