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

// dataPoint stores all metrics for a single timestamp
type dataPoint struct {
	Timestamp int64
	CPU       float64
	Memory    float64
	Disk      float64
	Load      float64
}

// MemoryStore implements Store using a generic ring buffer
type MemoryStore struct {
	mu     sync.RWMutex
	buffer *RingBuffer[dataPoint]
}

// NewMemoryStore creates a new store with capacity based on retention
func NewMemoryStore(retention time.Duration) *MemoryStore {
	return NewMemoryStoreWithInterval(retention, 10*time.Second)
}

// NewMemoryStoreWithInterval creates a store with explicit interval for capacity calculation
func NewMemoryStoreWithInterval(retention, interval time.Duration) *MemoryStore {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	capacity := int(retention/interval) + 10
	return &MemoryStore{
		buffer: NewRingBuffer[dataPoint](capacity),
	}
}

// Add writes a new data point. Zero allocations, O(1).
func (s *MemoryStore) Add(cpu, memory, disk, load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer.Push(dataPoint{
		Timestamp: time.Now().Unix(),
		CPU:       cpu,
		Memory:    memory,
		Disk:      disk,
		Load:      load,
	})
}

// GetHistory returns metrics for the specified duration
func (s *MemoryStore) GetHistory(duration time.Duration) *ResourceHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.buffer.Len() == 0 {
		return &ResourceHistory{}
	}

	cutoff := time.Now().Add(-duration).Unix()

	// Count matching points for pre-allocation
	n := 0
	s.buffer.ForEach(func(p *dataPoint) bool {
		if p.Timestamp >= cutoff {
			n++
		}
		return true
	})

	if n == 0 {
		return &ResourceHistory{}
	}

	cpu := make([]MetricPoint, 0, n)
	memory := make([]MetricPoint, 0, n)
	disk := make([]MetricPoint, 0, n)
	load := make([]MetricPoint, 0, n)

	s.buffer.ForEach(func(p *dataPoint) bool {
		if p.Timestamp >= cutoff {
			cpu = append(cpu, MetricPoint{Timestamp: p.Timestamp, Value: p.CPU})
			memory = append(memory, MetricPoint{Timestamp: p.Timestamp, Value: p.Memory})
			disk = append(disk, MetricPoint{Timestamp: p.Timestamp, Value: p.Disk})
			load = append(load, MetricPoint{Timestamp: p.Timestamp, Value: p.Load})
		}
		return true
	})

	return &ResourceHistory{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
		Load:   load,
	}
}
