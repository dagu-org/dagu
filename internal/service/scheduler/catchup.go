package scheduler

import (
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/core"
)

// ComputeReplayFrom computes the earliest timestamp worth replaying for a DAG.
//
//	replayFrom = max(
//	    now - catchupWindow,
//	    lastTick,
//	    lastScheduledTime,
//	)
func ComputeReplayFrom(catchupWindow time.Duration, lastTick, lastScheduledTime, now time.Time) time.Time {
	earliest := now.Add(-catchupWindow)
	if lastTick.After(earliest) {
		earliest = lastTick
	}
	if lastScheduledTime.After(earliest) {
		earliest = lastScheduledTime
	}
	return earliest
}

// MaxMissedRuns is the maximum number of missed intervals that will be
// replayed per DAG. Prevents memory explosion for large catchup windows
// with high-frequency schedules (e.g., 30-day window + per-minute cron).
const MaxMissedRuns = 1000

// ComputeMissedIntervals iterates each schedule's cron expression from
// replayFrom to replayTo, collects all missed ticks, merges and sorts
// chronologically. Duplicates across schedules are removed.
// If the total exceeds MaxMissedRuns, only the most recent runs are kept.
func ComputeMissedIntervals(schedules []core.Schedule, replayFrom, replayTo time.Time) []time.Time {
	seen := make(map[time.Time]struct{})
	var result []time.Time

	for _, sched := range schedules {
		if sched.Parsed == nil {
			continue
		}
		// Exclusive start: replayFrom was already dispatched.
		t := sched.Parsed.Next(replayFrom)
		for !t.After(replayTo) {
			if _, dup := seen[t]; !dup {
				seen[t] = struct{}{}
				result = append(result, t)
			}
			t = sched.Parsed.Next(t)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Before(result[j])
	})

	// Cap to most recent runs to prevent memory explosion.
	if len(result) > MaxMissedRuns {
		result = result[len(result)-MaxMissedRuns:]
	}

	return result
}
