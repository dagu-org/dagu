package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
		wantStr string
	}{
		{
			name:    "simple tag",
			input:   "production",
			wantKey: "production",
			wantVal: "",
			wantStr: "production",
		},
		{
			name:    "key=value tag",
			input:   "env=prod",
			wantKey: "env",
			wantVal: "prod",
			wantStr: "env=prod",
		},
		{
			name:    "uppercase normalized to lowercase",
			input:   "ENV=PROD",
			wantKey: "env",
			wantVal: "prod",
			wantStr: "env=prod",
		},
		{
			name:    "spaces trimmed",
			input:   "  env = production  ",
			wantKey: "env",
			wantVal: "production",
			wantStr: "env=production",
		},
		{
			name:    "empty value",
			input:   "env=",
			wantKey: "env",
			wantVal: "",
			wantStr: "env",
		},
		{
			name:    "empty string",
			input:   "",
			wantKey: "",
			wantVal: "",
			wantStr: "",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantKey: "",
			wantVal: "",
			wantStr: "",
		},
		{
			name:    "value with equals sign",
			input:   "config=key=value",
			wantKey: "config",
			wantVal: "key=value",
			wantStr: "config=key=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := ParseTag(tt.input)
			assert.Equal(t, tt.wantKey, tag.Key)
			assert.Equal(t, tt.wantVal, tag.Value)
			if tt.wantKey != "" {
				assert.Equal(t, tt.wantStr, tag.String())
			}
		})
	}
}

func TestTag_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  Tag
		want bool
	}{
		{"empty tag", Tag{}, true},
		{"key only", Tag{Key: "env"}, false},
		{"key and value", Tag{Key: "env", Value: "prod"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tag.IsZero())
		})
	}
}

func TestNewTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		wantLen  int
		wantStrs []string
	}{
		{
			name:     "mixed tags",
			input:    []string{"env=prod", "team=platform", "critical"},
			wantLen:  3,
			wantStrs: []string{"env=prod", "team=platform", "critical"},
		},
		{
			name:     "filters empty strings",
			input:    []string{"env=prod", "", "  ", "critical"},
			wantLen:  2,
			wantStrs: []string{"env=prod", "critical"},
		},
		{
			name:     "nil input",
			input:    nil,
			wantLen:  0,
			wantStrs: []string{},
		},
		{
			name:     "empty input",
			input:    []string{},
			wantLen:  0,
			wantStrs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := NewTags(tt.input)
			assert.Len(t, tags, tt.wantLen)
			assert.Equal(t, tt.wantStrs, tags.Strings())
		})
	}
}

func TestTags_Keys(t *testing.T) {
	t.Parallel()

	tags := Tags{
		{Key: "env", Value: "prod"},
		{Key: "env", Value: "staging"},
		{Key: "team", Value: "platform"},
		{Key: "critical", Value: ""},
	}

	keys := tags.Keys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "env")
	assert.Contains(t, keys, "team")
	assert.Contains(t, keys, "critical")
}

func TestTags_HasKey(t *testing.T) {
	t.Parallel()

	tags := Tags{
		{Key: "env", Value: "prod"},
		{Key: "team", Value: "platform"},
	}

	assert.True(t, tags.HasKey("env"))
	assert.True(t, tags.HasKey("ENV"))
	assert.True(t, tags.HasKey("team"))
	assert.False(t, tags.HasKey("missing"))
}

func TestTags_Get(t *testing.T) {
	t.Parallel()

	tags := Tags{
		{Key: "env", Value: "prod"},
		{Key: "env", Value: "staging"},
		{Key: "team", Value: "platform"},
		{Key: "critical", Value: ""},
	}

	envValues := tags.Get("env")
	assert.Len(t, envValues, 2)
	assert.Contains(t, envValues, "prod")
	assert.Contains(t, envValues, "staging")

	teamValues := tags.Get("team")
	assert.Len(t, teamValues, 1)
	assert.Equal(t, "platform", teamValues[0])

	criticalValues := tags.Get("critical")
	assert.Len(t, criticalValues, 1)
	assert.Equal(t, "", criticalValues[0])

	missingValues := tags.Get("missing")
	assert.Nil(t, missingValues)
}

func TestTags_JSON(t *testing.T) {
	t.Parallel()

	t.Run("marshal", func(t *testing.T) {
		tags := Tags{
			{Key: "env", Value: "prod"},
			{Key: "critical", Value: ""},
		}

		data, err := json.Marshal(tags)
		require.NoError(t, err)
		assert.JSONEq(t, `["env=prod","critical"]`, string(data))
	})

	t.Run("unmarshal", func(t *testing.T) {
		data := []byte(`["env=prod","team=platform","critical"]`)

		var tags Tags
		err := json.Unmarshal(data, &tags)
		require.NoError(t, err)

		assert.Len(t, tags, 3)
		assert.Equal(t, "env", tags[0].Key)
		assert.Equal(t, "prod", tags[0].Value)
		assert.Equal(t, "team", tags[1].Key)
		assert.Equal(t, "platform", tags[1].Value)
		assert.Equal(t, "critical", tags[2].Key)
		assert.Equal(t, "", tags[2].Value)
	})

	t.Run("nil tags marshal", func(t *testing.T) {
		var tags Tags
		data, err := json.Marshal(tags)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})
}

func TestParseTagFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantType  TagFilterType
		wantKey   string
		wantValue string
	}{
		{
			name:      "key only",
			input:     "env",
			wantType:  TagFilterTypeKeyOnly,
			wantKey:   "env",
			wantValue: "",
		},
		{
			name:      "exact match",
			input:     "env=prod",
			wantType:  TagFilterTypeExact,
			wantKey:   "env",
			wantValue: "prod",
		},
		{
			name:      "negation",
			input:     "!deprecated",
			wantType:  TagFilterTypeNegation,
			wantKey:   "deprecated",
			wantValue: "",
		},
		{
			name:      "case normalized",
			input:     "ENV=PROD",
			wantType:  TagFilterTypeExact,
			wantKey:   "env",
			wantValue: "prod",
		},
		{
			name:      "spaces trimmed",
			input:     "  env = prod  ",
			wantType:  TagFilterTypeExact,
			wantKey:   "env",
			wantValue: "prod",
		},
		{
			name:      "negation with spaces",
			input:     "! deprecated ",
			wantType:  TagFilterTypeNegation,
			wantKey:   "deprecated",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := ParseTagFilter(tt.input)
			assert.Equal(t, tt.wantType, filter.Type)
			assert.Equal(t, tt.wantKey, filter.Key)
			assert.Equal(t, tt.wantValue, filter.Value)
		})
	}
}

func TestTagFilter_MatchesTags(t *testing.T) {
	t.Parallel()

	tags := Tags{
		{Key: "env", Value: "prod"},
		{Key: "team", Value: "platform"},
		{Key: "critical", Value: ""},
	}

	tests := []struct {
		name   string
		filter string
		want   bool
	}{
		// Key-only filters
		{"key exists", "env", true},
		{"key exists (uppercase)", "ENV", true},
		{"key missing", "missing", false},
		{"key-only tag exists", "critical", true},

		// Exact match filters
		{"exact match", "env=prod", true},
		{"exact match (wrong value)", "env=staging", false},
		{"exact match (key-only tag)", "critical=", true},
		{"exact match (missing key)", "missing=value", false},

		// Negation filters
		{"negation (key missing)", "!deprecated", true},
		{"negation (key exists)", "!env", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := ParseTagFilter(tt.filter)
			assert.Equal(t, tt.want, filter.MatchesTags(tags))
		})
	}
}

func TestTags_MatchesFilters(t *testing.T) {
	t.Parallel()

	tags := Tags{
		{Key: "env", Value: "prod"},
		{Key: "team", Value: "platform"},
		{Key: "critical", Value: ""},
	}

	tests := []struct {
		name    string
		filters []string
		want    bool
	}{
		{
			name:    "no filters",
			filters: []string{},
			want:    true,
		},
		{
			name:    "single match",
			filters: []string{"env=prod"},
			want:    true,
		},
		{
			name:    "multiple matches (AND)",
			filters: []string{"env=prod", "team"},
			want:    true,
		},
		{
			name:    "one filter fails (AND)",
			filters: []string{"env=prod", "missing"},
			want:    false,
		},
		{
			name:    "complex filter",
			filters: []string{"env=prod", "team", "!deprecated"},
			want:    true,
		},
		{
			name:    "negation fails",
			filters: []string{"env=prod", "!env"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := make([]TagFilter, len(tt.filters))
			for i, f := range tt.filters {
				filters[i] = ParseTagFilter(f)
			}
			assert.Equal(t, tt.want, tags.MatchesFilters(filters))
		})
	}
}
