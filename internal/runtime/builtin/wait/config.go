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

type parsedConfig struct {
	config
	duration       time.Duration
	until          time.Time
	expectedStatus int
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
		if !deferred(cfg.Until) {
			until, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cfg.Until))
			if err != nil {
				return parsed, withUnresolvedHint(fmt.Errorf("wait: until must be RFC3339 timestamp: %w", err), cfg.Until)
			}
			parsed.until = until
		}
	case opFile:
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
		"until":           {Type: "string", Description: "RFC3339 timestamp for action: wait.until."},
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
