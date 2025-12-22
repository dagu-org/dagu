package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleValue_UnmarshalYAML(t *testing.T) {
	t.Run("single cron expression", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`"0 * * * *"`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 * * * *"}, s.Starts())
		assert.Empty(t, s.Stops())
		assert.Empty(t, s.Restarts())
		assert.False(t, s.HasStopSchedule())
		assert.False(t, s.HasRestartSchedule())
	})

	t.Run("array of cron expressions", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`["0 * * * *", "30 * * * *"]`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 * * * *", "30 * * * *"}, s.Starts())
	})

	t.Run("multiline array", func(t *testing.T) {
		data := `
- "0 8 * * *"
- "0 12 * * *"
- "0 18 * * *"
`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * *", "0 12 * * *", "0 18 * * *"}, s.Starts())
	})

	t.Run("map with start only", func(t *testing.T) {
		data := `start: "0 8 * * *"`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * *"}, s.Starts())
		assert.False(t, s.HasStopSchedule())
	})

	t.Run("map with start and stop", func(t *testing.T) {
		data := `
start: "0 8 * * *"
stop: "0 18 * * *"
`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * *"}, s.Starts())
		assert.Equal(t, []string{"0 18 * * *"}, s.Stops())
		assert.True(t, s.HasStopSchedule())
	})

	t.Run("map with all keys", func(t *testing.T) {
		data := `
start: "0 8 * * *"
stop: "0 18 * * *"
restart: "0 0 * * *"
`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * *"}, s.Starts())
		assert.Equal(t, []string{"0 18 * * *"}, s.Stops())
		assert.Equal(t, []string{"0 0 * * *"}, s.Restarts())
		assert.True(t, s.HasRestartSchedule())
	})

	t.Run("map with array values", func(t *testing.T) {
		data := `
start:
  - "0 8 * * *"
  - "0 12 * * *"
stop: "0 18 * * *"
`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * *", "0 12 * * *"}, s.Starts())
		assert.Equal(t, []string{"0 18 * * *"}, s.Stops())
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var s types.ScheduleValue
		assert.True(t, s.IsZero())
		assert.Nil(t, s.Starts())
	})

	t.Run("invalid map key", func(t *testing.T) {
		data := `invalid: "0 * * * *"`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown key")
	})

	t.Run("invalid array element type", func(t *testing.T) {
		data := `
start:
  - 123
  - 456
`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})
}

func TestScheduleValue_InStruct(t *testing.T) {
	type DAGConfig struct {
		Name     string              `yaml:"name"`
		Schedule types.ScheduleValue `yaml:"schedule"`
	}

	t.Run("simple schedule", func(t *testing.T) {
		data := `
name: my-dag
schedule: "0 * * * *"
`
		var cfg DAGConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-dag", cfg.Name)
		assert.Equal(t, []string{"0 * * * *"}, cfg.Schedule.Starts())
	})

	t.Run("complex schedule", func(t *testing.T) {
		data := `
name: my-dag
schedule:
  start: "0 8 * * 1-5"
  stop: "0 18 * * 1-5"
`
		var cfg DAGConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"0 8 * * 1-5"}, cfg.Schedule.Starts())
		assert.Equal(t, []string{"0 18 * * 1-5"}, cfg.Schedule.Stops())
	})

	t.Run("no schedule", func(t *testing.T) {
		data := `name: my-dag`
		var cfg DAGConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Schedule.IsZero())
	})
}

func TestScheduleValue_AdditionalCoverage(t *testing.T) {
	t.Run("Value returns raw value - string", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`"0 * * * *"`), &s)
		require.NoError(t, err)
		assert.Equal(t, "0 * * * *", s.Value())
	})

	t.Run("Value returns raw value - array", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`["0 * * * *"]`), &s)
		require.NoError(t, err)
		val, ok := s.Value().([]any)
		require.True(t, ok)
		assert.Len(t, val, 1)
	})

	t.Run("null value sets isSet to false", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`null`), &s)
		require.NoError(t, err)
		assert.True(t, s.IsZero())
	})

	t.Run("empty string", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`""`), &s)
		require.NoError(t, err)
		assert.False(t, s.IsZero())
		assert.Nil(t, s.Starts())
	})

	t.Run("invalid type - number", func(t *testing.T) {
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(`123`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string, array, or map")
	})

	t.Run("invalid schedule entry type in map", func(t *testing.T) {
		data := `start: 123`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string or array")
	})

	t.Run("invalid type in start array", func(t *testing.T) {
		data := `["0 * * * *", 123]`
		var s types.ScheduleValue
		err := yaml.Unmarshal([]byte(data), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})
}
