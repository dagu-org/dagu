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

func TestKebabToCamel(t *testing.T) {
	t.Run("SimpleKebabCase", func(t *testing.T) {
		result := stringutil.KebabToCamel("hello-world")
		require.Equal(t, "helloWorld", result)
	})

	t.Run("MultipleWords", func(t *testing.T) {
		result := stringutil.KebabToCamel("hello-world-foo-bar")
		require.Equal(t, "helloWorldFooBar", result)
	})

	t.Run("SingleWord", func(t *testing.T) {
		result := stringutil.KebabToCamel("hello")
		require.Equal(t, "hello", result)
	})

	t.Run("EmptyString", func(t *testing.T) {
		result := stringutil.KebabToCamel("")
		require.Equal(t, "", result)
	})

	t.Run("LeadingHyphen", func(t *testing.T) {
		result := stringutil.KebabToCamel("-hello-world")
		require.Equal(t, "helloWorld", result)
	})

	t.Run("TrailingHyphen", func(t *testing.T) {
		result := stringutil.KebabToCamel("hello-world-")
		require.Equal(t, "helloWorld", result)
	})

	t.Run("MultipleConsecutiveHyphens", func(t *testing.T) {
		result := stringutil.KebabToCamel("hello--world")
		require.Equal(t, "helloWorld", result)
	})

	t.Run("AllUpperCase", func(t *testing.T) {
		result := stringutil.KebabToCamel("HELLO-WORLD")
		require.Equal(t, "HELLOWORLD", result)
	})

	t.Run("MixedCase", func(t *testing.T) {
		result := stringutil.KebabToCamel("Hello-World")
		require.Equal(t, "HelloWorld", result)
	})

	t.Run("NumbersInString", func(t *testing.T) {
		result := stringutil.KebabToCamel("test-123-case")
		require.Equal(t, "test123Case", result)
	})

	t.Run("SingleLetter", func(t *testing.T) {
		result := stringutil.KebabToCamel("a-b-c")
		require.Equal(t, "aBC", result)
	})

	t.Run("WithDots", func(t *testing.T) {
		result := stringutil.KebabToCamel("main-func.foo-bar")
		require.Equal(t, "mainFunc.fooBar", result)
	})

	t.Run("MultipleDots", func(t *testing.T) {
		result := stringutil.KebabToCamel("com.example.my-package.my-class")
		require.Equal(t, "com.example.myPackage.myClass", result)
	})

	t.Run("DotsOnly", func(t *testing.T) {
		result := stringutil.KebabToCamel("com.example.package")
		require.Equal(t, "com.example.package", result)
	})
}

func TestIsMultiLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Single line cases
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "single line text",
			input:    "hello world",
			expected: false,
		},
		{
			name:     "single word",
			input:    "hello",
			expected: false,
		},
		{
			name:     "single line with spaces",
			input:    "   hello world   ",
			expected: false,
		},
		{
			name:     "single line with special characters",
			input:    "hello@world.com",
			expected: false,
		},

		// Multi-line cases with Unix line endings (\n)
		{
			name:     "two lines with unix ending",
			input:    "line1\nline2",
			expected: true,
		},
		{
			name:     "multiple lines with unix endings",
			input:    "line1\nline2\nline3",
			expected: true,
		},
		{
			name:     "line ending at start",
			input:    "\nhello",
			expected: true,
		},
		{
			name:     "line ending at end",
			input:    "hello\n",
			expected: true,
		},
		{
			name:     "only newline character",
			input:    "\n",
			expected: true,
		},

		// Multi-line cases with Windows line endings (\r\n)
		{
			name:     "two lines with windows ending",
			input:    "line1\r\nline2",
			expected: true,
		},
		{
			name:     "multiple lines with windows endings",
			input:    "line1\r\nline2\r\nline3",
			expected: true,
		},
		{
			name:     "windows line ending at start",
			input:    "\r\nhello",
			expected: true,
		},
		{
			name:     "windows line ending at end",
			input:    "hello\r\n",
			expected: true,
		},

		// Multi-line cases with old Mac line endings (\r)
		{
			name:     "two lines with mac ending",
			input:    "line1\rline2",
			expected: true,
		},
		{
			name:     "multiple lines with mac endings",
			input:    "line1\rline2\rline3",
			expected: true,
		},
		{
			name:     "mac line ending at start",
			input:    "\rhello",
			expected: true,
		},
		{
			name:     "mac line ending at end",
			input:    "hello\r",
			expected: true,
		},
		{
			name:     "only carriage return character",
			input:    "\r",
			expected: true,
		},

		// Mixed line endings
		{
			name:     "mixed line endings",
			input:    "line1\nline2\r\nline3\rline4",
			expected: true,
		},
		{
			name:     "consecutive newlines",
			input:    "line1\n\nline3",
			expected: true,
		},
		{
			name:     "consecutive carriage returns",
			input:    "line1\r\rline3",
			expected: true,
		},

		// Edge cases
		{
			name:     "only whitespace with newline",
			input:    "   \n   ",
			expected: true,
		},
		{
			name:     "tabs and newlines",
			input:    "hello\tworld\nfoo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringutil.IsMultiLine(tt.input)
			require.Equal(t, tt.expected, result, "IsMultiLine(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}

func TestDetectSeparatorType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected stringutil.SeparatorType
	}{
		// JSON tests
		{
			name:     "json array",
			input:    `["item1", "item2", "item3"]`,
			expected: stringutil.SeparatorTypeJSON,
		},
		{
			name:     "json array with mixed types",
			input:    `["string", 123, true, null]`,
			expected: stringutil.SeparatorTypeJSON,
		},

		// Newline tests
		{
			name:     "unix newlines",
			input:    "line1\nline2\nline3",
			expected: stringutil.SeparatorTypeNewline,
		},
		{
			name:     "windows newlines",
			input:    "line1\r\nline2\r\nline3",
			expected: stringutil.SeparatorTypeNewline,
		},
		{
			name:     "mixed newlines",
			input:    "line1\nline2\r\nline3\rline4",
			expected: stringutil.SeparatorTypeNewline,
		},

		// Delimiter tests
		{
			name:     "comma separated",
			input:    "item1,item2,item3",
			expected: stringutil.SeparatorTypeComma,
		},
		{
			name:     "semicolon separated",
			input:    "item1;item2;item3",
			expected: stringutil.SeparatorTypeSemicolon,
		},
		{
			name:     "pipe separated",
			input:    "item1|item2|item3",
			expected: stringutil.SeparatorTypePipe,
		},
		{
			name:     "tab separated",
			input:    "item1\titem2\titem3",
			expected: stringutil.SeparatorTypeTab,
		},

		// Quoted strings tests
		{
			name:     "quoted strings with spaces",
			input:    `"item one" "item two" "item three"`,
			expected: stringutil.SeparatorTypeQuoted,
		},
		{
			name:     "mixed quoted and unquoted",
			input:    `"quoted item" unquoted`,
			expected: stringutil.SeparatorTypeQuoted,
		},

		// Space separated tests
		{
			name:     "simple space separated",
			input:    "item1 item2 item3",
			expected: stringutil.SeparatorTypeSpace,
		},
		{
			name:     "multiple spaces",
			input:    "item1   item2     item3",
			expected: stringutil.SeparatorTypeSpace,
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: stringutil.SeparatorTypeSpace,
		},
		{
			name:     "single item",
			input:    "single",
			expected: stringutil.SeparatorTypeSpace,
		},
		{
			name:     "whitespace only",
			input:    "   \t\n   ",
			expected: stringutil.SeparatorTypeSpace,
		},

		// Priority tests (comma should win over other separators)
		{
			name:     "comma wins over semicolon",
			input:    "a,b,c;d",
			expected: stringutil.SeparatorTypeComma,
		},
		{
			name:     "pipe wins over comma when more frequent",
			input:    "a|b|c|d,e",
			expected: stringutil.SeparatorTypePipe,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringutil.DetectSeparatorType(tt.input)
			require.Equal(t, tt.expected, result, "DetectSeparatorType(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}

func TestParseSeparatedValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		// JSON array tests
		{
			name:     "json array strings",
			input:    `["item1", "item2", "item3"]`,
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "json array with numbers",
			input:    `["string", 123, true, false, null]`,
			expected: []string{"string", "123", "true", "false", "null"},
			wantErr:  false,
		},
		{
			name:     "json array with objects",
			input:    `[{"key": "value"}, "string", 42]`,
			expected: []string{`{"key":"value"}`, "string", "42"},
			wantErr:  false,
		},
		{
			name:     "invalid json array detected as quoted",
			input:    `["unclosed array"`,
			expected: []string{`[`, `unclosed array`},
			wantErr:  false,
		},

		// Newline-separated tests
		{
			name:     "unix newlines",
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
			wantErr:  false,
		},
		{
			name:     "windows newlines",
			input:    "line1\r\nline2\r\nline3",
			expected: []string{"line1", "line2", "line3"},
			wantErr:  false,
		},
		{
			name:     "mixed newlines with empty lines",
			input:    "line1\n\nline3\r\n\r\nline5",
			expected: []string{"line1", "line3", "line5"},
			wantErr:  false,
		},
		{
			name:     "newlines with whitespace",
			input:    "  line1  \n\t\tline2\t\t\n   line3   ",
			expected: []string{"line1", "line2", "line3"},
			wantErr:  false,
		},

		// Comma-separated tests
		{
			name:     "simple comma separated",
			input:    "item1,item2,item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "comma separated with spaces",
			input:    "item1 , item2 , item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "comma separated with empty values",
			input:    "item1,,item3",
			expected: []string{"item1", "item3"},
			wantErr:  false,
		},

		// Other delimiter tests
		{
			name:     "semicolon separated",
			input:    "item1;item2;item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "pipe separated",
			input:    "item1|item2|item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "tab separated",
			input:    "item1\titem2\titem3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},

		// Quoted strings tests
		{
			name:     "quoted strings with spaces",
			input:    `"item one" "item two" "item three"`,
			expected: []string{"item one", "item two", "item three"},
			wantErr:  false,
		},
		{
			name:     "mixed quoted and unquoted",
			input:    `"quoted item" unquoted "another quoted"`,
			expected: []string{"quoted item", "unquoted", "another quoted"},
			wantErr:  false,
		},
		{
			name:     "quoted strings with escaped quotes (complex parsing)",
			input:    `"item with \"quotes\"" "normal item"`,
			expected: []string{`item with \`, `quotes\`, ``, `normal item`},
			wantErr:  false,
		},
		{
			name:     "unclosed quote falls back to space",
			input:    `"unclosed quote item`,
			expected: []string{`"unclosed`, `quote`, `item`},
			wantErr:  false,
		},

		// Space-separated tests
		{
			name:     "simple space separated",
			input:    "item1 item2 item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "multiple spaces",
			input:    "item1   item2     item3",
			expected: []string{"item1", "item2", "item3"},
			wantErr:  false,
		},

		// File path tests (real-world examples)
		{
			name:     "file paths with spaces (newline separated)",
			input:    "/path/to/my file.txt\n/another/path with spaces.csv\n/simple/path.json",
			expected: []string{"/path/to/my file.txt", "/another/path with spaces.csv", "/simple/path.json"},
			wantErr:  false,
		},
		{
			name:     "file paths with spaces (quoted)",
			input:    `"/path/to/my file.txt" "/another/path with spaces.csv" "/simple/path.json"`,
			expected: []string{"/path/to/my file.txt", "/another/path with spaces.csv", "/simple/path.json"},
			wantErr:  false,
		},
		{
			name:     "comma separated file paths",
			input:    "file1.txt,file2.csv,file3.json",
			expected: []string{"file1.txt", "file2.csv", "file3.json"},
			wantErr:  false,
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "whitespace only",
			input:    "   \t\n   ",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "single item",
			input:    "single",
			expected: []string{"single"},
			wantErr:  false,
		},
		{
			name:     "single quoted item",
			input:    `"single item"`,
			expected: []string{"single item"},
			wantErr:  false,
		},

		// Real-world command output examples
		{
			name:     "find command output",
			input:    "/usr/bin/find\n/usr/bin/grep\n/usr/bin/awk",
			expected: []string{"/usr/bin/find", "/usr/bin/grep", "/usr/bin/awk"},
			wantErr:  false,
		},
		{
			name:     "ls command output with spaces",
			input:    "My Document.pdf\nAnother File.txt\nno-spaces-file.json",
			expected: []string{"My Document.pdf", "Another File.txt", "no-spaces-file.json"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stringutil.ParseSeparatedValues(tt.input)
			if tt.wantErr {
				require.Error(t, err, "ParseSeparatedValues(%q) should return error", tt.input)
			} else {
				require.NoError(t, err, "ParseSeparatedValues(%q) should not return error", tt.input)
				require.Equal(t, tt.expected, result, "ParseSeparatedValues(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSeparatedValues_BackwardCompatibility(t *testing.T) {
	// These tests ensure backward compatibility with the existing strings.Fields behavior
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "space separated servers (existing behavior)",
			input:    "server1 server2 server3",
			expected: []string{"server1", "server2", "server3"},
		},
		{
			name:     "multiple whitespace with tabs",
			input:    "item1\t\titem2\t\titem3",
			expected: []string{"item1", "item2", "item3"},
		},
		{
			name:     "json array compatibility",
			input:    `["alpha", "beta", "gamma", "delta"]`,
			expected: []string{"alpha", "beta", "gamma", "delta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stringutil.ParseSeparatedValues(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result, "ParseSeparatedValues(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}

func TestRandomString(t *testing.T) {
	t.Run("GeneratesCorrectLength", func(t *testing.T) {
		// Test various lengths
		for _, length := range []int{0, 1, 5, 10, 32, 100} {
			result := stringutil.RandomString(length)
			require.Len(t, result, length, "expected string of length %d, got %d", length, len(result))
		}
	})

	t.Run("ContainsOnlyAlphabetic", func(t *testing.T) {
		// Generate a string and check all characters are alphabetic
		result := stringutil.RandomString(50)
		for i, ch := range result {
			require.True(t, (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z'),
				"character at position %d (%c) is not alphabetic", i, ch)
		}
	})

	t.Run("GeneratesDifferentStrings", func(t *testing.T) {
		// Generate multiple strings and check they're different
		// (technically could fail with extremely low probability)
		const numStrings = 10
		const stringLength = 10
		strings := make(map[string]bool)

		for i := 0; i < numStrings; i++ {
			s := stringutil.RandomString(stringLength)
			strings[s] = true
		}

		require.Greater(t, len(strings), 1, "expected different strings, but all were identical")
	})
}
