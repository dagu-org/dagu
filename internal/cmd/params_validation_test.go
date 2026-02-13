package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseParamTokens_BacktickWithDoubleQuotes(t *testing.T) {
	t.Parallel()

	tokens := parseParamTokens("cmd=`echo \"hello world\"`")
	require.Len(t, tokens, 1)
	require.Equal(t, "cmd", tokens[0].Name)
	require.Equal(t, "`echo \"hello world\"`", tokens[0].Value)
}

func TestCountDeclaredPositionalParams(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, countDeclaredPositionalParams(""))
	require.Equal(t, 2, countDeclaredPositionalParams(`1="p1" 2="p2"`))
	require.Equal(t, 0, countDeclaredPositionalParams(`KEY1="v1" KEY2="v2"`))
}

func TestParseParamTokens_Matrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []paramToken
	}{
		{
			name:  "NamedBacktickWithInnerDoubleQuotes",
			input: `cmd=` + "`echo \"hello world\"`",
			expected: []paramToken{
				{Name: "cmd", Value: "`echo \"hello world\"`"},
			},
		},
		{
			name:  "PositionalBacktickToken",
			input: "`echo \"hello\"`",
			expected: []paramToken{
				{Name: "", Value: "`echo \"hello\"`"},
			},
		},
		{
			name:  "MixedNamedBacktickQuotedAndPositional",
			input: `A=1 cmd=` + "`echo \"x\"`" + ` B="y z" bare`,
			expected: []paramToken{
				{Name: "A", Value: "1"},
				{Name: "cmd", Value: "`echo \"x\"`"},
				{Name: "B", Value: "y z"},
				{Name: "", Value: "bare"},
			},
		},
		{
			name:  "EscapedQuotesInDoubleQuotedToken",
			input: `X="a \"b\"" Y=2`,
			expected: []paramToken{
				{Name: "X", Value: `a "b"`},
				{Name: "Y", Value: "2"},
			},
		},
		{
			name:     "EmptyInput",
			input:    "",
			expected: []paramToken{},
		},
		{
			name:     "WhitespaceInput",
			input:    "   ",
			expected: []paramToken{},
		},
		{
			name:  "UnterminatedDoubleQuoteFallback",
			input: `A="unterminated B=2`,
			expected: []paramToken{
				{Name: "", Value: "A="},
				{Name: "", Value: "unterminated"},
				{Name: "B", Value: "2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseParamTokens(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}
