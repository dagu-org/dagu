package scheduler

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
	Load(ctx context.Context) (*SchedulerState, error)
	Save(ctx context.Context, state *SchedulerState) error
}

// noopWatermarkStore is a no-op implementation used when no store is configured.
type noopWatermarkStore struct{}

var _ WatermarkStore = noopWatermarkStore{}

func (noopWatermarkStore) Load(_ context.Context) (*SchedulerState, error) {
	return &SchedulerState{Version: 1, DAGs: make(map[string]DAGWatermark)}, nil
}

func (noopWatermarkStore) Save(_ context.Context, _ *SchedulerState) error {
	return nil
}
