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

// parseScheduleMap parses the schedule map and populates the starts, stops,
// and restarts slices. Each key in the map must be either "start", "stop", or
// "restart". The value can be Case 1 or Case 2.
//
// Case 1: The value is a string
// Case 2: The value is an array of strings
//
// Example:
// ```yaml
// schedule:
//
//	start: "0 1 * * *"
//	stop: "0 18 * * *"
//	restart:
//	  - "0 1 * * *"
//	  - "0 18 * * *"
//
// ```
func parseScheduleMap(
	scheduleMap map[string]any, starts, stops, restarts *[]string,
) error {
	for key, v := range scheduleMap {
		var values []string

		switch v := v.(type) {
		case string:
			// Case 1. schedule is a string.
			values = append(values, v)

		case []any:
			// Case 2. schedule is an array of strings.
			// Append all the schedules to the values slice.
			for _, s := range v {
				s, ok := s.(string)
				if !ok {
					return core.NewValidationError("schedule", s, ErrScheduleMustBeStringOrArray)
				}
				values = append(values, s)
			}

		}

		var targets *[]string

		switch scheduleKey(key) {
		case scheduleKeyStart:
			targets = starts

		case scheduleKeyStop:
			targets = stops

		case scheduleKeyRestart:
			targets = restarts

		}

		for _, v := range values {
			if _, err := cronParser.Parse(v); err != nil {
				return core.NewValidationError("schedule", v, fmt.Errorf("%w: %s", ErrInvalidSchedule, err))
			}
			*targets = append(*targets, v)
		}
	}

	return nil
}

type scheduleKey string

const (
	scheduleKeyStart   scheduleKey = "start"
	scheduleKeyStop    scheduleKey = "stop"
	scheduleKeyRestart scheduleKey = "restart"
)
