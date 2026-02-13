package spec

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

func validateParams(paramPairs []paramPair, schema *jsonschema.Resolved) ([]paramPair, error) {
	// Convert paramPairs to a map for validation
	paramMap := make(map[string]any)
	for _, pair := range paramPairs {
		// Try to parse as JSON first, fall back to string
		var value any
		if err := json.Unmarshal([]byte(pair.Value), &value); err != nil {
			// If JSON parsing fails, use as string
			value = pair.Value
		}
		paramMap[pair.Name] = value
	}

	// Apply schema defaults to the parameter map
	if err := schema.ApplyDefaults(&paramMap); err != nil {
		return nil, fmt.Errorf("failed to apply schema defaults: %w", err)
	}

	if err := schema.Validate(paramMap); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Convert the updated paramMap back to paramPair format
	updatedPairs := make([]paramPair, 0, len(paramMap))
	for name, value := range paramMap {
		var valueStr string
		if str, ok := value.(string); ok {
			valueStr = str
		} else {
			// Convert non-string values to JSON string
			if jsonBytes, err := json.Marshal(value); err == nil {
				valueStr = string(jsonBytes)
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}
		}
		updatedPairs = append(updatedPairs, paramPair{Name: name, Value: valueStr})
	}

	return updatedPairs, nil
}

func overrideParams(paramPairs *[]paramPair, override []paramPair) {
	// Override the default parameters with the command line parameters
	pairsIndex := make(map[string]int)
	for i, paramPair := range *paramPairs {
		if paramPair.Name != "" {
			pairsIndex[paramPair.Name] = i
		}
	}
	for i, paramPair := range override {
		if paramPair.Name == "" {
			// For positional parameters
			if i < len(*paramPairs) {
				(*paramPairs)[i] = paramPair
			} else {
				*paramPairs = append(*paramPairs, paramPair)
			}
			continue
		}

		if foundIndex, ok := pairsIndex[paramPair.Name]; ok {
			(*paramPairs)[foundIndex] = paramPair
		} else {
			*paramPairs = append(*paramPairs, paramPair)
		}
	}
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value any, params *[]paramPair, envs *[]string) error {
	var paramPairs []paramPair

	paramPairs, err := parseParamValue(ctx, value)
	if err != nil {
		return core.NewValidationError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
	}

	// Accumulated vars for sequential param expansion (e.g., Y=${P1})
	accumulatedVars := make(map[string]string)

	for index, paramPair := range paramPairs {
		if !ctx.opts.Has(BuildFlagNoEval) {
			evaluated, err := evalParamValue(ctx, paramPair.Value, accumulatedVars)
			if err != nil {
				return core.NewValidationError("params", paramPair.Value, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
			}
			paramPair.Value = evaluated
		}

		*params = append(*params, paramPair)

		paramString := paramPair.String()

		// Store in accumulated vars for next param expansion
		// Positional params: $1, $2, $3, ...
		accumulatedVars[strconv.Itoa(index+1)] = paramString

		if paramPair.Name != "" {
			accumulatedVars[paramPair.Name] = paramPair.Value
		}

		if !ctx.opts.Has(BuildFlagNoEval) && paramPair.Name != "" {
			*envs = append(*envs, paramString)
		}

		if paramPair.Name == "" {
			(*params)[index].Name = strconv.Itoa(index + 1)
		}
	}

	return nil
}

func evalParamValue(ctx BuildContext, raw string, accumulatedVars map[string]string) (string, error) {
	var evalOptions []eval.Option

	if len(accumulatedVars) > 0 {
		evalOptions = append(evalOptions, eval.WithVariables(accumulatedVars))
	}

	// Use envScope.buildEnv if available (new thread-safe approach),
	// fall back to ctx.buildEnv for backward compatibility
	if ctx.envScope != nil && len(ctx.envScope.buildEnv) > 0 {
		evalOptions = append(evalOptions, eval.WithVariables(ctx.envScope.buildEnv))
	} else if ctx.buildEnv != nil {
		evalOptions = append(evalOptions, eval.WithVariables(ctx.buildEnv))
	}

	// Also set EnvScope on context for command substitution
	evalCtx := ctx.ctx
	if ctx.envScope != nil && ctx.envScope.scope != nil {
		evalCtx = eval.WithEnvScope(evalCtx, ctx.envScope.scope)
	}

	evalOptions = append(evalOptions, eval.WithOSExpansion())
	return eval.String(evalCtx, raw, evalOptions...)
}

// parseParamValue parses the parameters for the DAG.
func parseParamValue(ctx BuildContext, input any) ([]paramPair, error) {
	switch v := input.(type) {
	case nil:
		return nil, nil

	case string:
		return parseStringParams(ctx, v)

	case []any:
		return parseMapParams(ctx, v)

	case []string:
		return parseListParams(ctx, v)

	// At this point, the schema input can be two cases:
	// 1. a map with a "schema" key and a "values" key
	// e.g. { "schema": "./schema.json", "values": { "batch_size": 10, "environment": "dev" } }
	// 2. a map with no "schema" key
	// e.g. { "batch_size": 10, "environment": "dev" }
	case map[string]any:
		schemaRef := extractSchemaReference(v)
		if schemaRef == "" {
			return parseMapParams(ctx, []any{v})
		}

		values, ok := v["values"]
		if !ok {
			return []paramPair{}, nil // Schema-only mode, no values to validate
		}

		return parseMapParams(ctx, []any{values})
	default:
		return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %T", ErrInvalidParamValue, v))

	}
}

func parseListParams(ctx BuildContext, input []string) ([]paramPair, error) {
	var params []paramPair

	for _, v := range input {
		parsedParams, err := parseStringParams(ctx, v)
		if err != nil {
			return nil, err
		}
		params = append(params, parsedParams...)
	}

	return params, nil
}

func parseMapParams(ctx BuildContext, input []any) ([]paramPair, error) {
	var params []paramPair

	for _, m := range input {
		switch m := m.(type) {
		case string:
			parsedParams, err := parseStringParams(ctx, m)
			if err != nil {
				return nil, err
			}
			params = append(params, parsedParams...)

		case map[string]any:
			// Iterate deterministically to avoid random param order from Go maps.
			keys := make([]string, 0, len(m))
			for name := range m {
				keys = append(keys, name)
			}
			sort.Strings(keys)

			for _, name := range keys {
				value := m[name]
				var valueStr string

				switch v := value.(type) {
				case string:
					valueStr = v

				default:
					valueStr = fmt.Sprintf("%v", v)

				}

				paramPair := paramPair{name, valueStr}
				params = append(params, paramPair)
			}

		default:
			return nil, core.NewValidationError("params", m, fmt.Errorf("%w: %T", ErrInvalidParamValue, m))
		}
	}

	return params, nil
}

// paramRegex is a regex to match the parameters in the command.
var paramRegex = regexp.MustCompile(
	`(?:([^\s=]+)=)?("(?:\\"|[^"])*"|` + "`(" + `?:\\"|[^"]*)` + "`" + `|[^"\s]+)`,
)

// backtickRegex matches backtick-enclosed commands for substitution.
var backtickRegex = regexp.MustCompile("`[^`]*`")

// tryParseJSONParams attempts to parse the input as JSON and convert it to paramPairs.
// Returns an error if the input is not valid JSON.
func tryParseJSONParams(ctx BuildContext, input string) ([]paramPair, error) {
	// Try parsing as JSON object first
	var jsonObj map[string]any
	if err := json.Unmarshal([]byte(input), &jsonObj); err == nil {
		return parseMapParams(ctx, []any{jsonObj})
	}

	// Try parsing as JSON array
	var jsonArr []any
	if err := json.Unmarshal([]byte(input), &jsonArr); err == nil {
		var params []paramPair
		for _, item := range jsonArr {
			switch v := item.(type) {
			case string:
				params = append(params, paramPair{Name: "", Value: v})
			case map[string]any:
				mapParams, err := parseMapParams(ctx, []any{v})
				if err != nil {
					return nil, err
				}
				params = append(params, mapParams...)
			default:
				// Convert other types (numbers, booleans) to string
				params = append(params, paramPair{Name: "", Value: fmt.Sprintf("%v", v)})
			}
		}
		return params, nil
	}

	return nil, fmt.Errorf("not valid JSON")
}

func parseStringParams(ctx BuildContext, input string) ([]paramPair, error) {
	input = strings.TrimSpace(input)

	// Check if input looks like a JSON object or array
	if (strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")) ||
		(strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")) {
		params, err := tryParseJSONParams(ctx, input)
		if err == nil {
			return params, nil
		}
		// If JSON parsing fails, fall through to regex parsing
	}

	matches := paramRegex.FindAllStringSubmatch(input, -1)

	var params []paramPair

	for _, match := range matches {
		name := match[1]
		value := match[2]

		if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "`") {
			if strings.HasPrefix(value, `"`) {
				value = strings.Trim(value, `"`)
				value = strings.ReplaceAll(value, `\"`, `"`)
			}

			if !ctx.opts.Has(BuildFlagNoEval) {
				// Perform backtick command substitution using package-level regex
				var cmdErr error
				value = backtickRegex.ReplaceAllStringFunc(
					value,
					func(match string) string {
						var err error
						cmdStr := strings.Trim(match, "`")
						cmdStr, err = eval.String(ctx.ctx, cmdStr, eval.WithOSExpansion())
						if err != nil {
							cmdErr = err
							// Leave the original command if it fails
							return fmt.Sprintf("`%s`", cmdStr)
						}
						cmdOut, err := exec.Command("sh", "-c", cmdStr).Output() //nolint:gosec
						if err != nil {
							cmdErr = err
							// Leave the original command if it fails
							return fmt.Sprintf("`%s`", cmdStr)
						}
						return strings.TrimSpace(string(cmdOut))
					},
				)

				if cmdErr != nil {
					return nil, core.NewValidationError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, cmdErr))
				}
			}
		}

		params = append(params, paramPair{name, value})
	}

	return params, nil
}

type paramPair struct {
	Name  string
	Value string
}

func (p paramPair) String() string {
	if p.Name != "" {
		return fmt.Sprintf("%s=%s", p.Name, p.Value)
	}
	return p.Value
}

func (p paramPair) Escaped() string {
	if p.Name != "" {
		return fmt.Sprintf("%s=%q", p.Name, p.Value)
	}
	return fmt.Sprintf("%q", p.Value)
}

// SmartEscape returns a string representation that only quotes values when
// necessary (empty, or containing spaces/tabs/newlines/double-quotes).
// This allows variable references like ${ITEM.xxx} to remain unquoted so
// that their expanded content can be re-split into separate KEY=VALUE pairs.
func (p paramPair) SmartEscape() string {
	needsQuoting := p.Value == "" || strings.ContainsAny(p.Value, " \t\n\"")
	if p.Name != "" {
		if needsQuoting {
			return fmt.Sprintf("%s=%q", p.Name, p.Value)
		}
		return fmt.Sprintf("%s=%s", p.Name, p.Value)
	}
	if needsQuoting {
		return fmt.Sprintf("%q", p.Value)
	}
	return p.Value
}
