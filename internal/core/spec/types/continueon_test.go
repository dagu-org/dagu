package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContinueOnValue_UnmarshalYAML(t *testing.T) {
	t.Run("string skipped", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`skipped`), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
		assert.False(t, c.Failed())
		assert.False(t, c.IsZero())
	})

	t.Run("string failed", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`failed`), &c)
		require.NoError(t, err)
		assert.False(t, c.Skipped())
		assert.True(t, c.Failed())
	})

	t.Run("string case insensitive - SKIPPED", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`SKIPPED`), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
	})

	t.Run("string case insensitive - Failed", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`Failed`), &c)
		require.NoError(t, err)
		assert.True(t, c.Failed())
	})

	t.Run("string with whitespace", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`"  skipped  "`), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
	})

	t.Run("map form - skipped only", func(t *testing.T) {
		data := `skipped: true`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
		assert.False(t, c.Failed())
	})

	t.Run("map form - failed only", func(t *testing.T) {
		data := `failed: true`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.False(t, c.Skipped())
		assert.True(t, c.Failed())
	})

	t.Run("map form - both", func(t *testing.T) {
		data := `
skipped: true
failed: true
`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
		assert.True(t, c.Failed())
	})

	t.Run("map with exit codes array", func(t *testing.T) {
		data := `exitCode: [0, 1, 2]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{0, 1, 2}, c.ExitCode())
	})

	t.Run("map with single exit code", func(t *testing.T) {
		data := `exitCode: 1`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{1}, c.ExitCode())
	})

	t.Run("map with output pattern", func(t *testing.T) {
		data := `output: "success|warning"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []string{"success|warning"}, c.Output())
	})

	t.Run("map with all fields", func(t *testing.T) {
		data := `
skipped: true
failed: true
exitCode: [0, 1]
output: "OK"
markSuccess: true
`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.True(t, c.Skipped())
		assert.True(t, c.Failed())
		assert.Equal(t, []int{0, 1}, c.ExitCode())
		assert.Equal(t, []string{"OK"}, c.Output())
		assert.True(t, c.MarkSuccess())
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var c types.ContinueOnValue
		assert.True(t, c.IsZero())
		assert.False(t, c.Skipped())
		assert.False(t, c.Failed())
	})

	t.Run("invalid string value", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`invalid`), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 'skipped' or 'failed'")
	})

	t.Run("invalid map key", func(t *testing.T) {
		data := `unknown: true`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown key")
	})

	t.Run("invalid skipped type", func(t *testing.T) {
		data := `skipped: "yes"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected bool")
	})

	t.Run("invalid exit code type", func(t *testing.T) {
		data := `exitCode: "not a number"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
	})

	t.Run("invalid type - array", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`[1, 2, 3]`), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or map")
	})

	t.Run("output as string array", func(t *testing.T) {
		data := `output: ["success", "warning", "info"]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []string{"success", "warning", "info"}, c.Output())
	})

	t.Run("exit code as int64", func(t *testing.T) {
		data := `exitCode: 255`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{255}, c.ExitCode())
	})

	t.Run("exit code as string", func(t *testing.T) {
		data := `exitCode: "42"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{42}, c.ExitCode())
	})

	t.Run("exit code array with mixed types", func(t *testing.T) {
		data := `exitCode: [0, "1", 2]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{0, 1, 2}, c.ExitCode())
	})

	t.Run("output invalid type in array", func(t *testing.T) {
		data := `output: [123, true]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})

	t.Run("exit code invalid string", func(t *testing.T) {
		data := `exitCode: "not-a-number"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot parse")
	})
}

func TestContinueOnValue_InStruct(t *testing.T) {
	type StepConfig struct {
		Name       string                `yaml:"name"`
		ContinueOn types.ContinueOnValue `yaml:"continueOn"`
	}

	t.Run("continueOn as string", func(t *testing.T) {
		data := `
name: my-step
continueOn: skipped
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.ContinueOn.Skipped())
	})

	t.Run("continueOn as map", func(t *testing.T) {
		data := `
name: my-step
continueOn:
  failed: true
  exitCode: [0, 1]
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.ContinueOn.Failed())
		assert.Equal(t, []int{0, 1}, cfg.ContinueOn.ExitCode())
	})

	t.Run("continueOn not set", func(t *testing.T) {
		data := `name: my-step`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.ContinueOn.IsZero())
	})
}

func TestContinueOnValue_Value(t *testing.T) {
	t.Run("Value returns raw value", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`skipped`), &c)
		require.NoError(t, err)
		assert.Equal(t, "skipped", c.Value())
	})

	t.Run("Value returns map", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`failed: true`), &c)
		require.NoError(t, err)
		val, ok := c.Value().(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, val["failed"])
	})
}

func TestContinueOnValue_EdgeCases(t *testing.T) {
	t.Run("null value", func(t *testing.T) {
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(`null`), &c)
		require.NoError(t, err)
		assert.True(t, c.IsZero())
	})

	t.Run("failure key alias", func(t *testing.T) {
		// 'failure' is an alias for 'failed'
		data := `failure: true`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.True(t, c.Failed())
	})

	t.Run("exit code as float", func(t *testing.T) {
		data := `exitCode: 1.0`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{1}, c.ExitCode())
	})

	t.Run("exit code array with float", func(t *testing.T) {
		data := `exitCode: [1.0, 2.0]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, c.ExitCode())
	})

	t.Run("invalid exit code type in array", func(t *testing.T) {
		data := `exitCode: [true]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected int")
	})

	t.Run("invalid exit code type - not int or array", func(t *testing.T) {
		data := `exitCode: {key: value}`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected int or array")
	})

	t.Run("output as nil", func(t *testing.T) {
		data := `output: null`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Nil(t, c.Output())
	})

	t.Run("output as empty string", func(t *testing.T) {
		data := `output: ""`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.NoError(t, err)
		assert.Nil(t, c.Output())
	})

	t.Run("output invalid type - not string or array", func(t *testing.T) {
		data := `output: 123`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string or array")
	})

	t.Run("markSuccess invalid type", func(t *testing.T) {
		data := `markSuccess: "yes"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected bool")
	})

	t.Run("failed invalid type", func(t *testing.T) {
		data := `failed: "yes"`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected bool")
	})

	t.Run("exit code invalid string in array", func(t *testing.T) {
		data := `exitCode: ["not-a-number"]`
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte(data), &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot parse")
	})
}
