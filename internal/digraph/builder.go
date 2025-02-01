package digraph

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"golang.org/x/sys/unix"
)

// BuilderFn is a function that builds a part of the DAG.
type BuilderFn func(ctx BuildContext, spec *definition, dag *DAG) error

// BuildContext is the context for building a DAG.
type BuildContext struct {
	ctx  context.Context
	file string
	opts buildOpts
}

func (c BuildContext) WithOpts(opts buildOpts) BuildContext {
	copy := c
	copy.opts = opts
	return copy
}

func (c BuildContext) WithFile(file string) BuildContext {
	copy := c
	copy.file = file
	return copy
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

var builderRegistry = []builderEntry{
	{metadata: true, name: "env", fn: buildEnvs},
	{metadata: true, name: "schedule", fn: buildSchedule},
	{metadata: true, name: "skipIfSuccessful", fn: skipIfSuccessful},
	{metadata: true, name: "params", fn: buildParams},
	{name: "dotenv", fn: buildDotenv},
	{name: "mailOn", fn: buildMailOn},
	{name: "steps", fn: buildSteps},
	{name: "logDir", fn: buildLogDir},
	{name: "handlers", fn: buildHandlers},
	{name: "smtpConfig", fn: buildSMTPConfig},
	{name: "errMailConfig", fn: buildErrMailConfig},
	{name: "infoMailConfig", fn: buildInfoMailConfig},
	{name: "maxHistoryRetentionDays", fn: maxHistoryRetentionDays},
	{name: "maxCleanUpTime", fn: maxCleanUpTime},
	{name: "preconditions", fn: buildPrecondition},
}

type builderEntry struct {
	metadata bool
	name     string
	fn       BuilderFn
}

var stepBuilderRegistry = []stepBuilderEntry{
	{name: "executor", fn: buildExecutor},
	{name: "command", fn: buildCommand},
	{name: "depends", fn: buildDepends},
	{name: "subworkflow", fn: buildSubWorkflow},
	{name: "continueOn", fn: buildContinueOn},
	{name: "retryPolicy", fn: buildRetryPolicy},
	{name: "repeatPolicy", fn: buildRepeatPolicy},
	{name: "signalOnStop", fn: buildSignalOnStop},
	{name: "precondition", fn: buildStepPrecondition},
}

type stepBuilderEntry struct {
	name string
	fn   StepBuilderFn
}

// StepBuilderFn is a function that builds a part of the step.
type StepBuilderFn func(ctx BuildContext, def stepDef, step *Step) error

// build builds a DAG from the specification.
func build(ctx BuildContext, spec *definition) (*DAG, error) {
	dag := &DAG{
		Location:      ctx.file,
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
		if !builder.metadata && ctx.opts.onlyMetadata {
			continue
		}
		if err := builder.fn(ctx, spec, dag); err != nil {
			errs.Add(wrapError(builder.name, nil, err))
		}
	}

	if !ctx.opts.onlyMetadata {
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
				return wrapError("schedule", s, errScheduleMustBeStringOrArray)
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
		return wrapError("schedule", spec.Schedule, errInvalidScheduleType)

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

func buildDotenv(ctx BuildContext, spec *definition, dag *DAG) error {
	switch v := spec.Dotenv.(type) {
	case nil:
		return nil

	case string:
		dag.Dotenv = append(dag.Dotenv, v)

	case []any:
		for _, e := range v {
			switch e := e.(type) {
			case string:
				dag.Dotenv = append(dag.Dotenv, e)
			default:
				return wrapError("dotenv", e, errDotenvMustBeStringOrArray)
			}
		}
	default:
		return wrapError("dotenv", v, errDotenvMustBeStringOrArray)
	}

	if !ctx.opts.noEval {
		var relativeTos []string
		if ctx.file != "" {
			relativeTos = append(relativeTos, ctx.file)
		}

		resolver := fileutil.NewFileResolver(relativeTos)
		for _, filePath := range dag.Dotenv {
			filePath, err := cmdutil.EvalString(ctx.ctx, filePath)
			if err != nil {
				return wrapError("dotenv", filePath, fmt.Errorf("failed to evaluate dotenv file path %s: %w", filePath, err))
			}
			resolvedPath, err := resolver.ResolveFilePath(filePath)
			if err != nil {
				continue
			}
			if err := godotenv.Load(resolvedPath); err != nil {
				return wrapError("dotenv", filePath, fmt.Errorf("failed to load dotenv file %s: %w", filePath, err))
			}
			// Break after the first successful load.
			break
		}
	}

	return nil
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
	vars, err := loadVariables(ctx, spec.Env)
	if err != nil {
		return err
	}

	for k, v := range vars {
		dag.Env = append(dag.Env, fmt.Sprintf("%s=%s", k, v))
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

func buildPrecondition(ctx BuildContext, spec *definition, dag *DAG) error {
	// Parse both `preconditions` and `precondition` fields.
	conditions, err := parsePrecondition(ctx, spec.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx, spec.Precondition)
	if err != nil {
		return err
	}

	dag.Preconditions = conditions
	dag.Preconditions = append(dag.Preconditions, condition...)

	return nil
}

func parsePrecondition(ctx BuildContext, precondition any) ([]Condition, error) {
	switch v := precondition.(type) {
	case nil:
		return nil, nil

	case string:
		return []Condition{{Command: v}}, nil

	case map[any]any:
		var ret Condition
		for k, vv := range v {
			key, ok := k.(string)
			if !ok {
				return nil, wrapError("preconditions", k, errPreconditionKeyMustBeString)
			}

			switch strings.ToLower(key) {
			case "condition":
				ret.Condition, ok = vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, errPreconditionValueMustBeString)
				}

			case "expected":
				ret.Expected, ok = vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, errPreconditionValueMustBeString)
				}

			case "command":
				ret.Command, ok = vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, errPreconditionValueMustBeString)
				}

			default:
				return nil, wrapError("preconditions", k, fmt.Errorf("%w: %s", errPreconditionHasInvalidKey, key))

			}
		}

		if err := ret.Validate(); err != nil {
			return nil, wrapError("preconditions", v, err)
		}

		return []Condition{ret}, nil

	case []any:
		var ret []Condition
		for _, vv := range v {
			parsed, err := parsePrecondition(ctx, vv)
			if err != nil {
				return nil, err
			}
			ret = append(ret, parsed...)
		}
		return ret, nil

	default:
		return nil, wrapError("preconditions", v, errPreconditionMustBeArrayOrString)

	}
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
	switch v := spec.Steps.(type) {
	case nil:
		return nil

	case []any:
		var stepDefs []stepDef
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepDefs,
		})
		if err := md.Decode(v); err != nil {
			return wrapError("steps", v, err)
		}
		for _, stepDef := range stepDefs {
			step, err := buildStep(ctx, stepDef, spec.Functions)
			if err != nil {
				return err
			}
			dag.Steps = append(dag.Steps, *step)
		}

		return nil

	case map[any]any:
		stepDefs := make(map[string]stepDef)
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepDefs,
		})
		if err := md.Decode(v); err != nil {
			return wrapError("steps", v, err)
		}
		for name, stepDef := range stepDefs {
			stepDef.Name = name
			step, err := buildStep(ctx, stepDef, spec.Functions)
			if err != nil {
				return err
			}
			dag.Steps = append(dag.Steps, *step)
		}

		return nil

	default:
		return wrapError("steps", v, errStepsMustBeArrayOrMap)

	}
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
		MailOnError:    def.MailOnError,
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
	if def.ContinueOn == nil {
		return nil
	}
	step.ContinueOn.Skipped = def.ContinueOn.Skipped
	step.ContinueOn.Failure = def.ContinueOn.Failure
	step.ContinueOn.MarkSuccess = def.ContinueOn.MarkSuccess

	exitCodes, err := parseIntOrArray(def.ContinueOn.ExitCode)
	if err != nil {
		return wrapError("continueOn.exitCode", def.ContinueOn.ExitCode, errContinueOnExitCodeMustBeIntOrArray)
	}
	step.ContinueOn.ExitCode = exitCodes

	output, err := parseStringOrArray(def.ContinueOn.Output)
	if err != nil {
		return wrapError("continueOn.stdout", def.ContinueOn.Output, errContinueOnOutputMustBeStringOrArray)
	}
	step.ContinueOn.Output = output

	return nil
}

// buildRetryPolicy builds the retry policy for a step.
func buildRetryPolicy(_ BuildContext, def stepDef, step *Step) error {
	if def.RetryPolicy != nil {
		switch v := def.RetryPolicy.Limit.(type) {
		case int:
			step.RetryPolicy.Limit = v
		case string:
			step.RetryPolicy.LimitStr = v
		default:
			return wrapError("retryPolicy.Limit", v, fmt.Errorf("invalid type: %T", v))
		}

		switch v := def.RetryPolicy.IntervalSec.(type) {
		case int:
			step.RetryPolicy.Interval = time.Second * time.Duration(v)
		case string:
			step.RetryPolicy.IntervalSecStr = v
		default:
			return wrapError("retryPolicy.IntervalSec", v, fmt.Errorf("invalid type: %T", v))
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

func buildStepPrecondition(ctx BuildContext, def stepDef, step *Step) error {
	// Parse both `preconditions` and `precondition` fields.
	conditions, err := parsePrecondition(ctx, def.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx, def.Precondition)
	if err != nil {
		return err
	}
	step.Preconditions = conditions
	step.Preconditions = append(step.Preconditions, condition...)
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

func buildDepends(_ BuildContext, def stepDef, step *Step) error {
	deps, err := parseStringOrArray(def.Depends)
	if err != nil {
		return wrapError("depends", def.Depends, errDependsMustBeStringOrArray)
	}
	step.Depends = deps

	return nil
}

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
				return wrapError("executor.config", k, errExecutorConfigMustBeString)
			}

			switch key {
			case executorKeyType:
				// Executor type is a string.
				typ, ok := v.(string)
				if !ok {
					return wrapError("executor.type", v, errExecutorTypeMustBeString)
				}
				step.ExecutorConfig.Type = typ

			case executorKeyConfig:
				// Executor config is a map of string keys and values.
				// The values can be of any type.
				// It is up to the executor to parse the values.
				executorConfig, ok := v.(map[any]any)
				if !ok {
					return wrapError("executor.config", v, errExecutorConfigValueMustBeMap)
				}
				for k, v := range executorConfig {
					configKey, ok := k.(string)
					if !ok {
						return wrapError("executor.config", k, errExecutorConfigMustBeString)
					}
					step.ExecutorConfig.Config[configKey] = v
				}

			default:
				// Unknown key in the executor config.
				return wrapError("executor.config", key, fmt.Errorf("%w: %s", errExecutorHasInvalidKey, key))

			}
		}

	default:
		// Unknown key for executor field.
		return wrapError("executor", val, errExecutorConfigMustBeStringOrMap)

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

func parseIntOrArray(v any) ([]int, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil

	case int:
		return []int{v}, nil

	case []any:
		var ret []int
		for _, vv := range v {
			i, ok := vv.(int)
			if !ok {
				return nil, fmt.Errorf("int or array expected, got %T", vv)
			}
			ret = append(ret, i)
		}
		return ret, nil

	case string:
		// try to parse the string as an integer
		exitCode, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("int or array expected, got %T", v)
		}
		return []int{exitCode}, nil

	default:
		return nil, fmt.Errorf("int or array expected, got %T", v)

	}
}

func parseStringOrArray(v any) ([]string, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil

	case string:
		return []string{v}, nil

	case []any:
		var ret []string
		for _, vv := range v {
			s, ok := vv.(string)
			if !ok {
				return nil, fmt.Errorf("string or array expected, got %T", vv)
			}
			ret = append(ret, s)
		}
		return ret, nil

	default:
		return nil, fmt.Errorf("string or array expected, got %T", v)

	}
}
