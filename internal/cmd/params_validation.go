package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
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

	expected := countDeclaredPositionalParams(dag.DefaultParams)
	if expected == 0 {
		// No declared params means there is no positional-count contract to enforce.
		return nil
	}

	got := countPositionalParams(provided)
	if got == 0 {
		// Named-only params should not trigger positional count validation.
		return nil
	}

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
		dashArgs := args[argsLenAtDash:]
		if shouldSkipDashArgsPositionalValidation(dashArgs) {
			return nil, true, nil
		}
		return parseParamsFromArgs(dashArgs), false, nil
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

func shouldSkipDashArgsPositionalValidation(args []string) bool {
	// JSON payloads are passed as a single arg after "--" and should bypass
	// positional-count validation (same behavior as --params JSON input).
	if len(args) != 1 {
		return false
	}
	input := stringutil.RemoveQuotes(args[0])
	return isJSONParams(input)
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

	return len(parseParamTokens(defaultParams))
}
