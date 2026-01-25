package types

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsValue_UnmarshalYAML_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		want    []TagEntry
		wantErr bool
	}{
		{
			name: "space-separated key=value",
			yaml: `"env=prod team=platform"`,
			want: []TagEntry{
				{key: "env", value: "prod"},
				{key: "team", value: "platform"},
			},
		},
		{
			name: "comma-separated simple tags (backward compatible)",
			yaml: `"production, critical, batch"`,
			want: []TagEntry{
				{key: "production", value: ""},
				{key: "critical", value: ""},
				{key: "batch", value: ""},
			},
		},
		{
			name: "comma-separated key=value",
			yaml: `"env=prod,team=platform"`,
			want: []TagEntry{
				{key: "env", value: "prod"},
				{key: "team", value: "platform"},
			},
		},
		{
			name: "single tag",
			yaml: `"production"`,
			want: []TagEntry{
				{key: "production", value: ""},
			},
		},
		{
			name: "single key=value",
			yaml: `"env=prod"`,
			want: []TagEntry{
				{key: "env", value: "prod"},
			},
		},
		{
			name: "empty string",
			yaml: `""`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v TagsValue
			err := yaml.Unmarshal([]byte(tt.yaml), &v)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, v.Entries())
		})
	}
}

func TestTagsValue_UnmarshalYAML_Map(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want map[string]string // Use map since order is not guaranteed
	}{
		{
			name: "simple map",
			yaml: `
env: prod
team: platform`,
			want: map[string]string{
				"env":  "prod",
				"team": "platform",
			},
		},
		{
			name: "map with empty value",
			yaml: `
env: prod
critical: ""`,
			want: map[string]string{
				"env":      "prod",
				"critical": "",
			},
		},
		{
			name: "map with numeric value",
			yaml: `
priority: 1
env: prod`,
			want: map[string]string{
				"priority": "1",
				"env":      "prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v TagsValue
			err := yaml.Unmarshal([]byte(tt.yaml), &v)
			require.NoError(t, err)

			// Convert to map for comparison (order not guaranteed)
			got := make(map[string]string)
			for _, entry := range v.Entries() {
				got[entry.Key()] = entry.Value()
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTagsValue_UnmarshalYAML_Array(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want []TagEntry
	}{
		{
			name: "array of key=value strings",
			yaml: `
- env=prod
- team=platform`,
			want: []TagEntry{
				{key: "env", value: "prod"},
				{key: "team", value: "platform"},
			},
		},
		{
			name: "array of simple tags (backward compatible)",
			yaml: `
- production
- critical
- batch`,
			want: []TagEntry{
				{key: "production", value: ""},
				{key: "critical", value: ""},
				{key: "batch", value: ""},
			},
		},
		{
			name: "mixed array",
			yaml: `
- env=prod
- critical
- team=platform`,
			want: []TagEntry{
				{key: "env", value: "prod"},
				{key: "critical", value: ""},
				{key: "team", value: "platform"},
			},
		},
		{
			name: "array with map entries",
			yaml: `
- env: prod
- team: platform`,
			want: []TagEntry{
				{key: "env", value: "prod"},
				{key: "team", value: "platform"},
			},
		},
		{
			name: "empty array",
			yaml: `[]`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v TagsValue
			err := yaml.Unmarshal([]byte(tt.yaml), &v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.Entries())
		})
	}
}

func TestTagsValue_UnmarshalYAML_Null(t *testing.T) {
	t.Parallel()

	var v TagsValue
	err := yaml.Unmarshal([]byte(`null`), &v)
	require.NoError(t, err)
	assert.True(t, v.IsZero())
	assert.Nil(t, v.Entries())
}

func TestTagsValue_UnmarshalYAML_NumericValues(t *testing.T) {
	t.Parallel()

	t.Run("numeric key in map gets stringified", func(t *testing.T) {
		var v TagsValue
		err := yaml.Unmarshal([]byte(`
456: value`), &v)
		require.NoError(t, err)
		assert.Equal(t, []TagEntry{{key: "456", value: "value"}}, v.Entries())
	})

	t.Run("array with numeric value returns error", func(t *testing.T) {
		var v TagsValue
		err := yaml.Unmarshal([]byte(`
- env=prod
- 123`), &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or string")
	})
}

func TestTagsValue_IsZero(t *testing.T) {
	t.Parallel()

	t.Run("unset", func(t *testing.T) {
		var v TagsValue
		assert.True(t, v.IsZero())
	})

	t.Run("set to empty", func(t *testing.T) {
		var v TagsValue
		_ = yaml.Unmarshal([]byte(`""`), &v)
		assert.False(t, v.IsZero())
		assert.True(t, v.IsEmpty())
	})

	t.Run("set with values", func(t *testing.T) {
		var v TagsValue
		_ = yaml.Unmarshal([]byte(`"env=prod"`), &v)
		assert.False(t, v.IsZero())
		assert.False(t, v.IsEmpty())
	})
}

func TestTagsValue_Value(t *testing.T) {
	t.Parallel()

	var v TagsValue
	_ = yaml.Unmarshal([]byte(`"env=prod team=platform"`), &v)

	raw := v.Value()
	assert.Equal(t, "env=prod team=platform", raw)
}

func TestTagsValue_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	// Test that all existing YAML formats continue to work
	tests := []struct {
		name string
		yaml string
		want []TagEntry
	}{
		{
			name: "old format: simple string array",
			yaml: `
- production
- staging
- critical`,
			want: []TagEntry{
				{key: "production", value: ""},
				{key: "staging", value: ""},
				{key: "critical", value: ""},
			},
		},
		{
			name: "old format: comma-separated string",
			yaml: `"production, staging, critical"`,
			want: []TagEntry{
				{key: "production", value: ""},
				{key: "staging", value: ""},
				{key: "critical", value: ""},
			},
		},
		{
			name: "old format: single tag",
			yaml: `production`,
			want: []TagEntry{
				{key: "production", value: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v TagsValue
			err := yaml.Unmarshal([]byte(tt.yaml), &v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.Entries())
		})
	}
}

func TestTagsValue_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		// Invalid key formats
		{
			name:    "key starts with dash",
			yaml:    `"-invalid"`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "key with space",
			yaml:    `"my tag"`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "key with special char",
			yaml:    `"my@tag"`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "map key starts with dash",
			yaml: `
-invalid: value`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "array key starts with dash",
			yaml: `
- -invalid`,
			wantErr: true,
			errMsg:  "invalid characters",
		},

		// Invalid value formats (use array/map to avoid space-separation parsing)
		{
			name: "value with space in array",
			yaml: `
- "env=my value"`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "value with special char",
			yaml:    `"env=prod@test"`,
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "map value with space",
			yaml: `
env: my value`,
			wantErr: true,
			errMsg:  "invalid characters",
		},

		// Valid cases should still work
		{
			name:    "valid key with dash",
			yaml:    `"my-tag"`,
			wantErr: false,
		},
		{
			name:    "valid key with underscore",
			yaml:    `"my_tag"`,
			wantErr: false,
		},
		{
			name:    "valid key with dot",
			yaml:    `"my.tag"`,
			wantErr: false,
		},
		{
			name:    "valid value with slash",
			yaml:    `"path=foo/bar"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v TagsValue
			err := yaml.Unmarshal([]byte(tt.yaml), &v)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
