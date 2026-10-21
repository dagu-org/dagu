// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package schedulerstate

import (
	"context"
	"time"
)

// Pause is the cluster-wide scheduler pause state.
//
// While paused, the scheduler creates no scheduler-managed runs. Manual,
// webhook, and sub-DAG runs are unaffected, matching per-DAG suspension.
type Pause struct {
	Paused   bool
	PausedAt time.Time
	PausedBy string
	Reason   string
}

// PausedByScheduler is the actor of a pause the scheduler places on itself
// after the host wakes from sleep, a wake hold (see scheduler.wake_hold).
// Such a pause keeps only the scheduler's own ticks back: queued runs go on,
// and nothing is discarded. Clearing the pause through the API ends the hold.
const PausedByScheduler = "scheduler"

// WakeHold reports whether this pause is one the scheduler placed on itself
// after the host woke, rather than an operator's.
func (p Pause) WakeHold() bool {
	return p.Paused && p.PausedBy == PausedByScheduler
}

// PauseStore persists the scheduler pause state. Implementations must be safe
// for concurrent use. Get returns an independent snapshot that callers may
// mutate.
//
// A store that has never been written reports the zero value, which is not
// paused.
type PauseStore interface {
	// IsPaused reports whether scheduler-managed dispatch is currently paused.
	IsPaused(ctx context.Context) (bool, error)
	// Get returns the full pause state, including who paused and why.
	Get(ctx context.Context) (Pause, error)
	// Set records the pause state. Clearing the pause discards the recorded
	// actor and reason.
	Set(ctx context.Context, paused bool, actor, reason string) error
}
