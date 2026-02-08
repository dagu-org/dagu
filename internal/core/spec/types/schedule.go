package types

import (
	"fmt"
	"math"

	"github.com/goccy/go-yaml"
)

const maxInt = math.MaxInt

// ScheduleEntry holds a structured schedule entry with catch-up metadata.
type ScheduleEntry struct {
	Cron           string
	Misfire        string // "ignore", "runOnce", "runLatest", "runAll"
	CatchupWindow  string // duration string like "24h", "2d12h"
	MaxCatchupRuns int
}

// ScheduleValue represents a schedule configuration that can be specified as:
// - A single cron expression string
// - An array of cron expressions or schedule-entry objects
// - A map with start/stop/restart keys
// - A map with cron/misfire/catchupWindow/maxCatchupRuns keys (single schedule-entry)
//
// YAML examples:
//
//	schedule: "0 * * * *"
//	schedule: ["0 * * * *", "30 * * * *"]
//	schedule:
//	  start: "0 8 * * *"
//	  stop: "0 18 * * *"
//	  restart: "0 12 * * *"
//	schedule:
//	  cron: "0 * * * *"
//	  misfire: runAll
//	  catchupWindow: "6h"
//	schedule:
//	  - cron: "0 * * * *"
//	    misfire: runAll
//	  - cron: "30 * * * *"
//	    misfire: runOnce
type ScheduleValue struct {
	raw          any              // Original value for error reporting
	isSet        bool             // Whether the field was set in YAML
	starts       []string         // Start schedules (or simple schedule expressions)
	stops        []string         // Stop schedules
	restarts     []string         // Restart schedules
	startEntries []ScheduleEntry  // Structured start entries with catch-up metadata
}

// scheduleEntryKeys are the valid keys for a single schedule-entry map.
var scheduleEntryKeys = map[string]bool{
	"cron": true, "misfire": true, "catchupWindow": true, "maxCatchupRuns": true,
}

// typedScheduleKeys are the valid keys for a typed schedule map (start/stop/restart).
var typedScheduleKeys = map[string]bool{
	"start": true, "stop": true, "restart": true,
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
		return s.parseArray(v)

	case []string:
		// Array of strings (from Go types)
		s.starts = v
		return nil

	case map[string]any:
		return s.parseScheduleMap(v)

	case nil:
		// nil is valid, just means not set
		s.isSet = false
		return nil

	default:
		return fmt.Errorf("schedule must be string, array, or map, got %T", v)
	}
}

// parseArray handles array form: either all strings or all schedule-entry objects.
func (s *ScheduleValue) parseArray(items []any) error {
	if len(items) == 0 {
		return nil
	}

	// Detect type from first element
	switch items[0].(type) {
	case string:
		// All must be strings
		for i, item := range items {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("schedule[%d]: expected string (array must not mix strings and objects), got %T", i, item)
			}
			s.starts = append(s.starts, str)
		}
		return nil

	case map[string]any:
		// All must be schedule-entry objects
		for i, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("schedule[%d]: expected object (array must not mix strings and objects), got %T", i, item)
			}
			entry, err := parseScheduleEntryMap(m)
			if err != nil {
				return fmt.Errorf("schedule[%d]: %w", i, err)
			}
			s.startEntries = append(s.startEntries, entry)
		}
		return nil

	default:
		return fmt.Errorf("schedule[0]: expected string or object, got %T", items[0])
	}
}

func (s *ScheduleValue) parseScheduleMap(m map[string]any) error {
	// Disambiguate: schedule-entry keys vs typed-schedule keys
	hasEntryKeys := false
	hasTypedKeys := false
	for key := range m {
		if scheduleEntryKeys[key] {
			hasEntryKeys = true
		} else if typedScheduleKeys[key] {
			hasTypedKeys = true
		}
	}

	if hasEntryKeys && hasTypedKeys {
		return fmt.Errorf("schedule: cannot mix schedule-entry keys (cron, misfire, ...) with typed keys (start, stop, restart)")
	}

	if hasEntryKeys {
		// Single schedule-entry form: {cron: "...", misfire: "..."}
		entry, err := parseScheduleEntryMap(m)
		if err != nil {
			return err
		}
		s.startEntries = []ScheduleEntry{entry}
		return nil
	}

	// Typed schedule form: {start: "...", stop: "...", restart: "..."}
	for key, v := range m {
		switch key {
		case "start":
			if err := s.parseTypedStart(v); err != nil {
				return err
			}
		case "stop":
			values, err := parseScheduleEntryValue(v)
			if err != nil {
				return fmt.Errorf("schedule.stop: %w", err)
			}
			s.stops = values
		case "restart":
			values, err := parseScheduleEntryValue(v)
			if err != nil {
				return fmt.Errorf("schedule.restart: %w", err)
			}
			s.restarts = values
		default:
			return fmt.Errorf("schedule: unknown key %q (expected start, stop, restart, or cron/misfire/catchupWindow/maxCatchupRuns)", key)
		}
	}
	return nil
}

// parseTypedStart handles the "start" value in a typed schedule map.
// It can be a string, array of strings, schedule-entry object, or array of schedule-entry objects.
func (s *ScheduleValue) parseTypedStart(v any) error {
	switch val := v.(type) {
	case string:
		s.starts = []string{val}
		return nil
	case []any:
		if len(val) == 0 {
			return nil
		}
		// Detect type from first element
		switch val[0].(type) {
		case string:
			for i, item := range val {
				str, ok := item.(string)
				if !ok {
					return fmt.Errorf("schedule.start[%d]: expected string, got %T", i, item)
				}
				s.starts = append(s.starts, str)
			}
			return nil
		case map[string]any:
			for i, item := range val {
				m, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("schedule.start[%d]: expected object, got %T", i, item)
				}
				entry, err := parseScheduleEntryMap(m)
				if err != nil {
					return fmt.Errorf("schedule.start[%d]: %w", i, err)
				}
				s.startEntries = append(s.startEntries, entry)
			}
			return nil
		default:
			return fmt.Errorf("schedule.start[0]: expected string or object, got %T", val[0])
		}
	case map[string]any:
		// Single schedule-entry object under start key
		entry, err := parseScheduleEntryMap(val)
		if err != nil {
			return fmt.Errorf("schedule.start: %w", err)
		}
		s.startEntries = []ScheduleEntry{entry}
		return nil
	default:
		return fmt.Errorf("schedule.start: expected string, array, or object, got %T", v)
	}
}

// parseScheduleEntryMap parses a map as a schedule-entry object.
func parseScheduleEntryMap(m map[string]any) (ScheduleEntry, error) {
	var entry ScheduleEntry

	for key := range m {
		if !scheduleEntryKeys[key] {
			return entry, fmt.Errorf("unknown schedule-entry key %q (expected cron, misfire, catchupWindow, or maxCatchupRuns)", key)
		}
	}

	cronVal, ok := m["cron"]
	if !ok {
		return entry, fmt.Errorf("schedule-entry requires 'cron' key")
	}
	cronStr, ok := cronVal.(string)
	if !ok {
		return entry, fmt.Errorf("schedule-entry 'cron' must be a string, got %T", cronVal)
	}
	entry.Cron = cronStr

	if v, ok := m["misfire"]; ok {
		str, ok := v.(string)
		if !ok {
			return entry, fmt.Errorf("schedule-entry 'misfire' must be a string, got %T", v)
		}
		entry.Misfire = str
	}

	if v, ok := m["catchupWindow"]; ok {
		str, ok := v.(string)
		if !ok {
			return entry, fmt.Errorf("schedule-entry 'catchupWindow' must be a string, got %T", v)
		}
		entry.CatchupWindow = str
	}

	if v, ok := m["maxCatchupRuns"]; ok {
		switch num := v.(type) {
		case int:
			entry.MaxCatchupRuns = num
		case uint64:
			if num > uint64(maxInt) {
				return entry, fmt.Errorf("schedule-entry 'maxCatchupRuns' value %d overflows int", num)
			}
			entry.MaxCatchupRuns = int(num) //nolint:gosec // overflow checked above
		case float64:
			entry.MaxCatchupRuns = int(num)
		default:
			return entry, fmt.Errorf("schedule-entry 'maxCatchupRuns' must be an integer, got %T", v)
		}
	}

	return entry, nil
}

// parseScheduleEntryValue parses values for stop/restart which are always simple strings.
func parseScheduleEntryValue(v any) ([]string, error) {
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

// Starts returns the start/simple schedules (string-only form).
func (s ScheduleValue) Starts() []string { return s.starts }

// Stops returns the stop schedules.
func (s ScheduleValue) Stops() []string { return s.stops }

// Restarts returns the restart schedules.
func (s ScheduleValue) Restarts() []string { return s.restarts }

// StartEntries returns the structured start schedule entries with catch-up metadata.
func (s ScheduleValue) StartEntries() []ScheduleEntry { return s.startEntries }

// HasStopSchedule returns true if stop schedules are configured.
func (s ScheduleValue) HasStopSchedule() bool { return len(s.stops) > 0 }

// HasRestartSchedule returns true if restart schedules are configured.
func (s ScheduleValue) HasRestartSchedule() bool { return len(s.restarts) > 0 }
