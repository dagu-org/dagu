// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package wait

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/value"
	"github.com/dagucloud/dagu/v2/internal/executor/registry"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

type config struct {
	Duration       string            `mapstructure:"duration"`
	Until          string            `mapstructure:"until"`
	Offset         string            `mapstructure:"offset"`
	Timezone       string            `mapstructure:"timezone"`
	IfPast         string            `mapstructure:"if_past"`
	Window         string            `mapstructure:"window"`
	Path           string            `mapstructure:"path"`
	State          string            `mapstructure:"state"`
	URL            string            `mapstructure:"url"`
	Method         string            `mapstructure:"method"`
	Status         any               `mapstructure:"status"`
	Headers        map[string]string `mapstructure:"headers"`
	Body           string            `mapstructure:"body"`
	PollInterval   string            `mapstructure:"poll_interval"`
	RequestTimeout string            `mapstructure:"request_timeout"`
}

const (
	ifPastContinue = "continue"
	ifPastFail     = "fail"
	ifPastWindow   = "window"
)

type parsedConfig struct {
	config
	duration       time.Duration
	until          time.Time
	expectedStatus int
	offset         time.Duration
	ifPast         string
	window         time.Duration
	pollInterval   time.Duration
	requestTimeout time.Duration
}

func defaultConfig() config {
	return config{
		State:          stateExists,
		Method:         "GET",
		PollInterval:   "1s",
		RequestTimeout: "10s",
	}
}

func decodeConfig(raw map[string]any, cfg *config) error {
	if raw == nil {
		raw = map[string]any{}
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		TagName:          "mapstructure",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(raw)
}

func parseConfig(operation string, cfg config, deferReferences bool) (parsedConfig, error) {
	parsed := parsedConfig{config: cfg}
	// Build-time validation leaves a value that still holds a reference to the
	// run-time check, which sees it after resolution.
	deferred := func(raw string) bool {
		return deferReferences && value.HasAnyReference(raw)
	}

	if !deferred(cfg.PollInterval) {
		pollInterval, err := parsePositiveDuration("poll_interval", cfg.PollInterval)
		if err != nil {
			return parsed, withUnresolvedHint(err, cfg.PollInterval)
		}
		parsed.pollInterval = pollInterval
	}

	if !deferred(cfg.RequestTimeout) {
		requestTimeout, err := parsePositiveDuration("request_timeout", cfg.RequestTimeout)
		if err != nil {
			return parsed, withUnresolvedHint(err, cfg.RequestTimeout)
		}
		parsed.requestTimeout = requestTimeout
	}

	switch operation {
	case opDuration:
		if err := rejectUntilOnlyFields(cfg, operation); err != nil {
			return parsed, err
		}
		if strings.TrimSpace(cfg.Duration) == "" {
			return parsed, fmt.Errorf("wait: duration is required for %s", operation)
		}
		if !deferred(cfg.Duration) {
			duration, err := parsePositiveDuration("duration", cfg.Duration)
			if err != nil {
				return parsed, withUnresolvedHint(err, cfg.Duration)
			}
			parsed.duration = duration
		}
	case opUntil:
		if strings.TrimSpace(cfg.Until) == "" {
			return parsed, fmt.Errorf("wait: until is required for %s", operation)
		}
		if err := parseUntilFields(cfg, deferReferences, &parsed); err != nil {
			return parsed, err
		}
	case opFile:
		if err := rejectUntilOnlyFields(cfg, operation); err != nil {
			return parsed, err
		}
		if strings.TrimSpace(cfg.Path) == "" {
			return parsed, fmt.Errorf("wait: path is required for %s", operation)
		}
		if !deferred(cfg.State) {
			switch cfg.State {
			case stateExists, stateMissing:
			default:
				return parsed, withUnresolvedHint(fmt.Errorf("wait: state must be %q or %q", stateExists, stateMissing), cfg.State)
			}
		}
	case opHTTP:
		if err := rejectUntilOnlyFields(cfg, operation); err != nil {
			return parsed, err
		}
		if strings.TrimSpace(cfg.URL) == "" {
			return parsed, fmt.Errorf("wait: url is required for %s", operation)
		}
		if !deferred(cfg.URL) {
			parsedURL, err := url.ParseRequestURI(strings.TrimSpace(cfg.URL))
			if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
				return parsed, withUnresolvedHint(fmt.Errorf("wait: url must be an absolute HTTP URL"), cfg.URL)
			}
		}
		if strings.TrimSpace(cfg.Method) == "" {
			return parsed, fmt.Errorf("wait: method must not be empty")
		}
		statusText, _ := cfg.Status.(string)
		if !deferred(statusText) {
			status, err := parseStatus(cfg.Status)
			if err != nil {
				return parsed, withUnresolvedHint(err, statusText)
			}
			parsed.expectedStatus = status
		}
	default:
		return parsed, fmt.Errorf("wait: unsupported operation %q", operation)
	}

	return parsed, nil
}

// withUnresolvedHint notes in err that raw still holds a reference. Build-time
// validation defers such values, so at run time it means the reference did not
// resolve. The value itself is left out, as it may hold resolved secrets.
func withUnresolvedHint(err error, raw string) error {
	if !value.HasAnyReference(raw) {
		return err
	}
	return fmt.Errorf("%w (value holds an unresolved reference)", err)
}

// Bounds of a valid with.status.
const (
	minHTTPStatus = 100
	maxHTTPStatus = 599
)

var errInvalidStatus = fmt.Errorf("wait: status must be between %d and %d", minHTTPStatus, maxHTTPStatus)

// parseStatus reads with.status from any scalar whose text is an integer, such
// as 204 or "204", and defaults to 200 when it is unset.
func parseStatus(raw any) (int, error) {
	if raw == nil {
		return http.StatusOK, nil
	}
	status, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(raw)))
	if err != nil || status < minHTTPStatus || status > maxHTTPStatus {
		return 0, errInvalidStatus
	}
	return status, nil
}

// parseUntilFields validates the until-only options and parses a literal
// RFC3339 deadline. Clock times stay unresolved for the executor. The signed
// offset is stored separately and applied once by the executor, so both
// RFC3339 and clock-time deadlines shift exactly once.
func parseUntilFields(cfg config, deferReferences bool, parsed *parsedConfig) error {
	if strings.TrimSpace(cfg.Timezone) != "" {
		if !deferReferences || !value.HasValueReference(cfg.Timezone) {
			if _, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone)); err != nil {
				return fmt.Errorf("wait: timezone must be an IANA name: %w", err)
			}
		}
	}
	if strings.TrimSpace(cfg.Offset) != "" {
		if !deferReferences || !value.HasValueReference(cfg.Offset) {
			offset, err := time.ParseDuration(strings.TrimSpace(cfg.Offset))
			if err != nil {
				return fmt.Errorf("wait: offset must be a duration: %w", err)
			}
			parsed.offset = offset
		}
	}
	ifPast := strings.ToLower(strings.TrimSpace(cfg.IfPast))
	if ifPast == "" {
		ifPast = ifPastContinue
	}
	switch ifPast {
	case ifPastContinue, ifPastFail, ifPastWindow:
		parsed.ifPast = ifPast
	default:
		if !deferReferences || !value.HasValueReference(cfg.IfPast) {
			return fmt.Errorf("wait: if_past must be %q, %q, or %q", ifPastContinue, ifPastFail, ifPastWindow)
		}
	}
	if strings.TrimSpace(cfg.Window) != "" {
		if !deferReferences || !value.HasValueReference(cfg.Window) {
			window, err := time.ParseDuration(strings.TrimSpace(cfg.Window))
			if err != nil {
				return fmt.Errorf("wait: window must be a duration: %w", err)
			}
			if window <= 0 {
				return fmt.Errorf("wait: window must be greater than 0")
			}
			parsed.window = window
		}
	}
	if ifPast == ifPastWindow && strings.TrimSpace(cfg.Window) == "" {
		return fmt.Errorf("wait: window is required when if_past is %q", ifPastWindow)
	}
	if strings.TrimSpace(cfg.Window) != "" && ifPast != ifPastWindow &&
		(!deferReferences || (!value.HasValueReference(cfg.Window) && !value.HasValueReference(cfg.IfPast))) {
		return fmt.Errorf("wait: window requires if_past %q", ifPastWindow)
	}
	if !deferReferences || !value.HasValueReference(cfg.Until) {
		raw := strings.TrimSpace(cfg.Until)
		if until, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			parsed.until = until
		} else if _, _, _, err := parseClock(raw); err != nil {
			return err
		}
	}
	return nil
}

func untilLocation(tz string) *time.Location {
	if strings.TrimSpace(tz) == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(strings.TrimSpace(tz))
	if err != nil {
		return time.Local
	}
	return loc
}

func parseClock(raw string) (int, int, int, error) {
	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("wait: until must be RFC3339 timestamp or HH:MM[:SS]")
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, 0, 0, fmt.Errorf("wait: until must be RFC3339 timestamp or HH:MM[:SS]")
		}
		nums[i] = n
	}
	second := 0
	if len(nums) == 3 {
		second = nums[2]
	}
	if nums[0] < 0 || nums[0] > 23 || nums[1] < 0 || nums[1] > 59 || second < 0 || second > 59 {
		return 0, 0, 0, fmt.Errorf("wait: until must be RFC3339 timestamp or HH:MM[:SS]")
	}
	return nums[0], nums[1], second, nil
}

func rejectUntilOnlyFields(cfg config, operation string) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"offset", cfg.Offset},
		{"timezone", cfg.Timezone},
		{"if_past", cfg.IfPast},
		{"window", cfg.Window},
	} {
		if strings.TrimSpace(field.value) != "" {
			return fmt.Errorf("wait: %s is only valid for %s", field.name, opUntil)
		}
	}
	return nil
}

func parsePositiveDuration(field, value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("wait: %s must not be empty", field)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("wait: %s must be a duration: %w", field, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("wait: %s must be greater than 0", field)
	}
	return duration, nil
}

var configSchema = &jsonschema.Schema{
	Type:                 "object",
	AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	Properties: map[string]*jsonschema.Schema{
		"duration":        {Type: "string", Description: "Duration to wait for action: wait.duration, such as 30s or 5m."},
		"until":           {Type: "string", Description: "RFC3339 timestamp or HH:MM[:SS] clock time for action: wait.until."},
		"offset":          {Type: "string", Description: "Duration added to with.until, such as 5m. Only valid for action: wait.until."},
		"timezone":        {Type: "string", Description: "IANA timezone for HH:MM[:SS] with.until values. Ignored when with.until is RFC3339. Only valid for action: wait.until."},
		"if_past":         {Type: "string", Description: "Behavior when with.until is in the past: continue, fail, or window. Accepts a ${...} reference. Only valid for action: wait.until. Defaults to continue."},
		"window":          {Type: "string", Description: "How long after with.until still counts as on time when if_past is window. Only valid for action: wait.until."},
		"path":            {Type: "string", Description: "File or directory path for action: wait.file."},
		"state":           {Type: "string", Description: "File state to wait for: exists or missing. Defaults to exists."},
		"url":             {Type: "string", Description: "Absolute URL for action: wait.http."},
		"method":          {Type: "string", Description: "HTTP method for action: wait.http. Defaults to GET."},
		"status":          {Types: []string{"integer", "string"}, Minimum: new(float64(minHTTPStatus)), Maximum: new(float64(maxHTTPStatus)), Description: "Expected HTTP status for action: wait.http. Defaults to 200."},
		"headers":         {Type: "object", AdditionalProperties: &jsonschema.Schema{Type: "string"}, Description: "HTTP headers for action: wait.http."},
		"body":            {Type: "string", Description: "HTTP request body for action: wait.http."},
		"poll_interval":   {Type: "string", Description: "Polling interval for wait.file and wait.http. Defaults to 1s."},
		"request_timeout": {Type: "string", Description: "Per-request timeout for wait.http. Defaults to 10s."},
	},
}

func init() {
	registry.RegisterExecutorConfigSchema(executorType, configSchema)
}
