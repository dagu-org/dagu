// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dag

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/sys/unix"
)

var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// parseSchedules parses the schedule values and returns a list of schedules.
// each schedule is parsed as a cron expression.
func parseSchedules(values []string) ([]Schedule, error) {
	var ret []Schedule

	for _, v := range values {
		parsed, err := cronParser.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidSchedule, err)
		}
		ret = append(ret, Schedule{Expression: v, Parsed: parsed})
	}

	return ret, nil
}

// parseScheduleMap parses the schedule map and populates the starts, stops,
// and restarts slices. Each key in the map must be either "start", "stop", or
// "restart". The value can be Case 1 or Case 2.
//
// Case 1: The value is a string
// Case 2: The value is an array of strings
//
// Example:
// ```yaml
// schedule:
//
//	start: "0 1 * * *"
//	stop: "0 18 * * *"
//	restart:
//	  - "0 1 * * *"
//	  - "0 18 * * *"
//
// ```
func parseScheduleMap(
	scheduleMap map[any]any, starts, stops, restarts *[]string,
) error {
	for k, v := range scheduleMap {
		// Key must be a string.
		key, ok := k.(string)
		if !ok {
			return errScheduleKeyMustBeString
		}
		var values []string

		switch v := v.(type) {
		case string:
			// Case 1. schedule is a string.
			values = append(values, v)

		case []any:
			// Case 2. schedule is an array of strings.
			// Append all the schedules to the values slice.
			for _, s := range v {
				s, ok := s.(string)
				if !ok {
					return errScheduleMustBeStringOrArray
				}
				values = append(values, s)
			}

		}

		var targets *[]string

		switch scheduleKey(key) {
		case scheduleKeyStart:
			targets = starts

		case scheduleKeyStop:
			targets = stops

		case scheduleKeyRestart:
			targets = restarts

		}

		for _, v := range values {
			if _, err := cronParser.Parse(v); err != nil {
				return fmt.Errorf("%w: %s", errInvalidSchedule, err)
			}
			*targets = append(*targets, v)
		}
	}

	return nil
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(value string, eval bool, options buildOpts) (
	params []string,
	envs []string,
	err error,
) {
	var parsedParams []paramPair

	parsedParams, err = parseParamValue(value, eval)
	if err != nil {
		return
	}

	var ret []string
	for i, p := range parsedParams {
		if eval {
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

		if !options.noEval && p.name != "" {
			envs = append(envs, strParam)
			err = os.Setenv(p.name, p.value)
			if err != nil {
				return
			}
		}
	}

	return ret, envs, nil
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

// parseParamValue parses the parameters for the DAG.
func parseParamValue(
	input string, executeCommandSubstitution bool,
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

			if executeCommandSubstitution {
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

// pair represents a key-value pair.
type pair struct {
	key string
	val string
}

// parseKeyValue parse a key-value pair from a map and appends it to the pairs
// slice. Each entry in the map must have a string key and a string value.
func parseKeyValue(m map[any]any, pairs *[]pair) error {
	for k, v := range m {
		key, ok := k.(string)
		if !ok {
			return errInvalidKeyType
		}

		var val string
		switch v := v.(type) {
		case string:
			val = v
		default:
			val = fmt.Sprintf("%v", v)
		}

		*pairs = append(*pairs, pair{key: key, val: val})
	}
	return nil
}

// parseFuncCall parses the function call in the step definition.
func parseFuncCall(step *Step, call *callFuncDef, funcs []*funcDef) error {
	if call == nil {
		return nil
	}

	passedArgs := make(map[string]string)
	step.Args = make([]string, 0, len(call.Args))

	for k, v := range call.Args {
		if strV, ok := v.(string); ok {
			step.Args = append(step.Args, strV)
			passedArgs[k] = strV
			continue
		}

		if intV, ok := v.(int); ok {
			strV := strconv.Itoa(intV)
			step.Args = append(step.Args, strV)
			passedArgs[k] = strV
			continue
		}

		return errArgsMustBeConvertibleToIntOrString
	}

	calledFuncDef := &funcDef{}

	for _, funcDef := range funcs {
		if funcDef.Name == call.Function {
			calledFuncDef = funcDef
			break
		}
	}

	step.Command = paramRegex.ReplaceAllString(calledFuncDef.Command, "")
	step.CmdWithArgs = assignValues(calledFuncDef.Command, passedArgs)

	return nil
}

// parseMiscs parses the miscellaneous fields in the step definition.
func parseMiscs(def *stepDef, step *Step) error {
	if def.ContinueOn != nil {
		step.ContinueOn.Skipped = def.ContinueOn.Skipped
		step.ContinueOn.Failure = def.ContinueOn.Failure
	}

	if def.RetryPolicy != nil {
		step.RetryPolicy = &RetryPolicy{
			Limit:    def.RetryPolicy.Limit,
			Interval: time.Second * time.Duration(def.RetryPolicy.IntervalSec),
		}
	}

	if def.RepeatPolicy != nil {
		step.RepeatPolicy.Repeat = def.RepeatPolicy.Repeat
		step.RepeatPolicy.Interval = time.Second *
			time.Duration(def.RepeatPolicy.IntervalSec)
	}

	if def.SignalOnStop != nil {
		sigDef := *def.SignalOnStop
		sig := unix.SignalNum(sigDef)
		if sig == 0 {
			return fmt.Errorf("%w: %s", errInvalidSignal, sigDef)
		}
		step.SignalOnStop = sigDef
	}

	return nil
}

// parseKey parses the key as a string.
func parseKey(value any) (string, error) {
	val, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %T", errInvalidKeyType, value)
	}

	return val, nil
}

// parseTags builds a list of tags from the value.
// It converts the tags to lowercase and trims the whitespace.
func parseTags(value any) []string {
	var ret []string

	switch v := value.(type) {
	case string:
		for _, v := range strings.Split(v, ",") {
			tag := strings.ToLower(strings.TrimSpace(v))
			if tag != "" {
				ret = append(ret, tag)
			}
		}
	case []any:
		for _, v := range v {
			switch v := v.(type) {
			case string:
				ret = append(ret, strings.ToLower(strings.TrimSpace(v)))
			default:
				ret = append(ret, strings.ToLower(
					strings.TrimSpace(fmt.Sprintf("%v", v))),
				)
			}
		}
	}

	return ret
}
