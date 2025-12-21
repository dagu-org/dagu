package spec

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
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

