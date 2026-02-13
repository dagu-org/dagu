package cmd

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/spf13/cobra"
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
	require.Equal(t, 2, countDeclaredPositionalParams(`KEY1="v1" KEY2="v2"`))
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

func TestShouldSkipDashArgsPositionalValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "SingleJSONObject",
			args: []string{`{"KEY":"value"}`},
			want: true,
		},
		{
			name: "SingleJSONArray",
			args: []string{`["a","b"]`},
			want: true,
		},
		{
			name: "SingleQuotedJSONObject",
			args: []string{`'{"KEY":"value"}'`},
			want: false,
		},
		{
			name: "SingleNonJSON",
			args: []string{"key=value"},
			want: false,
		},
		{
			name: "MultipleArgs",
			args: []string{"key=value", "key2=value2"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldSkipDashArgsPositionalValidation(tt.args)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateStartPositionalParamCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cliArgs     []string
		defaultArgs string
		wantErr     string
	}{
		{
			name:        "NoDeclaredParamsAllowsNamedPairsAfterDash",
			cliArgs:     []string{"dag.yaml", "--", "key1=value1", "key2=value2"},
			defaultArgs: "",
		},
		{
			name:        "NoDeclaredParamsAllowsPositionalAfterDash",
			cliArgs:     []string{"dag.yaml", "--", "success"},
			defaultArgs: "",
		},
		{
			name:        "SingleNamedDeclaredParamAllowsSinglePositionalAfterDash",
			cliArgs:     []string{"dag.yaml", "--", "server1"},
			defaultArgs: `ITEM=default`,
		},
		{
			name:        "JSONAfterDashSkipsPositionalValidation",
			cliArgs:     []string{"dag.yaml", "--", `{"REGION":"us-east","VERSION":"2.0"}`},
			defaultArgs: `REGION=us-east-1 VERSION=1.0.0`,
		},
		{
			name:        "MismatchStillFailsWhenDeclaredCountExists",
			cliArgs:     []string{"dag.yaml", "--", "only-one"},
			defaultArgs: `p1 p2`,
			wantErr:     "invalid number of positional params: expected 2, got 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, args := testValidationContext(t, tt.cliArgs)
			dag := &core.DAG{DefaultParams: tt.defaultArgs}
			err := validateStartPositionalParamCount(ctx, args, dag)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func testValidationContext(t *testing.T, cliArgs []string) (*Context, []string) {
	t.Helper()

	command := &cobra.Command{Use: "start"}
	command.Flags().String("params", "", "")

	err := command.Flags().Parse(cliArgs)
	require.NoError(t, err)

	return &Context{Command: command}, command.Flags().Args()
}
