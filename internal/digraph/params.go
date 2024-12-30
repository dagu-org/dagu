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
)

// buildParams builds the parameters for the DAG.
func buildParams(ctx BuildContext, spec *definition, dag *DAG) (err error) {
	dag.DefaultParams = spec.Params

	params := dag.DefaultParams
	if ctx.opts.parameters != "" {
		params = ctx.opts.parameters
	}

	var envs []string
	dag.Params, envs, err = parseParams(ctx, params)
	if err == nil {
		dag.Env = append(dag.Env, envs...)
	}

	return
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value string) (
	params []string,
	envs []string,
	err error,
) {
	var parsedParams []paramPair

	parsedParams, err = parseParamValue(ctx, value)
	if err != nil {
		return
	}

	var ret []string
	for i, p := range parsedParams {
		if !ctx.opts.noEval {
			p.value = os.ExpandEnv(p.value)
		}

		strParam := stringifyParam(p)
		ret = append(ret, strParam)

		if p.name == "" {
			strParam = p.value
		}

		if err = os.Setenv(strconv.Itoa(i+1), strParam); err != nil {
			return
		}

		if !ctx.opts.noEval && p.name != "" {
			envs = append(envs, strParam)
			err = os.Setenv(p.name, p.value)
			if err != nil {
				return
			}
		}
	}

	return ret, envs, nil
}

// parseParamValue parses the parameters for the DAG.
func parseParamValue(
	ctx BuildContext, input string,
) ([]paramPair, error) {
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

// stringifyParam converts a paramPair to a string representation.
func stringifyParam(param paramPair) string {
	if param.name != "" {
		return fmt.Sprintf("%s=%s", param.name, param.value)
	}
	return param.value
}

// paramPair represents a key-value pair for the parameters.
type paramPair struct {
	name  string
	value string
}

var (
	// paramRegex is a regex to match the parameters in the command.
	paramRegex = regexp.MustCompile(`\$\w+`)
)
