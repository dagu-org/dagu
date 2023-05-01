package dag

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/errors"
	"github.com/yohamta/dagu/internal/utils"
	"golang.org/x/sys/unix"
)

var EXTENSIONS = []string{".yaml", ".yml"}

type BuildDAGOptions struct {
	loadMetadataOnly bool
	parameters       string
	skipEnvEval      bool
	skipEnvSetup     bool
	defaultEnvs      map[string]string
}
type DAGBuilder struct {
	options    BuildDAGOptions
	baseConfig *DAG
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func (b *DAGBuilder) buildFromDefinition(def *configDefinition, baseConfig *DAG) (d *DAG, err error) {
	b.baseConfig = baseConfig

	d = &DAG{}

	setDAGProperties(def, d)

	errList := &errors.ErrorList{}

	errList.Add(buildSchedule(def, d))
	if !b.options.skipEnvEval {
		errList.Add(buildEnvs(def, d, b.baseConfig, b.options))
	}
	errList.Add(buildParams(def, d, b.options))

	if errList.HasErrors() {
		return nil, errList
	}

	if b.options.loadMetadataOnly {
		return
	}

	errList.Add(buildAll(def, d, b.options))
	if errList.HasErrors() {
		return nil, errList
	}
	return d, nil
}

func buildAll(def *configDefinition, d *DAG, options BuildDAGOptions) error {
	errList := &errors.ErrorList{}

	errList.Add(buildLogDir(def, d))
	errList.Add(assertFunctions(def.Functions))
	errList.Add(buildSteps(def, d, options))
	errList.Add(buildHandlers(def, d, options))
	errList.Add(buildConfig(def, d))
	errList.Add(buildSMTPConfig(def, d))
	errList.Add(buildErrMailConfig(def, d))
	errList.Add(buildInfoMailConfig(def, d))

	if errList.HasErrors() {
		return errList
	}

	return nil
}

const (
	scheduleStart   = "start"
	scheduleStop    = "stop"
	scheduleRestart = "restart"
)

func setDAGProperties(def *configDefinition, d *DAG) {
	d.Name = def.Name
	if def.Name != "" {
		d.Name = def.Name
	}
	d.Group = def.Group
	d.Description = def.Description
	if def.MailOn != nil {
		d.MailOn = &MailOn{
			Failure: def.MailOn.Failure,
			Success: def.MailOn.Success,
		}
	}
	d.Delay = time.Second * time.Duration(def.DelaySec)
	d.RestartWait = time.Second * time.Duration(def.RestartWaitSec)
	d.Tags = parseTags(def.Tags)
}

func buildSchedule(def *configDefinition, d *DAG) error {
	starts := []string{}
	stops := []string{}
	restarts := []string{}

	switch (def.Schedule).(type) {
	case string:
		starts = append(starts, def.Schedule.(string))
	case []interface{}:
		for _, s := range def.Schedule.([]interface{}) {
			if s, ok := s.(string); ok {
				starts = append(starts, s)
			} else {
				return fmt.Errorf("schedule must be a string or an array of strings")
			}
		}
	case map[interface{}]interface{}:
		if err := parseScheduleMap(def.Schedule.(map[interface{}]interface{}), &starts, &stops, &restarts); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("invalid schedule type: %T", def.Schedule)
	}

	var err error
	d.Schedule, err = parseSchedule(starts)
	if err != nil {
		return err
	}
	d.StopSchedule, err = parseSchedule(stops)
	if err != nil {
		return err
	}
	d.RestartSchedule, err = parseSchedule(restarts)
	return err
}

func buildEnvs(def *configDefinition, d, base *DAG, options BuildDAGOptions) (err error) {
	var env map[string]string
	env, err = loadVariables(def.Env, options)
	if err == nil {
		d.Env = buildConfigEnv(env)
		if base != nil {
			for _, e := range base.Env {
				key := strings.SplitN(e, "=", 2)[0]
				if _, ok := env[key]; !ok {
					d.Env = append(d.Env, e)
				}
			}
		}
	}
	return
}

func buildLogDir(def *configDefinition, d *DAG) (err error) {
	d.LogDir, err = utils.ParseVariable(def.LogDir)
	return err
}

func buildParams(def *configDefinition, d *DAG, options BuildDAGOptions) (err error) {
	d.DefaultParams = def.Params
	p := d.DefaultParams
	if options.parameters != "" {
		p = options.parameters
	}
	var envs []string
	d.Params, envs, err = parseParameters(p, !options.skipEnvEval, options)
	if err == nil {
		d.Env = append(d.Env, envs...)
	}
	return
}

func buildHandlers(def *configDefinition, d *DAG, options BuildDAGOptions) (err error) {
	if def.HandlerOn.Exit != nil {
		def.HandlerOn.Exit.Name = constants.OnExit
		if d.HandlerOn.Exit, err = buildStep(d.Env, def.HandlerOn.Exit, def.Functions, options); err != nil {
			return err
		}
	}

	if def.HandlerOn.Success != nil {
		def.HandlerOn.Success.Name = constants.OnSuccess
		if d.HandlerOn.Success, err = buildStep(d.Env, def.HandlerOn.Success, def.Functions, options); err != nil {
			return
		}
	}

	if def.HandlerOn.Failure != nil {
		def.HandlerOn.Failure.Name = constants.OnFailure
		if d.HandlerOn.Failure, err = buildStep(d.Env, def.HandlerOn.Failure, def.Functions, options); err != nil {
			return
		}
	}

	if def.HandlerOn.Cancel != nil {
		def.HandlerOn.Cancel.Name = constants.OnCancel
		if d.HandlerOn.Cancel, err = buildStep(d.Env, def.HandlerOn.Cancel, def.Functions, options); err != nil {
			return
		}
	}
	return nil
}

func buildConfig(def *configDefinition, d *DAG) (err error) {
	if def.HistRetentionDays != nil {
		d.HistRetentionDays = *def.HistRetentionDays
	}
	d.Preconditions = loadPreCondition(def.Preconditions)
	d.MaxActiveRuns = def.MaxActiveRuns

	if def.MaxCleanUpTimeSec != nil {
		d.MaxCleanUpTime = time.Second * time.Duration(*def.MaxCleanUpTimeSec)
	}
	return nil
}

func parseParameters(value string, eval bool, options BuildDAGOptions) (
	params []string,
	envs []string,
	err error,
) {
	var parsedParams []utils.Parameter
	parsedParams, err = utils.ParseParams(value, eval)
	if err != nil {
		return
	}

	ret := []string{}
	for i, p := range parsedParams {
		if eval {
			p.Value = os.ExpandEnv(p.Value)
		}
		strParam := utils.StringifyParam(p)
		ret = append(ret, strParam)

		if p.Name == "" {
			strParam = p.Value
		}
		if err = os.Setenv(strconv.Itoa(i+1), strParam); err != nil {
			return
		}
		if !options.skipEnvSetup {
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

func loadVariables(strVariables interface{}, options BuildDAGOptions) (
	map[string]string, error,
) {
	var vals []*envVariable = []*envVariable{}
	for k, v := range options.defaultEnvs {
		vals = append(vals, &envVariable{k, v})
	}

	loadFn := func(a []*envVariable, m map[interface{}]interface{}) ([]*envVariable, error) {
		for k, v := range m {
			if k, ok := k.(string); ok {
				if vv, ok := v.(string); ok {
					a = append(a, &envVariable{k, vv})
				} else {
					return a, fmt.Errorf("invalid value for env %s", v)
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
		parsed, err := utils.ParseVariable(v.val)
		if err != nil {
			return nil, err
		}
		vars[v.key] = parsed
		if !options.skipEnvSetup {
			err = os.Setenv(v.key, parsed)
			if err != nil {
				return nil, err
			}
		}
	}
	return vars, nil
}

func buildSteps(def *configDefinition, d *DAG, options BuildDAGOptions) error {
	ret := []*Step{}
	for _, stepDef := range def.Steps {
		step, err := buildStep(d.Env, stepDef, def.Functions, options)
		if err != nil {
			return err
		}
		ret = append(ret, step)
	}
	d.Steps = ret

	return nil
}

func buildStep(variables []string, def *stepDef, funcs []*funcDef, options BuildDAGOptions) (*Step, error) {
	if err := assertStepDef(def, funcs); err != nil {
		return nil, err
	}
	step := &Step{}
	step.Name = def.Name
	step.Description = def.Description
	if def.Call != nil {
		step.Args = make([]string, 0, len(def.Call.Args))
		passedArgs := map[string]string{}
		for k, v := range def.Call.Args {
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

			return nil, fmt.Errorf("args must be convertible to either int or string")
		}

		calledFuncDef := &funcDef{}
		for _, funcDef := range funcs {
			if funcDef.Name == def.Call.Function {
				calledFuncDef = funcDef
				break
			}
		}
		step.Command = utils.RemoveParams(calledFuncDef.Command)
		step.CmdWithArgs = utils.AssignValues(calledFuncDef.Command, passedArgs)
	} else {
		step.CmdWithArgs = def.Command
		step.Command, step.Args = utils.SplitCommand(step.CmdWithArgs, false)
	}

	step.Script = def.Script
	step.Stdout = expandEnv(def.Stdout, options)
	step.Stderr = expandEnv(def.Stderr, options)
	step.Output = def.Output
	step.Dir = expandEnv(def.Dir, options)
	step.ExecutorConfig.Config = map[string]interface{}{}
	if def.Executor != nil {
		switch val := (def.Executor).(type) {
		case string:
			step.ExecutorConfig.Type = val
		case map[interface{}]interface{}:
			for k, v := range val {
				if k, ok := k.(string); ok {
					switch k {
					case "type":
						if v, ok := v.(string); ok {
							step.ExecutorConfig.Type = v
						} else {
							return nil, fmt.Errorf("executor.type value must be string")
						}
					case "config":
						if v, ok := v.(map[interface{}]interface{}); ok {
							for k, v := range v {
								if k, ok := k.(string); ok {
									step.ExecutorConfig.Config[k] = v
								} else {
									return nil, fmt.Errorf("executor.config key must be string")
								}
							}
						} else {
							return nil, fmt.Errorf("executor.config value must be a map")
						}
					default:
						return nil, fmt.Errorf("executor has invalid key '%s'", k)
					}
				} else {
					return nil, fmt.Errorf("executor config map key must be string")
				}
			}
		default:
			return nil, fmt.Errorf("executor config must be string or map")
		}
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
			return nil, fmt.Errorf("invalid signal: %s", sigDef)
		}
		step.SignalOnStop = sigDef
	}
	step.MailOnError = def.MailOnError
	step.Preconditions = loadPreCondition(def.Preconditions)
	return step, nil
}

func expandEnv(val string, options BuildDAGOptions) string {
	if options.skipEnvEval {
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
			return nil, fmt.Errorf("invalid key type: %t", v)
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
					if kk, err := convertKey(kk); err != nil {
						return fmt.Errorf("executor config key must be string: %s", err)
					} else {
						ret[kk.(string)] = vv
					}
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

func buildSMTPConfig(def *configDefinition, d *DAG) (err error) {
	smtp := &SmtpConfig{}
	smtp.Host = os.ExpandEnv(def.Smtp.Host)
	smtp.Port = os.ExpandEnv(def.Smtp.Port)
	smtp.Username = os.ExpandEnv(def.Smtp.Username)
	smtp.Password = os.ExpandEnv(def.Smtp.Password)
	d.Smtp = smtp
	return nil
}

func buildErrMailConfig(def *configDefinition, d *DAG) (err error) {
	d.ErrorMail, err = buildMailConfigFromDefinition(def.ErrorMail)
	return
}

func buildInfoMailConfig(def *configDefinition, d *DAG) (err error) {
	d.InfoMail, err = buildMailConfigFromDefinition(def.InfoMail)
	return
}

func buildMailConfigFromDefinition(def mailConfigDef) (*MailConfig, error) {
	d := &MailConfig{}
	d.From = def.From
	d.To = def.To
	d.Prefix = def.Prefix
	return d, nil
}

func buildConfigEnv(vars map[string]string) []string {
	ret := []string{}
	for k, v := range vars {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func loadPreCondition(cond []*conditionDef) []*Condition {
	ret := []*Condition{}
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

func parseSchedule(values []string) ([]*Schedule, error) {
	ret := []*Schedule{}
	for _, v := range values {
		paresed, err := cronParser.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule: %s", err)
		}
		ret = append(ret, &Schedule{
			Expression: v,
			Parsed:     paresed,
		})
	}
	return ret, nil
}

// only assert functions clause
func assertFunctions(funcs []*funcDef) error {
	if funcs == nil {
		return nil
	}

	nameMap := make(map[string]bool)
	for _, funcDef := range funcs {
		if _, exists := nameMap[funcDef.Name]; exists {
			return fmt.Errorf("duplicate function")
		}
		nameMap[funcDef.Name] = true

		definedParamNames := strings.Split(funcDef.Params, " ")
		passedParamNames := utils.ExtractParamNames(funcDef.Command)
		if len(definedParamNames) != len(passedParamNames) {
			return fmt.Errorf("func params and args given to func command do not match")
		}

		for i := 0; i < len(definedParamNames); i++ {
			if definedParamNames[i] != passedParamNames[i] {
				return fmt.Errorf("func params and args given to func command do not match")
			}
		}
	}

	return nil
}

func assertStepDef(def *stepDef, funcs []*funcDef) error {
	if def.Name == "" {
		return fmt.Errorf("step name must be specified")
	}
	// TODO: Refactor the validation check for each executor.
	if def.Executor == nil && (def.Command == "" && def.Call == nil) {
		return fmt.Errorf("either step command or step call must be specified if executor is nil")
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
			return fmt.Errorf("call must specify a functions that exists")
		}

		definedParamNames := strings.Split(calledFuncDef.Params, " ")
		if len(def.Call.Args) != len(definedParamNames) {
			return fmt.Errorf("the number of parameters defined in the function does not match the number of parameters given")
		}

		for _, paramName := range definedParamNames {
			_, exists := def.Call.Args[paramName]
			if !exists {
				return fmt.Errorf("required parameter not found")
			}
		}
	}

	return nil
}

func parseScheduleMap(scheduleMap map[interface{}]interface{}, starts, stops, restarts *[]string) error {
	for k, v := range scheduleMap {
		if _, ok := k.(string); !ok {
			return fmt.Errorf("schedule key must be a string")
		}
		switch k.(string) {
		case scheduleStart, scheduleStop, scheduleRestart:
			switch v := (v).(type) {
			case string:
				switch k {
				case scheduleStart:
					*starts = append(*starts, v)
				case scheduleStop:
					*stops = append(*stops, v)
				case scheduleRestart:
					*restarts = append(*restarts, v)
				}
			case []interface{}:
				for _, item := range v {
					if item, ok := item.(string); ok {
						switch k {
						case scheduleStart:
							*starts = append(*starts, item)
						case scheduleStop:
							*stops = append(*stops, item)
						case scheduleRestart:
							*restarts = append(*restarts, item)
						}
					} else {
						return fmt.Errorf("schedule must be a string or an array of strings")
					}
				}
			default:
				return fmt.Errorf("schedule must be a string or an array of strings")
			}
		default:
			return fmt.Errorf("schedule key must be start or stop")
		}
	}
	return nil
}
