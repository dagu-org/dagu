package eval

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString_DollarEscape(t *testing.T) {
	ctx := context.Background()
	opts := []Option{WithVariables(map[string]string{
		"HOME": "/tmp/home",
		"REAL": "value",
	})}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "BasicEscape",
			input:    "$$HOME",
			expected: "$HOME",
		},
		{
			name:     "PriceLiteral",
			input:    "Price: $$9.99",
			expected: "Price: $9.99",
		},
		{
			name:     "PasswordLiteral",
			input:    "p@ss$$word",
			expected: "p@ss$word",
		},
		{
			name:     "DoubleEscape",
			input:    "$$$$",
			expected: "$$",
		},
		{
			name:     "MixedEscapeAndExpand",
			input:    "$$$HOME",
			expected: "$/tmp/home",
		},
		{
			name:     "EscapedBraces",
			input:    "$${REAL}",
			expected: "${REAL}",
		},
		{
			name:     "RegularExpand",
			input:    "$HOME",
			expected: "/tmp/home",
		},
		{
			name:     "SingleQuotedLiteralUnchanged",
			input:    "'$$HOME'",
			expected: "'$$HOME'",
		},
		{
			name:     "SingleQuotedPreserved",
			input:    "'$HOME'",
			expected: "'$HOME'",
		},
		{
			name:     "SingleQuotedBracedPreserved",
			input:    "'${REAL}'",
			expected: "'${REAL}'",
		},
		{
			name:     "SingleQuotedPositionalPreserved",
			input:    "'$1'",
			expected: "'$1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := String(ctx, tt.input, opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestString_DollarEscape_Backtick(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping backtick tests on Windows")
	}
	ctx := context.Background()

	got, err := String(ctx, "`echo $$`")
	require.NoError(t, err)
	assert.Equal(t, "$", got)
}

func TestString_DollarEscape_Disabled(t *testing.T) {
	ctx := context.Background()

	got, err := String(ctx, "Price: $$9.99", WithoutDollarEscape())
	require.NoError(t, err)
	assert.Equal(t, "Price: $$9.99", got)
}
