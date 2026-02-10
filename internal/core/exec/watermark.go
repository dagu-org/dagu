// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package exec

import (
	"context"
	"time"
)

// SchedulerState holds the persistent watermark state for the scheduler.
// Callers load once at startup, mutate in memory, and save periodically.
type SchedulerState struct {
	// Version enables future schema migrations.
	Version int `json:"version"`
	// LastTick is the last tick time the scheduler successfully dispatched.
	LastTick time.Time `json:"lastTick"`
	// DAGs contains per-DAG watermark state.
	DAGs map[string]DAGWatermark `json:"dags,omitempty"`
}

// DAGWatermark tracks the last scheduled time for a single DAG.
type DAGWatermark struct {
	// LastScheduledTime is the most recent scheduled time dispatched for this DAG.
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
