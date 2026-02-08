package spec

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/cmn/duration"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// buildScheduler parses the schedule values and returns a list of schedules.
// each schedule is parsed as a cron expression.
func buildScheduler(values []string) ([]core.Schedule, error) {
	var ret []core.Schedule

	for _, v := range values {
		parsed, err := cronParser.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidSchedule, err)
		}
		ret = append(ret, core.Schedule{Expression: v, Parsed: parsed})
	}

	return ret, nil
}

// buildSchedulerFromEntries parses structured schedule entries with catch-up metadata.
func buildSchedulerFromEntries(entries []types.ScheduleEntry) ([]core.Schedule, error) {
	var ret []core.Schedule

	for _, e := range entries {
		parsed, err := cronParser.Parse(e.Cron)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidSchedule, err)
		}

		sched := core.Schedule{
			Expression:     e.Cron,
			Parsed:         parsed,
			MaxCatchupRuns: e.MaxCatchupRuns,
		}

		if e.Misfire != "" {
			sched.Misfire, err = core.ParseMisfirePolicy(e.Misfire)
			if err != nil {
				return nil, fmt.Errorf("schedule entry %q: %w", e.Cron, err)
			}
		}

		if e.CatchupWindow != "" {
			sched.CatchupWindow, err = duration.Parse(e.CatchupWindow)
			if err != nil {
				return nil, fmt.Errorf("schedule entry %q: %w", e.Cron, err)
			}
		}

		ret = append(ret, sched)
	}

	return ret, nil
}
