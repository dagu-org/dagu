// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// BuilderFn is a function that builds a part of the DAG.
type BuilderFn func(ctx BuildContext, spec *definition, dag *DAG) error

// BuildContext is the context for building a DAG.
type BuildContext struct {
	ctx            context.Context
	opts           buildOpts
	additionalEnvs []string
}

// buildOpts is used to control the behavior of the builder.
type buildOpts struct {
	// base specifies the base configuration file for the DAG.
	base string
	// onlyMetadata specifies whether to build only the metadata.
	onlyMetadata bool
	// parameters specifies the parameters to the DAG.
	// parameters are used to override the default parameters in the DAG.
	parameters string
	// parametersList specifies the parameters to the DAG.
	parametersList []string
	// noEval specifies whether to evaluate dynamic fields.
	noEval bool
}

// errors on building a DAG.
var (
	errInvalidSchedule                    = errors.New("invalid schedule")
	errScheduleMustBeStringOrArray        = errors.New("schedule must be a string or an array of strings")
	errInvalidScheduleType                = errors.New("invalid schedule type")
	errInvalidKeyType                     = errors.New("invalid key type")
	errExecutorConfigMustBeString         = errors.New("executor config key must be string")
	errDuplicateFunction                  = errors.New("duplicate function")
	errFuncParamsMismatch                 = errors.New("func params and args given to func command do not match")
	errStepNameRequired                   = errors.New("step name must be specified")
	errStepCommandIsRequired              = errors.New("step command is required")
	errStepCommandIsEmpty                 = errors.New("step command is empty")
	errStepCommandMustBeArrayOrString     = errors.New("step command must be an array of strings or a string")
	errInvalidParamValue                  = errors.New("invalid parameter value")
	errCallFunctionNotFound               = errors.New("call must specify a functions that exists")
	errNumberOfParamsMismatch             = errors.New("the number of parameters defined in the function does not match the number of parameters given")
	errRequiredParameterNotFound          = errors.New("required parameter not found")
	errScheduleKeyMustBeString            = errors.New("schedule key must be a string")
	errInvalidSignal                      = errors.New("invalid signal")
	errInvalidEnvValue                    = errors.New("invalid value for env")
	errArgsMustBeConvertibleToIntOrString = errors.New("args must be convertible to either int or string")
	errExecutorTypeMustBeString           = errors.New("executor.type value must be string")
	errExecutorConfigValueMustBeMap       = errors.New("executor.config value must be a map")
	errExecutorHasInvalidKey              = errors.New("executor has invalid key")
	errExecutorConfigMustBeStringOrMap    = errors.New("executor config must be string or map")
)

var builderRegistry = []builderEntry{
	{metadata: true, name: "env", fn: buildEnvs},
	{metadata: true, name: "schedule", fn: buildSchedule},
	{metadata: true, name: "skipIfSuccessful", fn: skipIfSuccessful},
	{metadata: true, name: "params", fn: buildParams},
	{name: "mailOn", fn: buildMailOn},
	{name: "steps", fn: buildSteps},
	{name: "logDir", fn: buildLogDir},
	{name: "handlers", fn: buildHandlers},
	{name: "smtpConfig", fn: buildSMTPConfig},
	{name: "errMailConfig", fn: buildErrMailConfig},
	{name: "infoMailConfig", fn: buildInfoMailConfig},
	{name: "maxHistoryRetentionDays", fn: maxHistoryRetentionDays},
	{name: "maxCleanUpTime", fn: maxCleanUpTime},
	{name: "preconditions", fn: buildPreconditions},
}

type builderEntry struct {
	metadata bool
	name     string
	fn       BuilderFn
}

var stepBuilderRegistry = []stepBuilderEntry{
	{name: "command", fn: buildCommand},
	{name: "executor", fn: buildExecutor},
	{name: "subworkflow", fn: buildSubWorkflow},
	{name: "continueOn", fn: buildContinueOn},
	{name: "retryPolicy", fn: buildRetryPolicy},
	{name: "repeatPolicy", fn: buildRepeatPolicy},
	{name: "signalOnStop", fn: buildSignalOnStop},
}

type stepBuilderEntry struct {
	name string
	fn   StepBuilderFn
}

// StepBuilderFn is a function that builds a part of the step.
type StepBuilderFn func(ctx BuildContext, def stepDef, step *Step) error

// build builds a DAG from the specification.
func build(ctx context.Context, spec *definition, opts buildOpts, additionalEnvs []string) (*DAG, error) {
	buildCtx := BuildContext{
		ctx:            ctx,
		opts:           opts,
		additionalEnvs: additionalEnvs,
	}

	dag := &DAG{
		Name:          spec.Name,
		Group:         spec.Group,
		Description:   spec.Description,
		Timeout:       time.Second * time.Duration(spec.TimeoutSec),
		Delay:         time.Second * time.Duration(spec.DelaySec),
		RestartWait:   time.Second * time.Duration(spec.RestartWaitSec),
		Tags:          parseTags(spec.Tags),
		MaxActiveRuns: spec.MaxActiveRuns,
	}

	var errs errorList
	for _, builder := range builderRegistry {
		if !builder.metadata && opts.onlyMetadata {
			continue
		}
		if err := builder.fn(buildCtx, spec, dag); err != nil {
			errs.Add(fmt.Errorf("%s: %w", builder.name, err))
		}
	}

	if !opts.onlyMetadata {
		// TODO: Remove functions feature.
		if err := assertFunctions(spec.Functions); err != nil {
			errs.Add(err)
		}
	}

	if len(errs) > 0 {
		return nil, &errs
	}

	return dag, nil
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
func buildSchedule(_ BuildContext, spec *definition, dag *DAG) error {
	var starts, stops, restarts []string

	switch schedule := (spec.Schedule).(type) {
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
		return fmt.Errorf("%w: %T", errInvalidScheduleType, spec.Schedule)

	}

	// Parse each schedule as a cron expression.
	var err error
	dag.Schedule, err = buildScheduler(starts)
	if err != nil {
		return err
	}
	dag.StopSchedule, err = buildScheduler(stops)
	if err != nil {
		return err
	}
	dag.RestartSchedule, err = buildScheduler(restarts)
	return err
}

func buildMailOn(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.MailOn == nil {
		return nil
	}
	dag.MailOn = &MailOn{
		Failure: spec.MailOn.Failure,
		Success: spec.MailOn.Success,
	}
	return nil
}

// buildEnvs builds the environment variables for the DAG.
// Case 1: env is an array of maps with string keys and string values.
// Case 2: env is a map with string keys and string values.
func buildEnvs(ctx BuildContext, spec *definition, dag *DAG) error {
	env, err := loadVariables(ctx, spec.Env)
	if err != nil {
		return err
	}
	dag.Env = buildConfigEnv(env)

	// Add the environment variables that are defined in the base
	// configuration. If the environment variable is already defined in
	// the DAG, it is not added.
	for _, e := range ctx.additionalEnvs {
		key := strings.SplitN(e, "=", 2)[0]
		if _, ok := env[key]; !ok {
			dag.Env = append(dag.Env, e)
		}
	}
	return nil
}

// buildLogDir builds the log directory for the DAG.
func buildLogDir(_ BuildContext, spec *definition, dag *DAG) (err error) {
	dag.LogDir = spec.LogDir
	return err
}

// buildHandlers builds the handlers for the DAG.
// The handlers are executed when the DAG is stopped, succeeded, failed, or
// cancelled.
func buildHandlers(ctx BuildContext, spec *definition, dag *DAG) (err error) {
	if spec.HandlerOn.Exit != nil {
		spec.HandlerOn.Exit.Name = HandlerOnExit.String()
		if dag.HandlerOn.Exit, err = buildStep(ctx, *spec.HandlerOn.Exit, spec.Functions); err != nil {
			return err
		}
	}

	if spec.HandlerOn.Success != nil {
		spec.HandlerOn.Success.Name = HandlerOnSuccess.String()
		if dag.HandlerOn.Success, err = buildStep(ctx, *spec.HandlerOn.Success, spec.Functions); err != nil {
			return
		}
	}

	if spec.HandlerOn.Failure != nil {
		spec.HandlerOn.Failure.Name = HandlerOnFailure.String()
		if dag.HandlerOn.Failure, err = buildStep(ctx, *spec.HandlerOn.Failure, spec.Functions); err != nil {
			return
		}
	}

	if spec.HandlerOn.Cancel != nil {
		spec.HandlerOn.Cancel.Name = HandlerOnCancel.String()
		if dag.HandlerOn.Cancel, err = buildStep(ctx, *spec.HandlerOn.Cancel, spec.Functions); err != nil {
			return
		}
	}

	return nil
}

func buildPreconditions(_ BuildContext, spec *definition, dag *DAG) error {
	dag.Preconditions = buildConditions(spec.Preconditions)
	return nil
}

func maxCleanUpTime(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.MaxCleanUpTimeSec != nil {
		dag.MaxCleanUpTime = time.Second * time.Duration(*spec.MaxCleanUpTimeSec)
	}
	return nil
}

func maxHistoryRetentionDays(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.HistRetentionDays != nil {
		dag.HistRetentionDays = *spec.HistRetentionDays
	}
	return nil
}

// skipIfSuccessful sets the skipIfSuccessful field for the DAG.
func skipIfSuccessful(_ BuildContext, spec *definition, dag *DAG) error {
	dag.SkipIfSuccessful = spec.SkipIfSuccessful
	return nil
}

// buildSteps builds the steps for the DAG.
func buildSteps(ctx BuildContext, spec *definition, dag *DAG) error {
	var steps []Step
	for _, stepDef := range spec.Steps {
		step, err := buildStep(ctx, stepDef, spec.Functions)
		if err != nil {
			return err
		}
		steps = append(steps, *step)
	}
	dag.Steps = steps
	return nil
}

// buildSMTPConfig builds the SMTP configuration for the DAG.
func buildSMTPConfig(_ BuildContext, spec *definition, dag *DAG) (err error) {
	dag.SMTP = &SMTPConfig{
		Host:     spec.SMTP.Host,
		Port:     spec.SMTP.Port,
		Username: spec.SMTP.Username,
		Password: spec.SMTP.Password,
	}

	return nil
}

// buildErrMailConfig builds the error mail configuration for the DAG.
func buildErrMailConfig(_ BuildContext, spec *definition, dag *DAG) (err error) {
	dag.ErrorMail, err = buildMailConfig(spec.ErrorMail)

	return
}

// buildInfoMailConfig builds the info mail configuration for the DAG.
func buildInfoMailConfig(_ BuildContext, spec *definition, dag *DAG) (err error) {
	dag.InfoMail, err = buildMailConfig(spec.InfoMail)

	return
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

// buildStep builds a step from the step definition.
func buildStep(ctx BuildContext, def stepDef, fns []*funcDef) (*Step, error) {
	if err := assertStepDef(def, fns); err != nil {
		return nil, err
	}

	step := &Step{
		Name:           def.Name,
		Description:    def.Description,
		Shell:          def.Shell,
		Script:         def.Script,
		Stdout:         def.Stdout,
		Stderr:         def.Stderr,
		Output:         def.Output,
		Dir:            def.Dir,
		Depends:        def.Depends,
		MailOnError:    def.MailOnError,
		Preconditions:  buildConditions(def.Preconditions),
		ExecutorConfig: ExecutorConfig{Config: make(map[string]any)},
	}

	// TODO: remove the deprecated call field.
	if err := parseFuncCall(step, def.Call, fns); err != nil {
		return nil, err
	}

	for _, entry := range stepBuilderRegistry {
		if err := entry.fn(ctx, def, step); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
	}

	return step, nil
}

func buildContinueOn(_ BuildContext, def stepDef, step *Step) error {
	if def.ContinueOn != nil {
		step.ContinueOn.Skipped = def.ContinueOn.Skipped
		step.ContinueOn.Failure = def.ContinueOn.Failure
	}
	return nil
}

func buildRetryPolicy(_ BuildContext, def stepDef, step *Step) error {
	if def.RetryPolicy != nil {
		switch v := def.RetryPolicy.Limit.(type) {
		case int:
			step.RetryPolicy.Limit = v
		case string:
			step.RetryPolicy.LimitStr = v
		default:
			return fmt.Errorf("invalid type for retryPolicy.Limit: %T", v)
		}

		switch v := def.RetryPolicy.IntervalSec.(type) {
		case int:
			step.RetryPolicy.Interval = time.Second * time.Duration(v)
		case string:
			step.RetryPolicy.IntervalSecStr = v
		default:
			return fmt.Errorf("invalid type for retryPolicy.IntervalSec: %T", v)
		}
	}
	return nil
}

func buildRepeatPolicy(_ BuildContext, def stepDef, step *Step) error {
	if def.RepeatPolicy != nil {
		step.RepeatPolicy.Repeat = def.RepeatPolicy.Repeat
		step.RepeatPolicy.Interval = time.Second * time.Duration(def.RepeatPolicy.IntervalSec)
	}
	return nil
}

func buildSignalOnStop(_ BuildContext, def stepDef, step *Step) error {
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

// commandRun is not a actual command.
// subworkflow does not use this command field so it is used
// just for display purposes.
const commandRun = "run"

// buildSubWorkflow parses the subworkflow definition and sets the step fields.
func buildSubWorkflow(_ BuildContext, def stepDef, step *Step) error {
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

// buildExecutor parses the executor field in the step definition.
// Case 1: executor is nil
// Case 2: executor is a string
// Case 3: executor is a struct
func buildExecutor(_ BuildContext, def stepDef, step *Step) error {
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

func parseKey(value any) (string, error) {
	val, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %T", errInvalidKeyType, value)
	}

	return val, nil
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
