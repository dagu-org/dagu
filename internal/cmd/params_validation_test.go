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
