package types

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// ScheduleValue represents a schedule configuration that can be specified as:
// - A single cron expression string
// - An array of cron expressions
// - A map with start/stop/restart keys
//
// YAML examples:
//
//	schedule: "0 * * * *"
//	schedule: ["0 * * * *", "30 * * * *"]
//	schedule:
//	  start: "0 8 * * *"
//	  stop: "0 18 * * *"
//	  restart: "0 12 * * *"
type ScheduleValue struct {
	raw      any      // Original value for error reporting
	isSet    bool     // Whether the field was set in YAML
	starts   []string // Start schedules (or simple schedule expressions)
	stops    []string // Stop schedules
	restarts []string // Restart schedules
}

// UnmarshalYAML implements BytesUnmarshaler for goccy/go-yaml.
func (s *ScheduleValue) UnmarshalYAML(data []byte) error {
	s.isSet = true

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("schedule unmarshal error: %w", err)
	}
	s.raw = raw

	switch v := raw.(type) {
	case string:
		// Single cron expression
		if v != "" {
			s.starts = []string{v}
		}
		return nil

	case []any:
		// Array of cron expressions
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("schedule[%d]: expected string, got %T", i, item)
			}
			s.starts = append(s.starts, str)
		}
		return nil

	case []string:
		// Array of strings (from Go types)
		s.starts = v
		return nil

	case map[string]any:
		// Map with start/stop/restart keys
		return s.parseScheduleMap(v)

	case nil:
		// nil is valid, just means not set
		s.isSet = false
		return nil

	default:
		return fmt.Errorf("schedule must be string, array, or map, got %T", v)
	}
}

func (s *ScheduleValue) parseScheduleMap(m map[string]any) error {
	for key, v := range m {
		values, err := parseScheduleEntry(v)
		if err != nil {
			return fmt.Errorf("schedule.%s: %w", key, err)
		}

		switch key {
		case "start":
			s.starts = values
		case "stop":
			s.stops = values
		case "restart":
			s.restarts = values
		default:
			return fmt.Errorf("schedule: unknown key %q (expected start, stop, or restart)", key)
		}
	}
	return nil
}

func parseScheduleEntry(v any) ([]string, error) {
	switch val := v.(type) {
	case string:
		return []string{val}, nil
	case []any:
		var result []string
		for i, item := range val {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("[%d]: expected string, got %T", i, item)
			}
			result = append(result, str)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected string or array, got %T", v)
	}
}

// IsZero returns true if the schedule was not set in YAML.
func (s ScheduleValue) IsZero() bool { return !s.isSet }

// Value returns the original raw value for error reporting.
func (s ScheduleValue) Value() any { return s.raw }

// Starts returns the start/simple schedules.
func (s ScheduleValue) Starts() []string { return s.starts }

// Stops returns the stop schedules.
func (s ScheduleValue) Stops() []string { return s.stops }

// Restarts returns the restart schedules.
func (s ScheduleValue) Restarts() []string { return s.restarts }

// HasStopSchedule returns true if stop schedules are configured.
func (s ScheduleValue) HasStopSchedule() bool { return len(s.stops) > 0 }

// HasRestartSchedule returns true if restart schedules are configured.
func (s ScheduleValue) HasRestartSchedule() bool { return len(s.restarts) > 0 }
