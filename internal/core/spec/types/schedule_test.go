package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleValue_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantErr         bool
		errContains     string
		wantStarts      []string
		wantStops       []string
		wantRestarts    []string
		wantHasStop     bool
		wantHasRestart  bool
		checkHasStop    bool
		checkHasRestart bool
	}{
		{
			name:            "SingleCronExpression",
			input:           `"0 * * * *"`,
			wantStarts:      []string{"0 * * * *"},
			checkHasStop:    true,
			wantHasStop:     false,
			checkHasRestart: true,
			wantHasRestart:  false,
		},
		{
			name:       "ArrayOfCronExpressions",
			input:      `["0 * * * *", "30 * * * *"]`,
			wantStarts: []string{"0 * * * *", "30 * * * *"},
		},
		{
			name: "MultilineArray",
			input: `
- "0 8 * * *"
- "0 12 * * *"
- "0 18 * * *"
`,
			wantStarts: []string{"0 8 * * *", "0 12 * * *", "0 18 * * *"},
		},
		{
			name:         "MapWithStartOnly",
			input:        `start: "0 8 * * *"`,
			wantStarts:   []string{"0 8 * * *"},
			checkHasStop: true,
			wantHasStop:  false,
		},
		{
			name: "MapWithStartAndStop",
			input: `
start: "0 8 * * *"
stop: "0 18 * * *"
`,
			wantStarts:   []string{"0 8 * * *"},
			wantStops:    []string{"0 18 * * *"},
			checkHasStop: true,
			wantHasStop:  true,
		},
		{
			name: "MapWithAllKeys",
			input: `
start: "0 8 * * *"
stop: "0 18 * * *"
restart: "0 0 * * *"
`,
			wantStarts:      []string{"0 8 * * *"},
			wantStops:       []string{"0 18 * * *"},
			wantRestarts:    []string{"0 0 * * *"},
			checkHasRestart: true,
			wantHasRestart:  true,
		},
		{
			name: "MapWithArrayValues",
			input: `
start:
  - "0 8 * * *"
  - "0 12 * * *"
stop: "0 18 * * *"
`,
			wantStarts: []string{"0 8 * * *", "0 12 * * *"},
			wantStops:  []string{"0 18 * * *"},
		},
		{
			name:        "InvalidMapKey",
			input:       `invalid: "0 * * * *"`,
			wantErr:     true,
			errContains: "unknown key",
		},
		{
			name: "InvalidArrayElementType",
			input: `
start:
  - 123
  - 456
`,
			wantErr:     true,
			errContains: "expected string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var s types.ScheduleValue
			err := yaml.Unmarshal([]byte(tt.input), &s)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			if tt.wantStarts != nil {
				assert.Equal(t, tt.wantStarts, s.Starts())
			}
			if tt.wantStops != nil {
				assert.Equal(t, tt.wantStops, s.Stops())
			}
			if tt.wantRestarts != nil {
				assert.Equal(t, tt.wantRestarts, s.Restarts())
			}
			if tt.checkHasStop {
				assert.Equal(t, tt.wantHasStop, s.HasStopSchedule())
			}
			if tt.checkHasRestart {
				assert.Equal(t, tt.wantHasRestart, s.HasRestartSchedule())
			}
		})
	}

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()
		var s types.ScheduleValue
		assert.True(t, s.IsZero())
		assert.Nil(t, s.Starts())
	})
}

func TestScheduleValue_InStruct(t *testing.T) {
	t.Parallel()

	type DAGConfig struct {
		Name     string              `yaml:"name"`
		Schedule types.ScheduleValue `yaml:"schedule"`
	}

	tests := []struct {
		name       string
		input      string
		wantName   string
		wantStarts []string
		wantStops  []string
		wantIsZero bool
	}{
		{
			name: "SimpleSchedule",
			input: `
name: my-dag
schedule: "0 * * * *"
`,
			wantName:   "my-dag",
			wantStarts: []string{"0 * * * *"},
		},
		{
			name: "ComplexSchedule",
			input: `
name: my-dag
schedule:
  start: "0 8 * * 1-5"
  stop: "0 18 * * 1-5"
`,
			wantStarts: []string{"0 8 * * 1-5"},
			wantStops:  []string{"0 18 * * 1-5"},
		},
		{
			name:       "NoSchedule",
			input:      "name: my-dag",
			wantIsZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg DAGConfig
			err := yaml.Unmarshal([]byte(tt.input), &cfg)
			require.NoError(t, err)
			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, cfg.Name)
			}
			if tt.wantStarts != nil {
				assert.Equal(t, tt.wantStarts, cfg.Schedule.Starts())
			}
			if tt.wantStops != nil {
				assert.Equal(t, tt.wantStops, cfg.Schedule.Stops())
			}
			if tt.wantIsZero {
				assert.True(t, cfg.Schedule.IsZero())
			}
		})
	}
}

func TestScheduleValue_AdditionalCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
		wantValue   any
		checkIsZero bool
		wantStarts  []string
	}{
		{
			name:      "ValueReturnsRawString",
			input:     `"0 * * * *"`,
			wantValue: "0 * * * *",
		},
		{
			name:        "NullValue",
			input:       "null",
			checkIsZero: true,
		},
		{
			name:       "EmptyString",
			input:      `""`,
			wantStarts: nil,
		},
		{
			name:        "InvalidTypeNumber",
			input:       "123",
			wantErr:     true,
			errContains: "must be string, array, or map",
		},
		{
			name:        "InvalidScheduleEntryTypeInMap",
			input:       "start: 123",
			wantErr:     true,
			errContains: "expected string, array, or object",
		},
		{
			name:        "InvalidTypeInStartArray",
			input:       `["0 * * * *", 123]`,
			wantErr:     true,
			errContains: "expected string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var s types.ScheduleValue
			err := yaml.Unmarshal([]byte(tt.input), &s)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			if tt.wantValue != nil {
				assert.Equal(t, tt.wantValue, s.Value())
			}
			if tt.checkIsZero {
				assert.True(t, s.IsZero())
			}
			if tt.wantStarts != nil {
				assert.Equal(t, tt.wantStarts, s.Starts())
			}
		})
	}

	t.Run("ValueReturnsRawArray", func(t *testing.T) {
		t.Parallel()
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`["0 * * * *"]`), &s)
		require.NoError(t, err)
		val, ok := s.Value().([]any)
		require.True(t, ok)
		assert.Len(t, val, 1)
	})

	t.Run("EmptyStringNotZero", func(t *testing.T) {
		t.Parallel()
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`""`), &s)
		require.NoError(t, err)
		assert.False(t, s.IsZero())
		assert.Nil(t, s.Starts())
	})
}

func TestScheduleValue_CatchupEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantErr       bool
		errContains   string
		wantEntries   int
		wantCatchup   string
		wantWindow    string
	}{
		{
			name:        "CatchupAll",
			input:       `{cron: "0 * * * *", catchup: all, catchupWindow: "6h"}`,
			wantEntries: 1,
			wantCatchup: "all",
			wantWindow:  "6h",
		},
		{
			name:        "CatchupLatest",
			input:       `{cron: "0 * * * *", catchup: latest}`,
			wantEntries: 1,
			wantCatchup: "latest",
		},
		{
			name:        "OldMisfireKeyRejected",
			input:       `{cron: "0 * * * *", misfire: runAll}`,
			wantErr:     true,
			errContains: "unknown schedule-entry key",
		},
		{
			name:        "OldMaxCatchupRunsKeyRejected",
			input:       `{cron: "0 * * * *", catchup: all, maxCatchupRuns: 10}`,
			wantErr:     true,
			errContains: "unknown schedule-entry key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var s types.ScheduleValue
			err := yaml.Unmarshal([]byte(tt.input), &s)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			entries := s.StartEntries()
			assert.Len(t, entries, tt.wantEntries)
			if tt.wantEntries > 0 {
				if tt.wantCatchup != "" {
					assert.Equal(t, tt.wantCatchup, entries[0].Catchup)
				}
				if tt.wantWindow != "" {
					assert.Equal(t, tt.wantWindow, entries[0].CatchupWindow)
				}
			}
		})
	}
}
