// Copyright (C) 2024 The Dagu Authors
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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/util"
)

// Exts is a list of supported file extensions for DAG files.
var Exts = []string{".yaml", ".yml"}

// builder is used to build a DAG from a configuration definition.
type builder struct {
	def         *definition // intermediate value to build the DAG.
	envs        []string    // environment variables for the DAG.
	opts        buildOpts   // options for building the DAG.
	dag         *DAG        // the final DAG.
	errs        errorList
	stepBuilder stepBuilder
}

// builderFunc is a function that builds a part of the DAG.
type builderFunc func() error

// buildOpts is used to control the behavior of the builder.
type buildOpts struct {
	// base specifies the base configuration file for the DAG.
	base string
	// metadataOnly specifies whether to build only the metadata.
	metadataOnly bool
	// parameters specifies the parameters to the DAG.
	// parameters are used to override the default parameters for
	// executing the DAG.
	parameters string
	// noEval specifies whether to evaluate environment variables.
	// This is useful when loading details for a DAG, but not
	// for execution.
	noEval bool
}

// errors on building a DAG.
var (
	errInvalidSchedule             = errors.New("invalid schedule")
	errScheduleMustBeStringOrArray = errors.New(
		"schedule must be a string or an array of strings",
	)
	errInvalidScheduleType        = errors.New("invalid schedule type")
	errInvalidKeyType             = errors.New("invalid key type")
	errExecutorConfigMustBeString = errors.New(
		"executor config key must be string",
	)
	errDuplicateFunction  = errors.New("duplicate function")
	errFuncParamsMismatch = errors.New(
		"func params and args given to func command do not match",
	)
	errStepNameRequired          = errors.New("step name must be specified")
	errStepCommandOrCallRequired = errors.New(
		"either step command or step call must be specified if executor is nil",
	)
	errStepCommandIsEmpty             = errors.New("step command is empty")
	errStepCommandMustBeArrayOrString = errors.New(
		"step command must be an array of strings or a string",
	)
	errInvalidParamValue    = errors.New("invalid parameter value")
	errCallFunctionNotFound = errors.New(
		"call must specify a functions that exists",
	)
	errNumberOfParamsMismatch = errors.New(
		"the number of parameters defined in the function does not match the number of parameters given",
	)
	errRequiredParameterNotFound = errors.New(
		"required parameter not found",
	)
	errScheduleKeyMustBeString = errors.New(
		"schedule key must be a string",
	)
	errInvalidSignal                      = errors.New("invalid signal")
	errInvalidEnvValue                    = errors.New("invalid value for env")
	errArgsMustBeConvertibleToIntOrString = errors.New(
		"args must be convertible to either int or string",
	)
	errExecutorTypeMustBeString = errors.New(
		"executor.type value must be string",
	)
	errExecutorConfigValueMustBeMap = errors.New(
		"executor.config value must be a map",
	)
	errExecutorHasInvalidKey = errors.New(
		"executor has invalid key",
	)
	errExecutorConfigMustBeStringOrMap = errors.New(
		"executor config must be string or map",
	)
)

// build builds a DAG from a configuration definition and the base DAG.
// This method requires two arguments:
//   - def: the configuration definition for the DAG.
//   - envs: the environment variables of the base configuration.
//     These are used to set the environment variables for the DAG.
func (b *builder) build(def *definition, envs []string) (*DAG, error) {
	b.def = def
	b.envs = envs
	b.dag = &DAG{
		Name:        def.Name,
		Group:       def.Group,
		Description: def.Description,
		Timeout:     time.Second * time.Duration(def.TimeoutSec),
		Delay:       time.Second * time.Duration(def.DelaySec),
		RestartWait: time.Second * time.Duration(def.RestartWaitSec),
		Tags:        parseTags(def.Tags),
	}
	b.stepBuilder = stepBuilder{noEval: b.opts.noEval}

	b.callBuilderFunc(b.buildEnvs)
	b.callBuilderFunc(b.buildSchedule)
	b.callBuilderFunc(b.buildMailOn)
	b.callBuilderFunc(b.buildParams)

	// If metadataOnly is set, return the DAG with the metadata.
	// This is done for avoiding unnecessary processing when
	// only the metadata is required.
	if !b.opts.metadataOnly {
		b.callBuilderFunc(b.buildSteps)
		b.callBuilderFunc(b.buildLogDir)
		b.callBuilderFunc(b.buildHandlers)
		b.callBuilderFunc(b.buildSMTPConfig)
		b.callBuilderFunc(b.buildErrMailConfig)
		b.callBuilderFunc(b.buildInfoMailConfig)
		b.callBuilderFunc(b.buildMiscs)

		if err := assertFunctions(def.Functions); err != nil {
			b.errs.Add(err)
		}
	}

	if len(b.errs) > 0 {
		return nil, &b.errs
	}

	return b.dag, nil
}

// buildSchedule parses the schedule in different formats and builds the
// schedule. It allows for flexibility in defining the schedule.
//
// Case 1: schedule is a string
//
// ```yaml
// schedule: "0 1 * * *"
// ```
//
// Case 2: schedule is an array of strings
//
// ```yaml
// schedule:
//   - "0 1 * * *"
//   - "0 18 * * *"
//
// ```
//
// Case 3: schedule is a map
// The map can have the following keys
// - start: string or array of strings
// - stop: string or array of strings
// - restart: string or array of strings
func (b *builder) buildSchedule() error {
	var starts, stops, restarts []string

	switch schedule := (b.def.Schedule).(type) {
	case string:
		// Case 1. schedule is a string.
		starts = append(starts, schedule)

	case []any:
		// Case 2. schedule is an array of strings.
		// Append all the schedules to the starts slice.
		for _, s := range schedule {
			s, ok := s.(string)
			if !ok {
				return fmt.Errorf(
					"%w, got %T: ", errScheduleMustBeStringOrArray, s,
				)
			}
			starts = append(starts, s)
		}

	case map[any]any:
		// Case 3. schedule is a map.
		if err := parseScheduleMap(
			schedule, &starts, &stops, &restarts,
		); err != nil {
			return err
		}

	case nil:
		// If schedule is nil, return without error.

	default:
		// If schedule is of an invalid type, return an error.
		return fmt.Errorf("%w: %T", errInvalidScheduleType, b.def.Schedule)

	}

	// Parse each schedule as a cron expression.
	var err error
	b.dag.Schedule, err = parseSchedules(starts)
	if err != nil {
		return err
	}
	b.dag.StopSchedule, err = parseSchedules(stops)
	if err != nil {
		return err
	}
	b.dag.RestartSchedule, err = parseSchedules(restarts)
	return err
}

func (b *builder) buildMailOn() error {
	if b.def.MailOn == nil {
		return nil
	}
	b.dag.MailOn = &MailOn{
		Failure: b.def.MailOn.Failure,
		Success: b.def.MailOn.Success,
	}
	return nil
}

// buildEnvs builds the environment variables for the DAG.
// Case 1: env is an array of maps with string keys and string values.
// Case 2: env is a map with string keys and string values.
func (b *builder) buildEnvs() error {
	env, err := loadVariables(b.def.Env, b.opts)
	if err != nil {
		return err
	}
	b.dag.Env = buildConfigEnv(env)

	// Add the environment variables that are defined in the base
	// configuration. If the environment variable is already defined in
	// the DAG, it is not added.
	for _, e := range b.envs {
		key := strings.SplitN(e, "=", 2)[0]
		if _, ok := env[key]; !ok {
			b.dag.Env = append(b.dag.Env, e)
		}
	}
	return nil
}

// buildLogDir builds the log directory for the DAG.
func (b *builder) buildLogDir() (err error) {
	logDir, err := substituteCommands(os.ExpandEnv(b.def.LogDir))
	if err != nil {
		return err
	}
	b.dag.LogDir = logDir
	return err
}

// buildParams builds the parameters for the DAG.
func (b *builder) buildParams() (err error) {
	params := b.def.Params
	if params == nil {
		params = ""
	}
	b.dag.DefaultParams = fmt.Sprint(params)

	if b.opts.parameters != "" {
		params = b.opts.parameters
	}

	var envs []string
	b.dag.Params, envs, err = parseParams(fmt.Sprint(params), !b.opts.noEval, b.opts)
	if err == nil {
		b.dag.Env = append(b.dag.Env, envs...)
	}

	return
}

// buildHandlers builds the handlers for the DAG.
// The handlers are executed when the DAG is stopped, succeeded, failed, or
// cancelled.
func (b *builder) buildHandlers() (err error) {
	if b.def.HandlerOn.Exit != nil {
		b.def.HandlerOn.Exit.Name = HandlerOnExit.String()
		if b.dag.HandlerOn.Exit, err = b.stepBuilder.buildStep(
			b.dag.Env, b.def.HandlerOn.Exit, b.def.Functions,
		); err != nil {
			return err
		}
	}

	if b.def.HandlerOn.Success != nil {
		b.def.HandlerOn.Success.Name = HandlerOnSuccess.String()
		if b.dag.HandlerOn.Success, err = b.stepBuilder.buildStep(
			b.dag.Env, b.def.HandlerOn.Success, b.def.Functions,
		); err != nil {
			return
		}
	}

	if b.def.HandlerOn.Failure != nil {
		b.def.HandlerOn.Failure.Name = HandlerOnFailure.String()
		if b.dag.HandlerOn.Failure, err = b.stepBuilder.buildStep(
			b.dag.Env, b.def.HandlerOn.Failure, b.def.Functions,
		); err != nil {
			return
		}
	}

	if b.def.HandlerOn.Cancel != nil {
		b.def.HandlerOn.Cancel.Name = HandlerOnCancel.String()
		if b.dag.HandlerOn.Cancel, err = b.stepBuilder.buildStep(
			b.dag.Env, b.def.HandlerOn.Cancel, b.def.Functions,
		); err != nil {
			return
		}
	}

	return nil
}

// buildMiscs builds the miscellaneous fields for the DAG.
func (b *builder) buildMiscs() (err error) {
	if b.def.HistRetentionDays != nil {
		b.dag.HistRetentionDays = *b.def.HistRetentionDays
	}

	b.dag.Preconditions = buildConditions(b.def.Preconditions)
	b.dag.MaxActiveRuns = b.def.MaxActiveRuns

	if b.def.MaxCleanUpTimeSec != nil {
		b.dag.MaxCleanUpTime = time.Second *
			time.Duration(*b.def.MaxCleanUpTimeSec)
	}

	return nil
}

// loadVariables loads the environment variables from the map.
// Case 1: env is a map.
// Case 2: env is an array of maps.
// Case 3: is recommended because the order of the environment variables is
// preserved.
func loadVariables(strVariables any, opts buildOpts) (
	map[string]string, error,
) {
	var pairs []pair

	switch a := strVariables.(type) {
	case map[any]any:
		// Case 1. env is a map.
		if err := parseKeyValue(a, &pairs); err != nil {
			return nil, err
		}

	case []any:
		// Case 2. env is an array of maps.
		for _, v := range a {
			if aa, ok := v.(map[any]any); ok {
				if err := parseKeyValue(aa, &pairs); err != nil {
					return nil, err
				}
			}
		}

	}

	// Parse each key-value pair and set the environment variable.
	vars := map[string]string{}
	for _, pair := range pairs {
		value := pair.val

		if !opts.noEval {
			// Evaluate the value of the environment variable.
			// This also executes command substitution.
			var err error

			value, err = substituteCommands(os.ExpandEnv(value))
			if err != nil {
				return nil, fmt.Errorf("%w: %s", errInvalidEnvValue, pair.val)
			}

			if err := os.Setenv(pair.key, value); err != nil {
				return nil, err
			}
		}

		vars[pair.key] = value
	}
	return vars, nil
}

// buildSteps builds the steps for the DAG.
func (b *builder) buildSteps() error {
	var ret []Step

	for _, stepDef := range b.def.Steps {
		step, err := b.stepBuilder.buildStep(
			b.dag.Env, stepDef, b.def.Functions,
		)
		if err != nil {
			return err
		}
		ret = append(ret, *step)
	}

	b.dag.Steps = ret

	return nil
}

// buildSMTPConfig builds the SMTP configuration for the DAG.
func (b *builder) buildSMTPConfig() (err error) {
	b.dag.SMTP = &SMTPConfig{
		Host:     os.ExpandEnv(b.def.SMTP.Host),
		Port:     os.ExpandEnv(b.def.SMTP.Port),
		Username: os.ExpandEnv(b.def.SMTP.Username),
		Password: os.ExpandEnv(b.def.SMTP.Password),
	}

	return nil
}

// buildErrMailConfig builds the error mail configuration for the DAG.
func (b *builder) buildErrMailConfig() (err error) {
	b.dag.ErrorMail, err = buildMailConfig(b.def.ErrorMail)

	return
}

// buildInfoMailConfig builds the info mail configuration for the DAG.
func (b *builder) buildInfoMailConfig() (err error) {
	b.dag.InfoMail, err = buildMailConfig(b.def.InfoMail)

	return
}

// callBuilderFunc calls a builder function and adds any errors to the error
// list.
func (b *builder) callBuilderFunc(fn builderFunc) {
	if err := fn(); err != nil {
		b.errs.Add(err)
	}
}

// stepBuilder is used to build a step from the step definition.
type stepBuilder struct {
	noEval bool
}

// buildStep builds a step from the step definition.
func (b *stepBuilder) buildStep(
	variables []string, def *stepDef, fns []*funcDef,
) (*Step, error) {
	if err := assertStepDef(def, fns); err != nil {
		return nil, err
	}

	step := &Step{
		Name:           def.Name,
		Description:    def.Description,
		Script:         def.Script,
		Stdout:         def.Stdout,
		Stderr:         def.Stderr,
		Output:         def.Output,
		Dir:            def.Dir,
		Variables:      variables,
		Depends:        def.Depends,
		MailOnError:    def.MailOnError,
		Preconditions:  buildConditions(def.Preconditions),
		ExecutorConfig: ExecutorConfig{Config: make(map[string]any)},
	}

	if err := parseFuncCall(step, def.Call, fns); err != nil {
		return nil, err
	}

	for _, fn := range stepBuilderFuncs {
		if err := fn(def, step); err != nil {
			return nil, err
		}
	}

	return step, nil
}

// buildMailConfig builds a MailConfig from the definition.
func buildMailConfig(def mailConfigDef) (*MailConfig, error) {
	return &MailConfig{
		From:       def.From,
		To:         def.To,
		Prefix:     def.Prefix,
		AttachLogs: def.AttachLogs,
	}, nil
}

// stepBuilderFunc is a function that builds a step from the step definition.
type stepBuilderFunc func(def *stepDef, step *Step) error

var (
	// stepBuilderFuncs is a list of functions that build a step from the step
	// definition.
	stepBuilderFuncs = []stepBuilderFunc{
		parseCommand,
		parseExecutor,
		parseSubWorkflow,
		parseMiscs,
	}
)

// commandRun is not a actual command.
// subworkflow does not use this command field so it is used
// just for display purposes.
const commandRun = "run"

// parseSubWorkflow parses the subworkflow definition and sets the step fields.
func parseSubWorkflow(def *stepDef, step *Step) error {
	name, params := def.Run, def.Params

	// if the run field is not set, return nil.
	if name == "" {
		return nil
	}

	// Set the step fields for the subworkflow.
	step.SubWorkflow = &SubWorkflow{Name: name, Params: params}
	step.ExecutorConfig.Type = ExecutorTypeSubWorkflow
	step.Command = commandRun
	step.Args = []string{name, params}
	step.CmdWithArgs = fmt.Sprintf("%s %s", name, params)
	return nil
}

const (
	executorKeyType   = "type"
	executorKeyConfig = "config"
)

// parseExecutor parses the executor field in the step definition.
// Case 1: executor is nil
// Case 2: executor is a string
// Case 3: executor is a struct
func parseExecutor(def *stepDef, step *Step) error {
	executor := def.Executor

	// Case 1: executor is nil
	if executor == nil {
		return nil
	}

	switch val := executor.(type) {
	case string:
		// Case 2: executor is a string
		// This can be an executor with default configuration.
		step.ExecutorConfig.Type = val

	case map[any]any:
		// Case 3: executor is a struct
		// In this case, the executor is a struct with type and config fields.
		// Config is a map of string keys and values.
		for k, v := range val {
			key, ok := k.(string)
			if !ok {
				return errExecutorConfigMustBeString
			}

			switch key {
			case executorKeyType:
				// Executor type is a string.
				typ, ok := v.(string)
				if !ok {
					return errExecutorTypeMustBeString
				}
				step.ExecutorConfig.Type = typ

			case executorKeyConfig:
				// Executor config is a map of string keys and values.
				// The values can be of any type.
				// It is up to the executor to parse the values.
				executorConfig, ok := v.(map[any]any)
				if !ok {
					return errExecutorConfigValueMustBeMap
				}
				for k, v := range executorConfig {
					configKey, ok := k.(string)
					if !ok {
						return errExecutorConfigMustBeString
					}
					step.ExecutorConfig.Config[configKey] = v
				}

			default:
				// Unknown key in the executor config.
				return fmt.Errorf("%w: %s", errExecutorHasInvalidKey, key)

			}
		}

	default:
		// Unknown key for executor field.
		return errExecutorConfigMustBeStringOrMap

	}

	// Convert map[any]any to map[string]any for executor config.
	// It is up to the executor to parse the values.
	return convertMap(step.ExecutorConfig.Config)
}

// parseCommand parses the command field in the step definition.
// Case 1: command is nil
// Case 2: command is a string
// Case 3: command is an array
//
// In case 3, the first element is the command and the rest are the arguments.
// If the arguments are not strings, they are converted to strings.
//
// Example:
// ```yaml
// step:
//   - name: "echo hello"
//     command: "echo hello"
//
// ```
// or
// ```yaml
// step:
//   - name: "echo hello"
//     command: ["echo", "hello"]
//
// ```
// It returns an error if the command is not nil but empty.
func parseCommand(def *stepDef, step *Step) error {
	command := def.Command

	// Case 1: command is nil
	if command == nil {
		return nil
	}

	switch val := command.(type) {
	case string:
		// Case 2: command is a string
		if val == "" {
			return errStepCommandIsEmpty
		}
		// We need to split the command into command and args.
		step.CmdWithArgs = val
		step.Command, step.Args = util.SplitCommand(val)

	case []any:
		// Case 3: command is an array
		for _, v := range val {
			val, ok := v.(string)
			if !ok {
				// If the value is not a string, convert it to a string.
				// This is useful when the value is an integer for example.
				val = fmt.Sprintf("%v", v)
			}
			if step.Command == "" {
				step.Command = val
				continue
			}
			step.Args = append(step.Args, val)
		}

	default:
		// Unknown type for command field.
		return errStepCommandMustBeArrayOrString

	}

	return nil
}

// assignValues Assign values to command parameters
func assignValues(command string, params map[string]string) string {
	updatedCommand := command

	for k, v := range params {
		updatedCommand = strings.ReplaceAll(
			updatedCommand, fmt.Sprintf("$%v", k), v,
		)
	}

	return updatedCommand
}

// convertMap converts a map[any]any to a map[string]any.
func convertMap(m map[string]any) error {
	if m == nil {
		return nil
	}

	queue := []map[string]any{m}

	for len(queue) > 0 {
		curr := queue[0]

		for k, v := range curr {
			mm, ok := v.(map[any]any)
			if !ok {
				// TODO: do we need to return an error here?
				continue
			}

			ret := make(map[string]any)
			for kk, vv := range mm {
				key, err := parseKey(kk)
				if err != nil {
					return fmt.Errorf(
						"%w: %s", errExecutorConfigMustBeString, err,
					)
				}
				ret[key] = vv
			}

			delete(curr, k)
			curr[k] = ret
			queue = append(queue, ret)
		}
		queue = queue[1:]
	}

	return nil
}

// buildConfigEnv builds the environment variables from the map.
func buildConfigEnv(vars map[string]string) []string {
	var ret []string
	for k, v := range vars {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}

	return ret
}

// buildConditions builds a list of conditions from the definition.
func buildConditions(cond []*conditionDef) []Condition {
	var ret []Condition
	for _, v := range cond {
		ret = append(ret, Condition{
			Condition: v.Condition,
			Expected:  v.Expected,
		})
	}

	return ret
}

// extractParamNames extracts a slice of parameter names by removing the '$'
// from the command string.
func extractParamNames(command string) []string {
	words := strings.Fields(command)

	var params []string
	for _, word := range words {
		if strings.HasPrefix(word, "$") {
			paramName := strings.TrimPrefix(word, "$")
			params = append(params, paramName)
		}
	}

	return params
}

type scheduleKey string

const (
	scheduleKeyStart   scheduleKey = "start"
	scheduleKeyStop    scheduleKey = "stop"
	scheduleKeyRestart scheduleKey = "restart"
)

// tickerMatcher matches the command in the value string.
// Example: "`date`"
var tickerMatcher = regexp.MustCompile("`[^`]+`")

// substituteCommands substitutes command in the value string.
// This logic needs to be refactored to handle more complex cases.
func substituteCommands(input string) (string, error) {
	matches := tickerMatcher.FindAllString(strings.TrimSpace(input), -1)
	if matches == nil {
		return input, nil
	}

	ret := input
	for i := 0; i < len(matches); i++ {
		// Execute the command and replace the command with the output.
		command := matches[i]

		cmd, args := util.SplitCommand(strings.ReplaceAll(command, "`", ""))

		out, err := exec.Command(cmd, args...).Output()
		if err != nil {
			return "", err
		}

		ret = strings.ReplaceAll(
			ret, command, strings.TrimSpace(string(out[:])),
		)
	}

	return ret, nil
}
