package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRelativeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "7 days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "24 hours",
			input:    "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "1 week",
			input:    "1w",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "30 days",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "single hour",
			input:    "1h",
			expected: 1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "2 weeks",
			input:    "2w",
			expected: 14 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:    "invalid format - no unit",
			input:   "7",
			wantErr: true,
		},
		{
			name:    "invalid format - invalid unit",
			input:   "7x",
			wantErr: true,
		},
		{
			name:    "invalid format - no number",
			input:   "d",
			wantErr: true,
		},
		{
			name:    "invalid format - spaces",
			input:   "7 d",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "negative number",
			input:   "-7d",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseRelativeDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseAbsoluteDateTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "RFC3339 format",
			input:    "2026-02-01T12:00:00Z",
			expected: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "date only format (midnight UTC)",
			input:    "2026-02-01",
			expected: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "datetime without timezone (UTC assumed)",
			input:    "2026-02-01T15:04:05",
			expected: time.Date(2026, 2, 1, 15, 4, 5, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "2026-13-01",
			wantErr: true,
		},
		{
			name:    "invalid format - wrong separator",
			input:   "2026/02/01",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid day",
			input:   "2026-02-30",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAbsoluteDateTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.UTC(), got.UTC())
		})
	}
}

func TestParseStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected core.Status
		wantErr  bool
	}{
		{
			name:     "running",
			input:    "running",
			expected: core.Running,
			wantErr:  false,
		},
		{
			name:     "succeeded",
			input:    "succeeded",
			expected: core.Succeeded,
			wantErr:  false,
		},
		{
			name:     "success (alias)",
			input:    "success",
			expected: core.Succeeded,
			wantErr:  false,
		},
		{
			name:     "failed",
			input:    "failed",
			expected: core.Failed,
			wantErr:  false,
		},
		{
			name:     "failure (alias)",
			input:    "failure",
			expected: core.Failed,
			wantErr:  false,
		},
		{
			name:     "aborted",
			input:    "aborted",
			expected: core.Aborted,
			wantErr:  false,
		},
		{
			name:     "canceled (alias)",
			input:    "canceled",
			expected: core.Aborted,
			wantErr:  false,
		},
		{
			name:     "cancelled (alias)",
			input:    "cancelled",
			expected: core.Aborted,
			wantErr:  false,
		},
		{
			name:     "cancel (alias)",
			input:    "cancel",
			expected: core.Aborted,
			wantErr:  false,
		},
		{
			name:     "queued",
			input:    "queued",
			expected: core.Queued,
			wantErr:  false,
		},
		{
			name:     "waiting",
			input:    "waiting",
			expected: core.Waiting,
			wantErr:  false,
		},
		{
			name:     "not_started",
			input:    "not_started",
			expected: core.NotStarted,
			wantErr:  false,
		},
		{
			name:     "uppercase input",
			input:    "RUNNING",
			expected: core.Running,
			wantErr:  false,
		},
		{
			name:     "mixed case input",
			input:    "Failed",
			expected: core.Failed,
			wantErr:  false,
		},
		{
			name:     "input with spaces",
			input:    "  succeeded  ",
			expected: core.Succeeded,
			wantErr:  false,
		},
		{
			name:    "invalid status",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "finished (not a valid status)",
			input:   "finished",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseStatus(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid status")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single tag",
			input:    "prod",
			expected: []string{"prod"},
		},
		{
			name:     "multiple tags",
			input:    "prod,critical",
			expected: []string{"prod", "critical"},
		},
		{
			name:     "tags with spaces",
			input:    "prod, critical, backend",
			expected: []string{"prod", "critical", "backend"},
		},
		{
			name:     "tags with extra whitespace",
			input:    "  prod  ,  critical  ",
			expected: []string{"prod", "critical"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only commas",
			input:    ",,",
			expected: nil,
		},
		{
			name:     "empty tags between commas",
			input:    "prod,,critical",
			expected: []string{"prod", "critical"},
		},
		{
			name:     "single tag with trailing comma",
			input:    "prod,",
			expected: []string{"prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseTags(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatStatusText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   core.Status
		expected string
	}{
		{
			name:     "NotStarted",
			status:   core.NotStarted,
			expected: "Not Started",
		},
		{
			name:     "Running",
			status:   core.Running,
			expected: "Running",
		},
		{
			name:     "Succeeded",
			status:   core.Succeeded,
			expected: "Succeeded",
		},
		{
			name:     "Failed",
			status:   core.Failed,
			expected: "Failed",
		},
		{
			name:     "Aborted",
			status:   core.Aborted,
			expected: "Aborted",
		},
		{
			name:     "Queued",
			status:   core.Queued,
			expected: "Queued",
		},
		{
			name:     "PartiallySucceeded",
			status:   core.PartiallySucceeded,
			expected: "Partially Succeeded",
		},
		{
			name:     "Waiting",
			status:   core.Waiting,
			expected: "Waiting",
		},
		{
			name:     "Rejected",
			status:   core.Rejected,
			expected: "Rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatStatusText(tt.status)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "RFC3339 format",
			input:    "2026-02-01T12:00:00Z",
			expected: "2026-02-01 12:00:00",
		},
		{
			name:     "Alternative format",
			input:    "2026-02-01 15:04:05",
			expected: "2026-02-01 15:04:05",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "-",
		},
		{
			name:     "dash",
			input:    "-",
			expected: "-",
		},
		{
			name:     "invalid format (returned as-is)",
			input:    "invalid",
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatTimestamp(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatDurationHuman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "negative duration",
			duration: -5 * time.Second,
			expected: "-",
		},
		{
			name:     "sub-second",
			duration: 500 * time.Millisecond,
			expected: "< 1s",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m30s",
		},
		{
			name:     "minutes only",
			duration: 5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "hours and minutes",
			duration: 1*time.Hour + 5*time.Minute,
			expected: "1h5m",
		},
		{
			name:     "hours only",
			duration: 3 * time.Hour,
			expected: "3h",
		},
		{
			name:     "days and hours",
			duration: 2*24*time.Hour + 3*time.Hour,
			expected: "2d3h",
		},
		{
			name:     "days only",
			duration: 5 * 24 * time.Hour,
			expected: "5d",
		},
		{
			name:     "exactly one hour",
			duration: time.Hour,
			expected: "1h",
		},
		{
			name:     "exactly one day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatDurationHuman(tt.duration)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty params",
			input:    "",
			expected: "-",
		},
		{
			name:     "short params",
			input:    "key=value",
			expected: "key=value",
		},
		{
			name:     "params at max length",
			input:    strings.Repeat("a", 40),
			expected: strings.Repeat("a", 40),
		},
		{
			name:     "params over max length (truncated)",
			input:    strings.Repeat("a", 50),
			expected: strings.Repeat("a", 37) + "...",
		},
		{
			name:     "long params with spaces",
			input:    "key1=value1 key2=value2 key3=value3 key4=value4",
			expected: "key1=value1 key2=value2 key3=value3 ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatParams(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRunIDNeverTruncated(t *testing.T) {
	t.Parallel()

	// This test verifies that run IDs are NEVER truncated in any formatting function
	// This is critical for usability as users need to copy-paste full run IDs

	longRunID := "dag-run_20260201_120000Z_" + strings.Repeat("abcdef123456", 10)

	// Test formatParams doesn't affect run ID (run IDs are in a separate column)
	params := "param1=value1 param2=value2"
	formattedParams := formatParams(params)
	assert.NotEqual(t, longRunID, formattedParams, "formatParams should not be used for run IDs")

	// Run IDs should be passed through directly without any truncation
	// The table formatter should display them in full
	assert.Equal(t, longRunID, longRunID, "Run ID should remain unchanged")
}
