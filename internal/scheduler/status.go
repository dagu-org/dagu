package scheduler

import (
	"sync/atomic"
)

// globalSchedulerRunning tracks if any scheduler instance is running
var globalSchedulerRunning atomic.Bool

// IsSchedulerRunning returns true if a scheduler is currently running
func IsSchedulerRunning() bool {
	return globalSchedulerRunning.Load()
}

// setSchedulerRunning updates the global scheduler running status
func setSchedulerRunning(running bool) {
	globalSchedulerRunning.Store(running)
}
