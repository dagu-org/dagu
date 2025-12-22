package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvValue_UnmarshalYAML(t *testing.T) {
	t.Run("map form", func(t *testing.T) {
		data := `
KEY1: value1
KEY2: value2
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		assert.Len(t, entries, 2)
		// Note: map order is not guaranteed, so we check by content
		keys := make(map[string]string)
		for _, entry := range entries {
			keys[entry.Key] = entry.Value
		}
		assert.Equal(t, "value1", keys["KEY1"])
		assert.Equal(t, "value2", keys["KEY2"])
	})

	t.Run("array of maps - preserves order", func(t *testing.T) {
		data := `
- KEY1: value1
- KEY2: value2
- KEY3: value3
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		require.Len(t, entries, 3)
		assert.Equal(t, "KEY1", entries[0].Key)
		assert.Equal(t, "value1", entries[0].Value)
		assert.Equal(t, "KEY2", entries[1].Key)
		assert.Equal(t, "value2", entries[1].Value)
		assert.Equal(t, "KEY3", entries[2].Key)
		assert.Equal(t, "value3", entries[2].Value)
	})

	t.Run("array of strings", func(t *testing.T) {
		data := `
- KEY1=value1
- KEY2=value2
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		require.Len(t, entries, 2)
		assert.Equal(t, "KEY1", entries[0].Key)
		assert.Equal(t, "value1", entries[0].Value)
	})

	t.Run("mixed array - maps and strings", func(t *testing.T) {
		data := `
- KEY1: value1
- KEY2=value2
- KEY3: value3
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		require.Len(t, entries, 3)
	})

	t.Run("numeric values are stringified", func(t *testing.T) {
		data := `
PORT: 8080
ENABLED: true
RATIO: 0.5
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		assert.Len(t, entries, 3)
		// Values should be stringified
		keys := make(map[string]string)
		for _, entry := range entries {
			keys[entry.Key] = entry.Value
		}
		assert.Equal(t, "8080", keys["PORT"])
		assert.Equal(t, "true", keys["ENABLED"])
		assert.Equal(t, "0.5", keys["RATIO"])
	})

	t.Run("value with equals sign", func(t *testing.T) {
		data := `
- CONNECTION_STRING=host=localhost;port=5432
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, "CONNECTION_STRING", entries[0].Key)
		assert.Equal(t, "host=localhost;port=5432", entries[0].Value)
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var e types.EnvValue
		assert.True(t, e.IsZero())
		assert.Nil(t, e.Entries())
	})

	t.Run("invalid string format - no equals", func(t *testing.T) {
		data := `["invalid_no_equals"]`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected KEY=value")
	})

	t.Run("invalid type - scalar string", func(t *testing.T) {
		data := `"just a string"`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be map or array")
	})

	t.Run("empty map", func(t *testing.T) {
		data := `{}`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		assert.False(t, e.IsZero()) // Was set
		assert.Empty(t, e.Entries())
	})

	t.Run("empty array", func(t *testing.T) {
		data := `[]`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		assert.False(t, e.IsZero()) // Was set
		assert.Empty(t, e.Entries())
	})

	t.Run("environment variable reference", func(t *testing.T) {
		data := `
- PATH: ${HOME}/bin
- DERIVED: ${OTHER_VAR}
`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		entries := e.Entries()
		require.Len(t, entries, 2)
		// Values are preserved as-is; expansion happens later
		assert.Equal(t, "${HOME}/bin", entries[0].Value)
		assert.Equal(t, "${OTHER_VAR}", entries[1].Value)
	})
}

func TestEnvValue_InStruct(t *testing.T) {
	type StepConfig struct {
		Name string         `yaml:"name"`
		Env  types.EnvValue `yaml:"env"`
	}

	t.Run("env set as map", func(t *testing.T) {
		data := `
name: my-step
env:
  DEBUG: "true"
  LOG_LEVEL: info
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-step", cfg.Name)
		assert.False(t, cfg.Env.IsZero())
		assert.Len(t, cfg.Env.Entries(), 2)
	})

	t.Run("env set as array", func(t *testing.T) {
		data := `
name: my-step
env:
  - DEBUG: "true"
  - LOG_LEVEL: info
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		entries := cfg.Env.Entries()
		require.Len(t, entries, 2)
		assert.Equal(t, "DEBUG", entries[0].Key)
		assert.Equal(t, "LOG_LEVEL", entries[1].Key)
	})

	t.Run("env not set", func(t *testing.T) {
		data := `name: my-step`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Env.IsZero())
	})
}

func TestEnvValue_AdditionalCoverage(t *testing.T) {
	t.Run("Value returns raw value - map", func(t *testing.T) {
		data := `KEY: value`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		val, ok := e.Value().(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "value", val["KEY"])
	})

	t.Run("Value returns raw value - array", func(t *testing.T) {
		data := `[KEY=value]`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		val, ok := e.Value().([]any)
		require.True(t, ok)
		assert.Len(t, val, 1)
	})

	t.Run("null value sets isSet to false", func(t *testing.T) {
		data := `null`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.NoError(t, err)
		assert.True(t, e.IsZero())
	})

	t.Run("invalid type in array - number", func(t *testing.T) {
		data := `[123]`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or string")
	})

	t.Run("invalid type in array - boolean", func(t *testing.T) {
		data := `[true]`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or string")
	})

	t.Run("invalid type - number", func(t *testing.T) {
		data := `123`
		var e types.EnvValue
		err := yaml.Unmarshal([]byte(data), &e)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be map or array")
	})
}
