// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

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

// ComputeMissedIntervals iterates each schedule's cron expression from
// replayFrom to replayTo, collects all missed ticks, merges and sorts
// chronologically. Duplicates across schedules are removed.
func ComputeMissedIntervals(schedules []core.Schedule, replayFrom, replayTo time.Time) []time.Time {
	seen := make(map[time.Time]bool)
	var result []time.Time

	for _, sched := range schedules {
		if sched.Parsed == nil {
			continue
		}
		// Start after replayFrom (exclusive start) — replayFrom was already dispatched.
		t := sched.Parsed.Next(replayFrom)
		for !t.After(replayTo) {
			if !seen[t] {
				seen[t] = true
				result = append(result, t)
			}
			t = sched.Parsed.Next(t)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Before(result[j])
	})

	return result
}
