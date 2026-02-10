package core

import (
	"context"
	"time"
)

// SchedulerState holds persistent watermark state for the scheduler.
// Loaded once at startup, mutated in memory, and saved periodically.
type SchedulerState struct {
	Version  int                     `json:"version"`
	LastTick time.Time               `json:"lastTick"`
	DAGs     map[string]DAGWatermark `json:"dags,omitempty"`
}

// DAGWatermark tracks the last scheduled time for a single DAG.
type DAGWatermark struct {
	LastScheduledTime time.Time `json:"lastScheduledTime"`
}

// WatermarkStore persists scheduler watermark state to durable storage.
type WatermarkStore interface {
	// Load reads the scheduler state from storage.
	// Returns a fresh state (Version=1) if the store is empty or corrupt.
	Load(ctx context.Context) (*SchedulerState, error)

	// Save writes the scheduler state to storage atomically.
	Save(ctx context.Context, state *SchedulerState) error
}
