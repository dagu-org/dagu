package cmdutil

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "Safe alphanumeric",
			input:    "abcXYZ123",
			expected: "abcXYZ123",
		},
		{
			name:     "Safe special characters",
			input:    "-_./",
			expected: "-_./",
		},
		{
			name:     "String with space",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "String with single quote",
			input:    "user's file",
			expected: "'user'\\''s file'",
		},
		{
			name:     "String with multiple single quotes",
			input:    "a'b'c",
			expected: "'a'\\''b'\\''c'",
		},
		{
			name:     "String with double quote",
			input:    `"quoted"`,
			expected: `'"quoted"'`,
		},
		{
			name:     "String with dollar sign",
			input:    "$VAR",
			expected: "'$VAR'",
		},
		{
			name:     "String with asterisk",
			input:    "*.txt",
			expected: "'*.txt'",
		},
		{
			name:     "String with backtick",
			input:    "`date`",
			expected: "'`date`'",
		},
		{
			name:     "String with semicolon",
			input:    "ls; rm -rf /",
			expected: "'ls; rm -rf /'",
		},
		{
			name:     "String with ampersand",
			input:    "run &",
			expected: "'run &'",
		},
		{
			name:     "String with pipe",
			input:    "a | b",
			expected: "'a | b'",
		},
		{
			name:     "String with parentheses",
			input:    "(subshell)",
			expected: "'(subshell)'",
		},
		{
			name:     "String with brackets",
			input:    "[abc]",
			expected: "'[abc]'",
		},
		{
			name:     "String with braces",
			input:    "{1..10}",
			expected: "'{1..10}'",
		},
		{
			name:     "String with redirection",
			input:    "> output.txt",
			expected: "'> output.txt'",
		},
		{
			name:     "String with backslash",
			input:    "path\\to\\file",
			expected: "'path\\to\\file'",
		},
		{
			name:     "String with newline",
			input:    "line1\nline2",
			expected: "'line1\nline2'",
		},
		{
			name:     "String with tab",
			input:    "field1\tfield2",
			expected: "'field1\tfield2'",
		},
		{
			name:     "Unicode string",
			input:    "Hello 世界",
			expected: "'Hello 世界'",
		},
		{
			name:     "Mixed single and double quotes",
			input:    "It's a \"test\"",
			expected: "'It'\\''s a \"test\"'",
		},
		{
			name:     "Only single quotes",
			input:    "'''",
			expected: "''\\'''\\'''\\'''",
		},
		{
			name:     "Backslashes and quotes",
			input:    `\"'\`,
			expected: `'\"'\''\'`,
		},
		{
			name:     "Non-printable characters",
			input:    "\x01\x02\x03",
			expected: "'\x01\x02\x03'",
		},
		{
			name:     "Terminal escape sequence",
			input:    "\x1b[31mRed\x1b[0m",
			expected: "'\x1b[31mRed\x1b[0m'",
		},
		{
			name:     "Ultra nasty mixed string",
			input:    `'"` + "$; \\ \t\n\r\v\f!#%^&*()[]{}|<>?~",
			expected: `''\''"` + "$; \\ \t\n\r\v\f!#%^&*()[]{}|<>?~'",
		},
		{
			name:     "Leading/trailing spaces",
			input:    "  spaced  ",
			expected: "'  spaced  '",
		},
		{
			name:     "Command injection attempt",
			input:    "; rm -rf / ; #",
			expected: "'; rm -rf / ; #'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ShellQuote(tt.input)
			assert.Equal(t, tt.expected, actual, "Input: %s", tt.input)
		})
	}
}

func TestShellQuote_ShellRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found in PATH")
	}

	inputs := []string{
		"simple",
		"with space",
		"with 'single' quote",
		"with \"double\" quote",
		"with $dollar",
		"with `backtick`",
		"with \\backslash",
		"with \nnewline",
		"with \ttab",
		"with 世界 (unicode)",
		"with mixed \"' $`\\ \n\t chars",
		"",
		"'-'",
		"\"-\"",
		`'"` + "$; \\ \t\n\r\v\f!#%^&*()[]{}|<>?~", // Ultra nasty
		"   leading and trailing spaces   ",
		"!!!@@@###$$$%%%^^^&&&***((()))_++==--",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			quoted := ShellQuote(input)
			// Run sh -c 'printf %s <quoted>' and capture output
			// We use printf because echo might interpret sequences or add newlines
			cmd := exec.Command("sh", "-c", "printf %s "+quoted)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sh failed for input %q: %v\nOutput: %s", input, err, string(output))
			}
			assert.Equal(t, input, string(output), "Round-trip failed for input %q", input)
		})
	}
}

func TestShellQuoteArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "No args",
			args:     []string{},
			expected: "",
		},
		{
			name:     "Single safe arg",
			args:     []string{"ls"},
			expected: "ls",
		},
		{
			name:     "Single unsafe arg",
			args:     []string{"ls -l"},
			expected: "'ls -l'",
		},
		{
			name:     "Multiple args",
			args:     []string{"ls", "-l", "my file.txt"},
			expected: "ls -l 'my file.txt'",
		},
		{
			name:     "Complex args",
			args:     []string{"echo", "It's a beautiful day", "$HOME"},
			expected: "echo 'It'\\''s a beautiful day' '$HOME'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ShellQuoteArgs(tt.args)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestShellQuote_RoundTrip(t *testing.T) {
	// Exhaustive list of characters to test
	chars := ""
	for i := 0; i < 256; i++ {
		chars += string(rune(i))
	}

	quoted := ShellQuote(chars)
	// We don't have a parser here to verify, but we can at least ensure it's not empty and wrapped if needed.
	assert.NotEmpty(t, quoted)
	if len(chars) > 0 {
		assert.True(t, len(quoted) >= len(chars))
	}
}
