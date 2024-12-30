// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
		pairsIndex := make(map[string]int)
		for i, paramPair := range paramPairs {
			if paramPair.name != "" {
				pairsIndex[paramPair.name] = i
			}
		}
		for i, paramPair := range overridePairs {
			if paramPair.name == "" {
				// For positional parameters
				if i < len(paramPairs) {
					paramPairs[i] = paramPair
				} else {
					paramPairs = append(paramPairs, paramPair)
				}
				continue
			}

			if foundIndex, ok := pairsIndex[paramPair.name]; ok {
				paramPairs[foundIndex] = paramPair
			} else {
				paramPairs = append(paramPairs, paramPair)
			}
		}

		envsIndex := make(map[string]int)
		for i, env := range envs {
			envsIndex[env] = i
		}
		for _, env := range overrideEnvs {
			if i, ok := envsIndex[env]; !ok {
				envs = append(envs, env)
			} else {
				envs[i] = env
			}
		}
	}

	// Convert the parameters to a string in the form of "key=value"
	var paramStrs []string
	for _, paramPair := range paramPairs {
		paramStrs = append(paramStrs, paramPair.String())
	}

	// Set the parameters as environment variables for the command
	dag.Env = append(dag.Env, envs...)
	dag.Params = append(dag.Params, paramStrs...)

	return nil
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value any, params *[]paramPair, envs *[]string) error {
	var parsedPairs []paramPair

	parsedPairs, err := parseParamValue(ctx, value)
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidParamValue, err)
	}

	for index, paramPair := range parsedPairs {
		if !ctx.opts.noEval {
			paramPair.value = os.ExpandEnv(paramPair.value)
		}

		*params = append(*params, paramPair)

		paramStr := paramPair.String()

		// Set the parameter as an environment variable for the command
		// $1, $2, $3, ...
		if err := os.Setenv(strconv.Itoa(index+1), paramStr); err != nil {
			return fmt.Errorf("failed to set environment variable: %w", err)
		}

		if !ctx.opts.noEval && paramPair.name != "" {
			*envs = append(*envs, paramStr)
			if err := os.Setenv(paramPair.name, paramPair.value); err != nil {
				return fmt.Errorf("failed to set environment variable: %w", err)
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

	case []map[string]string:
		return parseMapParams(ctx, v)

	default:
		return nil, fmt.Errorf("%w: %T", errInvalidParamValue, v)

	}
}

func parseMapParams(ctx BuildContext, input []map[string]string) ([]paramPair, error) {
	var params []paramPair

	for _, m := range input {
		for name, value := range m {
			if !ctx.opts.noEval {
				parsed, err := cmdutil.SubstituteWithEnvExpand(value)
				if err != nil {
					return nil, fmt.Errorf("%w: %s", errInvalidParamValue, err)
				}
				value = parsed
			}

			paramPair := paramPair{name, value}
			params = append(params, paramPair)
		}
	}

	return params, nil
}

func parseStringParams(ctx BuildContext, input string) ([]paramPair, error) {
	paramRegex := regexp.MustCompile(
		`(?:([^\s=]+)=)?("(?:\\"|[^"])*"|` + "`(" + `?:\\"|[^"]*)` + "`" + `|[^"\s]+)`,
	)
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
					return nil, fmt.Errorf("%w: %s", errInvalidParamValue, cmdErr)
				}
			}
		}

		params = append(params, paramPair{name, value})
	}

	return params, nil
}

type paramPair struct {
	name  string
	value string
}

func (p paramPair) String() string {
	if p.name != "" {
		return fmt.Sprintf("%s=%s", p.name, p.value)
	}
	return p.value
}

func (p paramPair) Escaped() string {
	if p.name != "" {
		return fmt.Sprintf("%s=%q", p.name, p.value)
	}
	return fmt.Sprintf("%q", p.value)
}

var (
	// paramRegex is a regex to match the parameters in the command.
	paramRegex = regexp.MustCompile(`\$\w+`)
)
