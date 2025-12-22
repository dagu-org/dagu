package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringOrArray_UnmarshalYAML(t *testing.T) {
	t.Run("single string", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`step1`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1"}, s.Values())
		assert.False(t, s.IsEmpty())
		assert.False(t, s.IsZero())
	})

	t.Run("array of strings inline", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`["step1", "step2", "step3"]`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1", "step2", "step3"}, s.Values())
	})

	t.Run("multiline array", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte("- step1\n- step2"), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1", "step2"}, s.Values())
	})

	t.Run("empty string", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`""`), &s)
		require.NoError(t, err)
		// Empty string is preserved for validation layer to handle
		assert.Equal(t, []string{""}, s.Values())
		assert.False(t, s.IsEmpty()) // Has one element (empty string)
		assert.False(t, s.IsZero())  // Was set
	})

	t.Run("empty array", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`[]`), &s)
		require.NoError(t, err)
		assert.Empty(t, s.Values())
		assert.True(t, s.IsEmpty())
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var s types.StringOrArray
		assert.True(t, s.IsZero())
		assert.Nil(t, s.Values())
	})

	t.Run("invalid type map", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`{key: value}`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or array")
	})

	t.Run("quoted string with spaces", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`"step with spaces"`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"step with spaces"}, s.Values())
	})
}

func TestStringOrArray_InStruct(t *testing.T) {
	type StepConfig struct {
		Name    string              `yaml:"name"`
		Depends types.StringOrArray `yaml:"depends"`
	}

	t.Run("depends as string", func(t *testing.T) {
		data := `
name: step2
depends: step1
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1"}, cfg.Depends.Values())
	})

	t.Run("depends as array", func(t *testing.T) {
		data := `
name: step3
depends:
  - step1
  - step2
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1", "step2"}, cfg.Depends.Values())
	})

	t.Run("depends not set", func(t *testing.T) {
		data := `name: step1`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Depends.IsZero())
	})

	t.Run("depends empty array - explicitly no deps", func(t *testing.T) {
		data := `
name: step2
depends: []
`
		var cfg StepConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.False(t, cfg.Depends.IsZero()) // Was set
		assert.True(t, cfg.Depends.IsEmpty()) // But empty
	})
}

func TestMailToValue(t *testing.T) {
	// MailToValue is an alias for StringOrArray
	t.Run("single email", func(t *testing.T) {
		var m types.MailToValue
		err := yaml.Unmarshal([]byte(`user@example.com`), &m)
		require.NoError(t, err)
		assert.Equal(t, []string{"user@example.com"}, m.Values())
	})

	t.Run("multiple emails", func(t *testing.T) {
		var m types.MailToValue
		err := yaml.Unmarshal([]byte(`["user1@example.com", "user2@example.com"]`), &m)
		require.NoError(t, err)
		assert.Equal(t, []string{"user1@example.com", "user2@example.com"}, m.Values())
	})
}

func TestTagsValue(t *testing.T) {
	// TagsValue is an alias for StringOrArray
	t.Run("single tag", func(t *testing.T) {
		var tags types.TagsValue
		err := yaml.Unmarshal([]byte(`production`), &tags)
		require.NoError(t, err)
		assert.Equal(t, []string{"production"}, tags.Values())
	})

	t.Run("multiple tags", func(t *testing.T) {
		var tags types.TagsValue
		err := yaml.Unmarshal([]byte(`["production", "critical", "monitored"]`), &tags)
		require.NoError(t, err)
		assert.Equal(t, []string{"production", "critical", "monitored"}, tags.Values())
	})
}

func TestStringOrArray_AdditionalCoverage(t *testing.T) {
	t.Run("Value returns raw value - string", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`step1`), &s)
		require.NoError(t, err)
		assert.Equal(t, "step1", s.Value())
	})

	t.Run("Value returns raw value - array", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`["step1", "step2"]`), &s)
		require.NoError(t, err)
		val, ok := s.Value().([]any)
		require.True(t, ok)
		assert.Len(t, val, 2)
	})

	t.Run("null value sets isSet to false", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`null`), &s)
		require.NoError(t, err)
		assert.True(t, s.IsZero())
	})

	t.Run("array with numeric values - stringified", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`[1, 2, 3]`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"1", "2", "3"}, s.Values())
	})

	t.Run("array with mixed types - stringified", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`["step1", 123, true]`), &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"step1", "123", "true"}, s.Values())
	})

	t.Run("invalid type - number", func(t *testing.T) {
		var s types.StringOrArray
		err := yaml.Unmarshal([]byte(`123`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or array")
	})
}
