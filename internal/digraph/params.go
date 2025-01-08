package digraph

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
)

// buildParams builds the parameters for the DAG.
func buildParams(ctx BuildContext, spec *definition, dag *DAG) error {
	var (
		paramPairs []paramPair
		envs       []string
	)

	if err := parseParams(ctx, spec.Params, &paramPairs, &envs); err != nil {
		return err
	}

	// Create default parameters string in the form of "key=value key=value ..."
	var paramsToJoin []string
	for _, paramPair := range paramPairs {
		paramsToJoin = append(paramsToJoin, paramPair.Escaped())
	}
	dag.DefaultParams = strings.Join(paramsToJoin, " ")

	if ctx.opts.parameters != "" {
		// Parse the parameters from the command line and override the default parameters
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.parameters, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
		overrideEnvirons(&envs, overrideEnvs)
	}

	if len(ctx.opts.parametersList) > 0 {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.parametersList, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
		overrideEnvirons(&envs, overrideEnvs)
	}

	for _, paramPair := range paramPairs {
		dag.Params = append(dag.Params, paramPair.String())
	}

	dag.Env = append(dag.Env, envs...)

	return nil
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

func overrideEnvirons(envs *[]string, override []string) {
	envsIndex := make(map[string]int)
	for i, env := range *envs {
		envsIndex[env] = i
	}
	for _, env := range override {
		if i, ok := envsIndex[env]; !ok {
			*envs = append(*envs, env)
		} else {
			(*envs)[i] = env
		}
	}
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value any, params *[]paramPair, envs *[]string) error {
	var paramPairs []paramPair

	paramPairs, err := parseParamValue(ctx, value)
	if err != nil {
		return wrapError("params", value, fmt.Errorf("%w: %s", errInvalidParamValue, err))
	}

	for index, paramPair := range paramPairs {
		if !ctx.opts.noEval {
			paramPair.Value = os.ExpandEnv(paramPair.Value)
		}

		*params = append(*params, paramPair)

		paramString := paramPair.String()

		// Set the parameter as an environment variable for the command
		// $1, $2, $3, ...
		if err := os.Setenv(strconv.Itoa(index+1), paramString); err != nil {
			return wrapError("params", paramString, fmt.Errorf("failed to set environment variable: %w", err))
		}

		if !ctx.opts.noEval && paramPair.Name != "" {
			*envs = append(*envs, paramString)
			if err := os.Setenv(paramPair.Name, paramPair.Value); err != nil {
				return wrapError("params", paramString, fmt.Errorf("failed to set environment variable: %w", err))
			}
		}
	}

	return nil
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

	default:
		return nil, wrapError("params", v, fmt.Errorf("%w: %T", errInvalidParamValue, v))

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

		case map[any]any:
			for name, value := range m {
				var nameStr string
				var valueStr string

				switch v := value.(type) {
				case string:
					valueStr = v

				default:
					return nil, wrapError("params", value, fmt.Errorf("%w: %T", errInvalidParamValue, v))

				}

				switch n := name.(type) {
				case string:
					nameStr = n

				default:
					return nil, wrapError("params", name, fmt.Errorf("%w: %T", errInvalidParamValue, n))

				}

				if !ctx.opts.noEval {
					parsed, err := cmdutil.EvalString(ctx.ctx, valueStr)
					if err != nil {
						return nil, wrapError("params", valueStr, fmt.Errorf("%w: %s", errInvalidParamValue, err))
					}
					valueStr = parsed
				}

				paramPair := paramPair{nameStr, valueStr}
				params = append(params, paramPair)
			}

		default:
			return nil, wrapError("params", m, fmt.Errorf("%w: %T", errInvalidParamValue, m))
		}
	}

	return params, nil
}

// paramRegex is a regex to match the parameters in the command.
var paramRegex = regexp.MustCompile(
	`(?:([^\s=]+)=)?("(?:\\"|[^"])*"|` + "`(" + `?:\\"|[^"]*)` + "`" + `|[^"\s]+)`,
)

func parseStringParams(ctx BuildContext, input string) ([]paramPair, error) {
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

			if !ctx.opts.noEval {
				// Perform backtick command substitution
				backtickRegex := regexp.MustCompile("`[^`]*`")

				var cmdErr error
				value = backtickRegex.ReplaceAllStringFunc(
					value,
					func(match string) string {
						cmdStr := strings.Trim(match, "`")
						cmdStr = os.ExpandEnv(cmdStr)
						cmdOut, err := exec.Command("sh", "-c", cmdStr).Output()
						if err != nil {
							cmdErr = err
							// Leave the original command if it fails
							return fmt.Sprintf("`%s`", cmdStr)
						}
						return strings.TrimSpace(string(cmdOut))
					},
				)

				if cmdErr != nil {
					return nil, wrapError("params", value, fmt.Errorf("%w: %s", errInvalidParamValue, cmdErr))
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
