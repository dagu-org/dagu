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
	status         int
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

	if !deferReferences || !isValueReference(cfg.PollInterval) {
		pollInterval, err := parsePositiveDuration("poll_interval", cfg.PollInterval)
		if err != nil {
			return parsed, err
		}
		parsed.pollInterval = pollInterval
	}

	if !deferReferences || !isValueReference(cfg.RequestTimeout) {
		requestTimeout, err := parsePositiveDuration("request_timeout", cfg.RequestTimeout)
		if err != nil {
			return parsed, err
		}
		parsed.requestTimeout = requestTimeout
	}

	switch operation {
	case opDuration:
		if strings.TrimSpace(cfg.Duration) == "" {
			return parsed, fmt.Errorf("wait: duration is required for %s", operation)
		}
		if !deferReferences || !isValueReference(cfg.Duration) {
			duration, err := parsePositiveDuration("duration", cfg.Duration)
			if err != nil {
				return parsed, err
			}
			parsed.duration = duration
		}
	case opUntil:
		if strings.TrimSpace(cfg.Until) == "" {
			return parsed, fmt.Errorf("wait: until is required for %s", operation)
		}
		if !deferReferences || !isValueReference(cfg.Until) {
			until, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cfg.Until))
			if err != nil {
				return parsed, fmt.Errorf("wait: until must be RFC3339 timestamp: %w", err)
			}
			parsed.until = until
		}
	case opFile:
		if strings.TrimSpace(cfg.Path) == "" {
			return parsed, fmt.Errorf("wait: path is required for %s", operation)
		}
		if !deferReferences || !isValueReference(cfg.State) {
			switch cfg.State {
			case stateExists, stateMissing:
			default:
				return parsed, fmt.Errorf("wait: state must be %q or %q", stateExists, stateMissing)
			}
		}
	case opHTTP:
		if strings.TrimSpace(cfg.URL) == "" {
			return parsed, fmt.Errorf("wait: url is required for %s", operation)
		}
		if !deferReferences || !isValueReference(cfg.URL) {
			parsedURL, err := url.ParseRequestURI(strings.TrimSpace(cfg.URL))
			if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
				return parsed, fmt.Errorf("wait: url must be an absolute HTTP URL")
			}
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return parsed, fmt.Errorf("wait: url must be an absolute HTTP URL")
			}
		}
		if strings.TrimSpace(cfg.Method) == "" {
			return parsed, fmt.Errorf("wait: method must not be empty")
		}
		status, err := parseStatus(cfg.Status, deferReferences)
		if err != nil {
			return parsed, err
		}
		parsed.status = status
	default:
		return parsed, fmt.Errorf("wait: unsupported operation %q", operation)
	}

	return parsed, nil
}

// isValueReference reports whether text holds a value reference, including the
// bare ${NAME} and $NAME environment forms, so it resolves only at run time.
func isValueReference(text string) bool {
	text = strings.TrimSpace(text)
	return value.IsWholeReference(text) || value.HasValueReference(text)
}

func parseStatus(raw any, deferReferences bool) (int, error) {
	invalid := fmt.Errorf("wait: status must be between 100 and 599")
	if raw == nil {
		return http.StatusOK, nil
	}
	var n int
	switch v := raw.(type) {
	case string:
		if deferReferences && isValueReference(v) {
			return 0, nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, invalid
		}
		n = parsed
	case int:
		n = v
	case int64:
		n = int(v)
	case uint64:
		n = int(v)
	case float64:
		if v != float64(int(v)) {
			return 0, invalid
		}
		n = int(v)
	default:
		return 0, invalid
	}
	if n < 100 || n > 599 {
		return 0, invalid
	}
	return n, nil
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
		"status":          {Types: []string{"integer", "string"}, Minimum: new(float64(100)), Maximum: new(float64(599)), Description: "Expected HTTP status for action: wait.http. Defaults to 200."},
		"headers":         {Type: "object", AdditionalProperties: &jsonschema.Schema{Type: "string"}, Description: "HTTP headers for action: wait.http."},
		"body":            {Type: "string", Description: "HTTP request body for action: wait.http."},
		"poll_interval":   {Type: "string", Description: "Polling interval for wait.file and wait.http. Defaults to 1s."},
		"request_timeout": {Type: "string", Description: "Per-request timeout for wait.http. Defaults to 10s."},
	},
}

func init() {
	registry.RegisterExecutorConfigSchema(executorType, configSchema)
}
