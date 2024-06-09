package dag

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/constants"

	"github.com/dagu-dev/dagu/internal/util"
	"github.com/robfig/cron/v3"
	"golang.org/x/sys/unix"
)

// EXTENSIONS is a list of supported file extensions for DAG files.
var EXTENSIONS = []string{".yaml", ".yml"}

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

// builder is used to build a DAG from a configuration definition.
type builder struct {
	def  *configDefinition
	opts buildOpts
	base *DAG
	dag  *DAG
	errs errorList
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
	errStepCommandOrCallRequired          = errors.New("either step command or step call must be specified if executor is nil")
	errStepCommandIsEmpty                 = errors.New("step command is empty")
	errStepCommandMustBeArrayOrString     = errors.New("step command must be an array of strings or a string")
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

// builderFunc is a function that builds a part of the DAG.
type builderFunc func() error

// callBuilderFunc calls a builder function and adds any errors to the error list.
func (b *builder) callBuilderFunc(fn builderFunc) {
	if err := fn(); err != nil {
		b.errs.Add(err)
	}
}

// build builds a DAG from a configuration definition and the base DAG.
func (b *builder) build(def *configDefinition, base *DAG) (*DAG, error) {
	b.def = def
	b.base = base
	b.dag = &DAG{
		Name:        def.Name,
		Group:       def.Group,
		Description: def.Description,
		Delay:       time.Second * time.Duration(def.DelaySec),
		RestartWait: time.Second * time.Duration(def.RestartWaitSec),
		Tags:        parseTags(def.Tags),
	}

	b.callBuilderFunc(b.buildEnvs)
	b.callBuilderFunc(b.buildSchedule)
	b.callBuilderFunc(b.buildMailOnConfig)
	b.callBuilderFunc(b.buildParams)

	if !b.opts.metadataOnly {
		b.callBuilderFunc(b.buildLogDir)
		b.callBuilderFunc(b.buildSteps)
		b.callBuilderFunc(b.buildHandlers)
		b.callBuilderFunc(b.buildConfig)
		b.callBuilderFunc(b.buildSMTPConfig)
		b.callBuilderFunc(b.buildErrMailConfig)
		b.callBuilderFunc(b.buildInfoMailConfig)

		if err := assertFunctions(def.Functions); err != nil {
			b.errs.Add(err)
		}
	}

	if len(b.errs) > 0 {
		return nil, &b.errs
	}

	return b.dag, nil
}

type scheduleKey string

const (
	scheduleStart   scheduleKey = "start"
	scheduleStop    scheduleKey = "stop"
	scheduleRestart scheduleKey = "restart"
)

// buildSchedule parses the schedule in different formats and builds the schedule.
// It allows for flexibility in defining the schedule.
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
	case []interface{}:
		// Case 2. schedule is an array of strings.
		// Append all the schedules to the starts slice.
		for _, s := range schedule {
			s, ok := s.(string)
			if !ok {
				return fmt.Errorf("%w, got %T: ", errScheduleMustBeStringOrArray, s)
			}
			starts = append(starts, s)
		}
	case map[any]any:
		// Case 3. schedule is a map.
		if err := parseScheduleMap(schedule, &starts, &stops, &restarts); err != nil {
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

func (b *builder) buildMailOnConfig() error {
	if b.def.MailOn == nil {
		return nil
	}
	b.dag.MailOn = &MailOn{
		Failure: b.def.MailOn.Failure,
		Success: b.def.MailOn.Success,
	}
	return nil
}

func (b *builder) buildEnvs() (err error) {
	var env map[string]string
	env, err = loadVariables(b.def.Env, b.opts)
	if err == nil {
		b.dag.Env = buildConfigEnv(env)
		if b.base != nil {
			for _, e := range b.base.Env {
				key := strings.SplitN(e, "=", 2)[0]
				if _, ok := env[key]; !ok {
					b.dag.Env = append(b.dag.Env, e)
				}
			}
		}
	}
	return
}

func (b *builder) buildLogDir() (err error) {
	b.dag.LogDir, err = util.ParseVariable(b.def.LogDir)
	return err
}

func (b *builder) buildParams() (err error) {
	b.dag.DefaultParams = b.def.Params
	p := b.dag.DefaultParams
	if b.opts.parameters != "" {
		p = b.opts.parameters
	}
	var envs []string
	b.dag.Params, envs, err = parseParameters(p, !b.opts.noEval, b.opts)
	if err == nil {
		b.dag.Env = append(b.dag.Env, envs...)
	}
	return
}

func (b *builder) buildHandlers() (err error) {
	if b.def.HandlerOn.Exit != nil {
		b.def.HandlerOn.Exit.Name = constants.OnExit
		if b.dag.HandlerOn.Exit, err = buildStep(b.dag.Env, b.def.HandlerOn.Exit, b.def.Functions, b.opts); err != nil {
			return err
		}
	}

	if b.def.HandlerOn.Success != nil {
		b.def.HandlerOn.Success.Name = constants.OnSuccess
		if b.dag.HandlerOn.Success, err = buildStep(b.dag.Env, b.def.HandlerOn.Success, b.def.Functions, b.opts); err != nil {
			return
		}
	}

	if b.def.HandlerOn.Failure != nil {
		b.def.HandlerOn.Failure.Name = constants.OnFailure
		if b.dag.HandlerOn.Failure, err = buildStep(b.dag.Env, b.def.HandlerOn.Failure, b.def.Functions, b.opts); err != nil {
			return
		}
	}

	if b.def.HandlerOn.Cancel != nil {
		b.def.HandlerOn.Cancel.Name = constants.OnCancel
		if b.dag.HandlerOn.Cancel, err = buildStep(b.dag.Env, b.def.HandlerOn.Cancel, b.def.Functions, b.opts); err != nil {
			return
		}
	}
	return nil
}

func (b *builder) buildConfig() (err error) {
	if b.def.HistRetentionDays != nil {
		b.dag.HistRetentionDays = *b.def.HistRetentionDays
	}
	b.dag.Preconditions = loadPreCondition(b.def.Preconditions)
	b.dag.MaxActiveRuns = b.def.MaxActiveRuns

	if b.def.MaxCleanUpTimeSec != nil {
		b.dag.MaxCleanUpTime = time.Second * time.Duration(*b.def.MaxCleanUpTimeSec)
	}
	return nil
}

func parseParameters(value string, eval bool, options buildOpts) (
	params []string,
	envs []string,
	err error,
) {
	var parsedParams []util.Parameter
	parsedParams, err = util.ParseParams(value, eval)
	if err != nil {
		return
	}

	ret := []string{}
	for i, p := range parsedParams {
		if eval {
			p.Value = os.ExpandEnv(p.Value)
		}
		strParam := util.StringifyParam(p)
		ret = append(ret, strParam)

		if p.Name == "" {
			strParam = p.Value
		}
		if err = os.Setenv(strconv.Itoa(i+1), strParam); err != nil {
			return
		}
		if !options.noEval {
			if p.Name != "" {
				envs = append(envs, strParam)
				err = os.Setenv(p.Name, p.Value)
				if err != nil {
					return
				}
			}
		}
	}
	return ret, envs, nil
}

type envVariable struct {
	key string
	val string
}

// nolint // cognitive complexity
func loadVariables(strVariables interface{}, opts buildOpts) (
	map[string]string, error,
) {
	var vals []*envVariable
	loadFn := func(a []*envVariable, m map[interface{}]interface{}) ([]*envVariable, error) {
		for k, v := range m {
			if k, ok := k.(string); ok {
				if vv, ok := v.(string); ok {
					a = append(a, &envVariable{k, vv})
				} else {
					return a, fmt.Errorf("%w: %s", errInvalidEnvValue, v)
				}
			}
		}
		return a, nil
	}

	var err error
	if a, ok := strVariables.(map[interface{}]interface{}); ok {
		vals, err = loadFn(vals, a)
		if err != nil {
			return nil, err
		}
	}

	if a, ok := strVariables.([]interface{}); ok {
		for _, v := range a {
			if aa, ok := v.(map[interface{}]interface{}); ok {
				vals, err = loadFn(vals, aa)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	vars := map[string]string{}
	for _, v := range vals {
		parsed, err := util.ParseVariable(v.val)
		if err != nil {
			return nil, err
		}
		vars[v.key] = parsed
		if !opts.noEval {
			err = os.Setenv(v.key, parsed)
			if err != nil {
				return nil, err
			}
		}
	}
	return vars, nil
}

func (b *builder) buildSteps() error {
	var ret []Step
	for _, stepDef := range b.def.Steps {
		step, err := buildStep(b.dag.Env, stepDef, b.def.Functions, b.opts)
		if err != nil {
			return err
		}
		ret = append(ret, *step)
	}
	b.dag.Steps = ret

	return nil
}

func (b *builder) buildSMTPConfig() (err error) {
	b.dag.Smtp = &SmtpConfig{
		Host:     os.ExpandEnv(b.def.Smtp.Host),
		Port:     os.ExpandEnv(b.def.Smtp.Port),
		Username: os.ExpandEnv(b.def.Smtp.Username),
		Password: os.ExpandEnv(b.def.Smtp.Password),
	}
	return nil
}

func (b *builder) buildErrMailConfig() (err error) {
	b.dag.ErrorMail, err = buildMailConfigFromDefinition(b.def.ErrorMail)
	return
}

func (b *builder) buildInfoMailConfig() (err error) {
	b.dag.InfoMail, err = buildMailConfigFromDefinition(b.def.InfoMail)
	return
}

// nolint // cognitive complexity
func buildStep(variables []string, def *stepDef, fns []*funcDef, options buildOpts) (*Step, error) {
	if err := assertStepDef(def, fns); err != nil {
		return nil, err
	}
	step := &Step{}
	step.Name = def.Name
	step.Description = def.Description

	if err := parseFuncCall(step, def.Call, fns); err != nil {
		return nil, err
	}

	if err := parseCommand(step, def.Command); err != nil {
		return nil, err
	}

	step.Script = def.Script
	step.Stdout = expandEnv(def.Stdout, options)
	step.Stderr = expandEnv(def.Stderr, options)
	step.Output = def.Output
	step.Dir = expandEnv(def.Dir, options)
	step.ExecutorConfig.Config = map[string]interface{}{}
	if err := parseExecutor(step, def.Executor); err != nil {
		return nil, err
	}

	// Convert map[interface{}]interface{} to map[string]interface{}
	if step.ExecutorConfig.Config != nil {
		if err := convertMap(step.ExecutorConfig.Config); err != nil {
			return nil, err
		}
	}

	// TODO: validate executor config
	step.Variables = variables
	step.Depends = def.Depends
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
		step.RepeatPolicy.Interval = time.Second * time.Duration(def.RepeatPolicy.IntervalSec)
	}
	if def.SignalOnStop != nil {
		sigDef := *def.SignalOnStop
		sig := unix.SignalNum(sigDef)
		if sig == 0 {
			return nil, fmt.Errorf("%w: %s", errInvalidSignal, sigDef)
		}
		step.SignalOnStop = sigDef
	}
	step.MailOnError = def.MailOnError
	step.Preconditions = loadPreCondition(def.Preconditions)

	if err := parseSubWorkflow(step, def.Run, def.Params); err != nil {
		return nil, err
	}

	return step, nil
}

func parseSubWorkflow(step *Step, name, params string) error {
	if name == "" {
		return nil
	}
	step.SubDAG = &SubWorkflow{
		Name:   name,
		Params: params,
	}
	step.ExecutorConfig.Type = ExecutorTypeSubWorkflow
	step.Command = fmt.Sprintf("run")
	step.Args = []string{name, params}
	step.CmdWithArgs = fmt.Sprintf("%s %s", name, params)
	return nil
}

func parseExecutor(step *Step, executor any) error {
	if executor == nil {
		return nil
	}
	switch val := executor.(type) {
	case string:
		step.ExecutorConfig.Type = val
	case map[any]any:
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return errExecutorConfigMustBeString
			}
			switch k {
			case "type":
				typ, ok := v.(string)
				if !ok {
					return errExecutorTypeMustBeString
				}
				step.ExecutorConfig.Type = typ
			case "config":
				configMap, ok := v.(map[any]any)
				if !ok {
					return errExecutorConfigValueMustBeMap
				}
				for k, v := range configMap {
					k, ok := k.(string)
					if !ok {
						return errExecutorConfigMustBeString
					}
					step.ExecutorConfig.Config[k] = v
				}
			default:
				return fmt.Errorf("%w: %s", errExecutorHasInvalidKey, k)
			}
		}
	default:
		return errExecutorConfigMustBeStringOrMap
	}
	return nil
}

func parseCommand(step *Step, command any) error {
	if command == nil {
		return nil
	}
	switch val := command.(type) {
	case string:
		if val == "" {
			return errStepCommandIsEmpty
		}
		step.CmdWithArgs = val
		step.Command, step.Args = util.SplitCommand(val, false)
	case []any:
		for _, v := range val {
			val, ok := v.(string)
			if !ok {
				val = fmt.Sprintf("%v", v)
			}
			if step.Command == "" {
				step.Command = val
				continue
			}
			step.Args = append(step.Args, val)
		}
	default:
		return errStepCommandMustBeArrayOrString
	}
	return nil
}

func parseFuncCall(step *Step, call *callFuncDef, funcs []*funcDef) error {
	if call == nil {
		return nil
	}
	step.Args = make([]string, 0, len(call.Args))
	passedArgs := map[string]string{}
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
	step.Command = util.RemoveParams(calledFuncDef.Command)
	step.CmdWithArgs = util.AssignValues(calledFuncDef.Command, passedArgs)
	return nil
}

func expandEnv(val string, options buildOpts) string {
	if options.noEval {
		return val
	}
	return os.ExpandEnv(val)
}

func convertMap(m map[string]interface{}) error {
	convertKey := func(v interface{}) (interface{}, error) {
		switch v.(type) {
		case string:
			return v, nil
		default:
			return nil, fmt.Errorf("%w: %t", errInvalidKeyType, v)
		}
	}

	queue := []map[string]interface{}{m}

	for len(queue) > 0 {
		curr := queue[0]
		for k, v := range curr {
			switch v := v.(type) {
			case map[interface{}]interface{}:
				ret := map[string]interface{}{}
				for kk, vv := range v {
					kk, err := convertKey(kk)
					if err != nil {
						return fmt.Errorf("%w: %s", errExecutorConfigMustBeString, err)
					}
					ret[kk.(string)] = vv
				}
				delete(curr, k)
				curr[k] = ret
				queue = append(queue, ret)
			}
		}
		queue = queue[1:]
	}

	return nil
}

func buildMailConfigFromDefinition(def mailConfigDef) (*MailConfig, error) {
	return &MailConfig{
		From:       def.From,
		To:         def.To,
		Prefix:     def.Prefix,
		AttachLogs: def.AttachLogs,
	}, nil
}

func buildConfigEnv(vars map[string]string) []string {
	var ret []string
	for k, v := range vars {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func loadPreCondition(cond []*conditionDef) []*Condition {
	var ret []*Condition
	for _, v := range cond {
		ret = append(ret, &Condition{
			Condition: v.Condition,
			Expected:  v.Expected,
		})
	}
	return ret
}

func parseTags(value string) []string {
	values := strings.Split(value, ",")
	ret := []string{}
	for _, v := range values {
		tag := strings.ToLower(strings.TrimSpace(v))
		if tag != "" {
			ret = append(ret, tag)
		}
	}
	return ret
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func parseSchedules(values []string) ([]*Schedule, error) {
	var ret []*Schedule
	for _, v := range values {
		parsed, err := cronParser.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidSchedule, err)
		}
		ret = append(ret, &Schedule{Expression: v, Parsed: parsed})
	}
	return ret, nil
}

// only assert functions clause
func assertFunctions(fns []*funcDef) error {
	if fns == nil {
		return nil
	}

	nameMap := make(map[string]bool)
	for _, funcDef := range fns {
		if _, exists := nameMap[funcDef.Name]; exists {
			return errDuplicateFunction
		}
		nameMap[funcDef.Name] = true

		definedParamNames := strings.Split(funcDef.Params, " ")
		passedParamNames := util.ExtractParamNames(funcDef.Command)
		if len(definedParamNames) != len(passedParamNames) {
			return errFuncParamsMismatch
		}

		for i := 0; i < len(definedParamNames); i++ {
			if definedParamNames[i] != passedParamNames[i] {
				return errFuncParamsMismatch
			}
		}
	}

	return nil
}

func assertStepDef(def *stepDef, funcs []*funcDef) error {
	if def.Name == "" {
		return errStepNameRequired
	}
	// TODO: Refactor the validation check for each executor.
	if def.Executor == nil && def.Command == nil && def.Call == nil && def.Run == "" {
		return errStepCommandOrCallRequired
	}

	if def.Call != nil {
		calledFunc := def.Call.Function
		calledFuncDef := &funcDef{}
		for _, funcDef := range funcs {
			if funcDef.Name == calledFunc {
				calledFuncDef = funcDef
				break
			}
		}
		if calledFuncDef.Name == "" {
			return errCallFunctionNotFound
		}

		definedParamNames := strings.Split(calledFuncDef.Params, " ")
		if len(def.Call.Args) != len(definedParamNames) {
			return errNumberOfParamsMismatch
		}

		for _, paramName := range definedParamNames {
			_, exists := def.Call.Args[paramName]
			if !exists {
				return errRequiredParameterNotFound
			}
		}
	}

	return nil
}

// parseScheduleMap parses the schedule map and populates the starts, stops, and restarts slices.
// Each key in the map must be either "start", "stop", or "restart".
// The value can be Case 1 or Case 2.
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
// nolint // cognitive complexity
func parseScheduleMap(scheduleMap map[any]any, starts, stops, restarts *[]string) error {
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
		case []interface{}:
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
		case scheduleStart:
			targets = starts
		case scheduleStop:
			targets = stops
		case scheduleRestart:
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
