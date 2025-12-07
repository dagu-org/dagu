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

// dataPoint stores all metrics for a single timestamp
type dataPoint struct {
	Timestamp int64
	CPU       float64
	Memory    float64
	Disk      float64
	Load      float64
}

// MemoryStore implements Store using in-memory storage
type MemoryStore struct {
	mu         sync.RWMutex
	points     []dataPoint
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

	s.points = append(s.points, dataPoint{
		Timestamp: now.Unix(),
		CPU:       cpu,
		Memory:    memory,
		Disk:      disk,
		Load:      load,
	})

	// Prune old data every minute to avoid excessive scanning
	if now.Sub(s.lastPruned) > time.Minute {
		cutoff := now.Add(-s.retention).Unix()
		s.points = prunePoints(s.points, cutoff)
		s.lastPruned = now
	}
}

// GetHistory returns the history of metrics for the specified duration
func (s *MemoryStore) GetHistory(duration time.Duration) *ResourceHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-duration).Unix()
	filtered := filterPoints(s.points, cutoff)

	if len(filtered) == 0 {
		return &ResourceHistory{}
	}

	// Pre-allocate result slices
	cpu := make([]MetricPoint, len(filtered))
	memory := make([]MetricPoint, len(filtered))
	disk := make([]MetricPoint, len(filtered))
	load := make([]MetricPoint, len(filtered))

	for i, p := range filtered {
		cpu[i] = MetricPoint{Timestamp: p.Timestamp, Value: p.CPU}
		memory[i] = MetricPoint{Timestamp: p.Timestamp, Value: p.Memory}
		disk[i] = MetricPoint{Timestamp: p.Timestamp, Value: p.Disk}
		load[i] = MetricPoint{Timestamp: p.Timestamp, Value: p.Load}
	}

	return &ResourceHistory{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
		Load:   load,
	}
}

// prunePoints removes old points in-place by reslicing (no allocation)
func prunePoints(points []dataPoint, cutoff int64) []dataPoint {
	if len(points) == 0 {
		return points
	}

	idx := sort.Search(len(points), func(i int) bool {
		return points[i].Timestamp >= cutoff
	})

	if idx == 0 {
		return points
	}
	if idx == len(points) {
		return points[:0] // Keep capacity, clear content
	}

	// Shift remaining points to front to allow GC of old data
	n := copy(points, points[idx:])
	return points[:n]
}

// filterPoints returns points with timestamp >= cutoff (creates new slice for reads)
func filterPoints(points []dataPoint, cutoff int64) []dataPoint {
	if len(points) == 0 {
		return nil
	}

	idx := sort.Search(len(points), func(i int) bool {
		return points[i].Timestamp >= cutoff
	})

	if idx == len(points) {
		return nil
	}

	// Copy to decouple from store
	result := make([]dataPoint, len(points)-idx)
	copy(result, points[idx:])
	return result
}
