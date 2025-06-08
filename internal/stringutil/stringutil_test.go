package stringutil_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/stretchr/testify/require"
)

func Test_FormatTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tm := time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC)
		formatted := stringutil.FormatTime(tm)
		require.Equal(t, "2022-02-01T02:02:02Z", formatted)

		parsed, err := stringutil.ParseTime(formatted)
		require.NoError(t, err)
		require.Equal(t, tm, parsed)

		// Test empty time
		require.Equal(t, "", stringutil.FormatTime(time.Time{}))
		parsed, err = stringutil.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
	t.Run("Empty", func(t *testing.T) {
		// Test empty time
		require.Equal(t, "", stringutil.FormatTime(time.Time{}))
	})
}

func Test_ParseTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("2022-02-01T02:02:02Z")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC), parsed)
	})
	t.Run("Valid_Legacy", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("2022-02-01 02:02:02")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location()), parsed)
	})

	t.Run("Empty", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
}

func TestTruncString(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		// Test empty string
		require.Equal(t, "", stringutil.TruncString("", 8))
		// Test string with length less than limit
		require.Equal(t, "1234567", stringutil.TruncString("1234567", 8))
		// Test string with length equal to limit
		require.Equal(t, "12345678", stringutil.TruncString("123456789", 8))
	})
}

func TestRemoveQuotes(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		require.Equal(t, "hello", stringutil.RemoveQuotes(`"hello"`))
		require.Equal(t, "hello world", stringutil.RemoveQuotes(`"hello world"`))
		require.Equal(t, "hello", stringutil.RemoveQuotes(`hello`))
		require.Equal(t, "", stringutil.RemoveQuotes(``))
	})
}

func TestIsJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid JSON arrays
		{
			name:     "empty array",
			input:    "[]",
			expected: true,
		},
		{
			name:     "array with strings",
			input:    `["item1", "item2", "item3"]`,
			expected: true,
		},
		{
			name:     "array with numbers",
			input:    "[1, 2, 3, 4, 5]",
			expected: true,
		},
		{
			name:     "array with objects",
			input:    `[{"key": "value"}, {"key2": "value2"}]`,
			expected: true,
		},
		{
			name:     "array with mixed types",
			input:    `["string", 123, true, null, {"key": "value"}]`,
			expected: true,
		},
		{
			name:     "array with whitespace",
			input:    "  [ 1 , 2 , 3 ]  ",
			expected: true,
		},
		{
			name:     "nested arrays",
			input:    `[["a", "b"], ["c", "d"]]`,
			expected: true,
		},
		
		// Invalid cases
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "single bracket",
			input:    "[",
			expected: false,
		},
		{
			name:     "space separated items",
			input:    "item1 item2 item3",
			expected: false,
		},
		{
			name:     "shell command with brackets",
			input:    "ls [abc]*.txt",
			expected: false,
		},
		{
			name:     "invalid JSON array - missing comma",
			input:    `["item1" "item2"]`,
			expected: false,
		},
		{
			name:     "invalid JSON array - trailing comma",
			input:    `["item1", "item2",]`,
			expected: false,
		},
		{
			name:     "JSON object not array",
			input:    `{"key": "value"}`,
			expected: false,
		},
		{
			name:     "string that starts with bracket",
			input:    `[hello world`,
			expected: false,
		},
		{
			name:     "string that ends with bracket",
			input:    `hello world]`,
			expected: false,
		},
		{
			name:     "malformed JSON",
			input:    `[{"key": "value"`,
			expected: false,
		},
		{
			name:     "array with unquoted strings",
			input:    `[item1, item2, item3]`,
			expected: false,
		},
		{
			name:     "text with brackets but not JSON",
			input:    "[this is not json]",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringutil.IsJSONArray(tt.input)
			require.Equal(t, tt.expected, result, "IsJSONArray(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}
