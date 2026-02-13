package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

type paramToken struct {
	Name  string
	Value string
}

// paramTokenRegex matches positional and named params similarly to spec.parseStringParams.
var paramTokenRegex = regexp.MustCompile(
	`(?:([^\s=]+)=)?("(?:\\"|[^"])*"|` + "`[^`]*`" + `|[^"\s]+)`,
)

func validateStartArgumentSeparator(ctx *Context, args []string) error {
	if ctx.Command.ArgsLenAtDash() != -1 {
		return nil
	}
	if len(args) <= 1 {
		return nil
	}
	return fmt.Errorf(
		"unexpected arguments after DAG definition: %s (use '--' before parameters)",
		strings.Join(args[1:], " "),
	)
}

func validateStartPositionalParamCount(ctx *Context, args []string, dag *core.DAG) error {
	provided, skipValidation, err := extractProvidedParamTokens(ctx, args)
	if err != nil {
		return err
	}
	if skipValidation || len(provided) == 0 {
		return nil
	}

	got := countPositionalParams(provided)
	if got == 0 {
		// Named-only params should not trigger positional count validation.
		return nil
	}

	expected := countDeclaredPositionalParams(dag.DefaultParams)
	if got != expected {
		return fmt.Errorf("invalid number of positional params: expected %d, got %d", expected, got)
	}
	return nil
}

func extractProvidedParamTokens(ctx *Context, args []string) ([]paramToken, bool, error) {
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		if argsLenAtDash >= len(args) {
			return nil, false, nil
		}
		return parseParamsFromArgs(args[argsLenAtDash:]), false, nil
	}

	raw, err := ctx.Command.Flags().GetString("params")
	if err != nil {
		return nil, false, fmt.Errorf("failed to get parameters: %w", err)
	}
	raw = stringutil.RemoveQuotes(raw)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}

	if isJSONParams(raw) {
		// Keep current JSON behavior; positional count validation is not applied.
		return nil, true, nil
	}
	return parseParamTokens(raw), false, nil
}

func isJSONParams(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}
	isJSONObject := strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")
	isJSONArray := strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")
	if !isJSONObject && !isJSONArray {
		return false
	}
	return json.Valid([]byte(input))
}

func parseParamsFromArgs(args []string) []paramToken {
	var tokens []paramToken
	for _, arg := range args {
		tokens = append(tokens, parseParamTokens(arg)...)
	}
	return tokens
}

func parseParamTokens(input string) []paramToken {
	matches := paramTokenRegex.FindAllStringSubmatch(strings.TrimSpace(input), -1)
	tokens := make([]paramToken, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := match[1]
		value := match[2]
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = stringutil.RemoveQuotes(value)
		}
		tokens = append(tokens, paramToken{Name: name, Value: value})
	}
	return tokens
}

func countPositionalParams(tokens []paramToken) int {
	count := 0
	for _, token := range tokens {
		if token.Name == "" {
			count++
		}
	}
	return count
}

func countDeclaredPositionalParams(defaultParams string) int {
	if strings.TrimSpace(defaultParams) == "" {
		return 0
	}

	count := 0
	for _, token := range parseParamTokens(defaultParams) {
		if token.Name == "" || isPositionalDefaultName(token.Name) {
			count++
		}
	}
	return count
}

func isPositionalDefaultName(name string) bool {
	n, err := strconv.Atoi(name)
	return err == nil && n > 0
}
