package spec

import (
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
)

// step defines a step in the DAG.
type step struct {
	// Name is the name of the step.
	Name string `yaml:"name,omitempty"`
	// ID is the optional unique identifier for the step.
	ID string `yaml:"id,omitempty"`
	// Description is the description of the step.
	Description string `yaml:"description,omitempty"`
	// WorkingDir is the working directory of the step.
	WorkingDir string `yaml:"workingDir,omitempty"`
	// Dir is the working directory of the step.
	// Deprecated: use WorkingDir instead
	Dir string `yaml:"dir,omitempty"`
	// Executor is the executor configuration.
	Executor any `yaml:"executor,omitempty"`
	// Command is the command to run (on shell).
	Command any `yaml:"command,omitempty"`
	// Shell is the shell to run the command. Default is `$SHELL` or `sh`.
	// Can be a string (e.g., "bash -e") or an array (e.g., ["bash", "-e"]).
	Shell types.ShellValue `yaml:"shell,omitempty"`
	// ShellPackages is the list of packages to install.
	// This is used only when the shell is `nix-shell`.
	ShellPackages []string `yaml:"shellPackages,omitempty"`
	// Script is the script to run.
	Script string `yaml:"script,omitempty"`
	// Stdout is the file to write the stdout.
	Stdout string `yaml:"stdout,omitempty"`
	// Stderr is the file to write the stderr.
	Stderr string `yaml:"stderr,omitempty"`
	// Output is the variable name to store the output.
	Output string `yaml:"output,omitempty"`
	// Depends is the list of steps to depend on.
	Depends types.StringOrArray `yaml:"depends,omitempty"`
	// ContinueOn is the condition to continue on.
	// Can be a string ("skipped", "failed") or an object with detailed config.
	ContinueOn types.ContinueOnValue `yaml:"continueOn,omitempty"`
	// RetryPolicy is the retry policy.
	RetryPolicy *retryPolicy `yaml:"retryPolicy,omitempty"`
	// RepeatPolicy is the repeat policy.
	RepeatPolicy *repeatPolicy `yaml:"repeatPolicy,omitempty"`
	// MailOnError is the flag to send mail on error.
	MailOnError bool `yaml:"mailOnError,omitempty"`
	// Precondition is the condition to run the step.
	Precondition any `yaml:"precondition,omitempty"`
	// Preconditions is the condition to run the step.
	Preconditions any `yaml:"preconditions,omitempty"`
	// SignalOnStop is the signal when the step is requested to stop.
	// When it is empty, the same signal as the parent process is sent.
	// It can be KILL when the process does not stop over the timeout.
	SignalOnStop *string `yaml:"signalOnStop,omitempty"`
	// Call is the name of a DAG to run as a sub dag-run.
	Call string `yaml:"call,omitempty"`
	// Run is the name of a DAG to run as a sub dag-run.
	// Deprecated: use Call instead.
	Run string `yaml:"run,omitempty"`
	// Params specifies the parameters for the sub dag-run.
	Params any `yaml:"params,omitempty"`
	// Parallel specifies parallel execution configuration.
	// Can be:
	// - Direct array reference: parallel: ${ITEMS}
	// - Static array: parallel: [item1, item2]
	// - Object configuration: parallel: {items: ${ITEMS}, maxConcurrent: 5}
	Parallel any `yaml:"parallel,omitempty"`
	// WorkerSelector specifies required worker labels for execution.
	WorkerSelector map[string]string `yaml:"workerSelector,omitempty"`
	// Env specifies the environment variables for the step.
	Env types.EnvValue `yaml:"env,omitempty"`
	// TimeoutSec specifies the maximum runtime for the step in seconds.
	TimeoutSec int `yaml:"timeoutSec,omitempty"`
}

// repeatPolicy defines the repeat policy for a step.
type repeatPolicy struct {
	Repeat         any    `yaml:"repeat,omitempty"`         // Flag to indicate if the step should be repeated, can be bool (legacy) or string ("while" or "until")
	IntervalSec    int    `yaml:"intervalSec,omitempty"`    // Interval in seconds to wait before repeating the step
	Limit          int    `yaml:"limit,omitempty"`          // Maximum number of times to repeat the step
	Condition      string `yaml:"condition,omitempty"`      // Condition to check before repeating
	Expected       string `yaml:"expected,omitempty"`       // Expected output to match before repeating
	ExitCode       []int  `yaml:"exitCode,omitempty"`       // List of exit codes to consider for repeating the step
	Backoff        any    `yaml:"backoff,omitempty"`        // Accepts bool or float
	MaxIntervalSec int    `yaml:"maxIntervalSec,omitempty"` // Maximum interval in seconds
}

// retryPolicy defines the retry policy for a step.
type retryPolicy struct {
	Limit          any   `yaml:"limit,omitempty"`
	IntervalSec    any   `yaml:"intervalSec,omitempty"`
	ExitCode       []int `yaml:"exitCode,omitempty"`
	Backoff        any   `yaml:"backoff,omitempty"` // Accepts bool or float
	MaxIntervalSec int   `yaml:"maxIntervalSec,omitempty"`
}

// Build transforms the step specification into a core.Step.
// Simple field mappings are done inline; complex transformations call dedicated methods.
func (s *step) Build(ctx StepBuildContext) (*core.Step, error) {
	// Initialize with simple field mappings
	result := &core.Step{
		Name:           strings.TrimSpace(s.Name),
		ID:             strings.TrimSpace(s.ID),
		Description:    strings.TrimSpace(s.Description),
		ShellPackages:  s.ShellPackages,
		Script:         strings.TrimSpace(s.Script),
		Stdout:         strings.TrimSpace(s.Stdout),
		Stderr:         strings.TrimSpace(s.Stderr),
		MailOnError:    s.MailOnError,
		ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)},
	}

	// Complex transformations that may return errors
	var errs core.ErrorList
	transformers := []struct {
		name string
		fn   func(ctx StepBuildContext, result *core.Step) error
	}{
		{name: "workingDir", fn: s.buildWorkingDir},
		{name: "shell", fn: s.buildShell},
		{name: "executor", fn: s.buildExecutor},
		{name: "command", fn: s.buildCommand},
		{name: "params", fn: s.buildParams},
		{name: "timeout", fn: s.buildTimeout},
		{name: "depends", fn: s.buildDepends},
		{name: "parallel", fn: s.buildParallel}, // Must be before subDAG to set executor type correctly
		{name: "subDAG", fn: s.buildSubDAG},
		{name: "continueOn", fn: s.buildContinueOn},
		{name: "retryPolicy", fn: s.buildRetryPolicy},
		{name: "repeatPolicy", fn: s.buildRepeatPolicy},
		{name: "signalOnStop", fn: s.buildSignalOnStop},
		{name: "precondition", fn: s.buildPrecondition},
		{name: "output", fn: s.buildOutput},
		{name: "env", fn: s.buildEnvs},
	}

	for _, t := range transformers {
		if err := t.fn(ctx, result); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return result, nil
}

func (s *step) buildWorkingDir(_ StepBuildContext, result *core.Step) error {
	switch {
	case s.WorkingDir != "":
		result.Dir = strings.TrimSpace(s.WorkingDir)
	case s.Dir != "":
		result.Dir = strings.TrimSpace(s.Dir)
	default:
		result.Dir = ""
	}
	return nil
}

func (s *step) buildShell(_ StepBuildContext, result *core.Step) error {
	if s.Shell.IsZero() {
		return nil
	}

	if s.Shell.IsArray() {
		result.Shell = s.Shell.Command()
		result.ShellArgs = s.Shell.Arguments()
		return nil
	}

	// For string form, need to split command and args
	command := s.Shell.Command()
	if command == "" {
		return nil
	}

	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return core.NewValidationError("shell", s.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	result.Shell = strings.TrimSpace(shell)
	result.ShellArgs = args
	return nil
}

func (s *step) buildTimeout(_ StepBuildContext, result *core.Step) error {
	if s.TimeoutSec < 0 {
		return core.NewValidationError("timeoutSec", s.TimeoutSec, ErrTimeoutSecMustBeNonNegative)
	}
	if s.TimeoutSec == 0 {
		return nil
	}
	result.Timeout = time.Second * time.Duration(s.TimeoutSec)
	return nil
}

func (s *step) buildDepends(_ StepBuildContext, result *core.Step) error {
	result.Depends = s.Depends.Values()

	// Check if depends was explicitly set to empty array
	if !s.Depends.IsZero() && s.Depends.IsEmpty() {
		result.ExplicitlyNoDeps = true
	}

	return nil
}

func (s *step) buildContinueOn(_ StepBuildContext, result *core.Step) error {
	if s.ContinueOn.IsZero() {
		return nil
	}

	result.ContinueOn.Skipped = s.ContinueOn.Skipped()
	result.ContinueOn.Failure = s.ContinueOn.Failed()
	result.ContinueOn.MarkSuccess = s.ContinueOn.MarkSuccess()
	result.ContinueOn.ExitCode = s.ContinueOn.ExitCode()
	result.ContinueOn.Output = s.ContinueOn.Output()

	return nil
}

func (s *step) buildRetryPolicy(_ StepBuildContext, result *core.Step) error {
	if s.RetryPolicy == nil {
		return nil
	}

	switch v := s.RetryPolicy.Limit.(type) {
	case int:
		result.RetryPolicy.Limit = v
	case int64:
		result.RetryPolicy.Limit = int(v)
	case uint64:
		result.RetryPolicy.Limit = int(v)
	case string:
		result.RetryPolicy.LimitStr = v
	case nil:
		// No limit specified
	default:
		return core.NewValidationError("retryPolicy.Limit", v, fmt.Errorf("invalid type: %T", v))
	}

	switch v := s.RetryPolicy.IntervalSec.(type) {
	case int:
		result.RetryPolicy.Interval = time.Second * time.Duration(v)
	case int64:
		result.RetryPolicy.Interval = time.Second * time.Duration(v)
	case uint64:
		result.RetryPolicy.Interval = time.Second * time.Duration(v)
	case string:
		result.RetryPolicy.IntervalSecStr = v
	case nil:
		// No interval specified
	default:
		return core.NewValidationError("retryPolicy.IntervalSec", v, fmt.Errorf("invalid type: %T", v))
	}

	if s.RetryPolicy.ExitCode != nil {
		result.RetryPolicy.ExitCodes = s.RetryPolicy.ExitCode
	}

	// Parse backoff field
	if s.RetryPolicy.Backoff != nil {
		switch v := s.RetryPolicy.Backoff.(type) {
		case bool:
			if v {
				result.RetryPolicy.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			result.RetryPolicy.Backoff = float64(v)
		case int64:
			result.RetryPolicy.Backoff = float64(v)
		case float64:
			result.RetryPolicy.Backoff = v
		default:
			return core.NewValidationError("retryPolicy.Backoff", v, fmt.Errorf("invalid type: %T", v))
		}

		// Validate backoff value
		if result.RetryPolicy.Backoff > 0 && result.RetryPolicy.Backoff <= 1.0 {
			return core.NewValidationError("retryPolicy.Backoff", result.RetryPolicy.Backoff,
				fmt.Errorf("backoff must be greater than 1.0 for exponential growth"))
		}
	}

	// Parse maxIntervalSec
	if s.RetryPolicy.MaxIntervalSec > 0 {
		result.RetryPolicy.MaxInterval = time.Second * time.Duration(s.RetryPolicy.MaxIntervalSec)
	}

	return nil
}

func (s *step) buildRepeatPolicy(_ StepBuildContext, result *core.Step) error {
	if s.RepeatPolicy == nil {
		return nil
	}
	rpDef := s.RepeatPolicy

	// Determine repeat mode
	var mode core.RepeatMode
	if rpDef.Repeat != nil {
		switch v := rpDef.Repeat.(type) {
		case bool:
			if v {
				mode = core.RepeatModeWhile
			}
		case string:
			switch v {
			case "while":
				mode = core.RepeatModeWhile
			case "until":
				mode = core.RepeatModeUntil
			default:
				return fmt.Errorf("invalid value for repeat: '%s'. It must be 'while', 'until', or a boolean", v)
			}
		default:
			return fmt.Errorf("invalid value for repeat: '%v'. It must be 'while', 'until', or a boolean", v)
		}
	}

	// Backward compatibility: infer mode if not set
	if mode == "" {
		if rpDef.Condition != "" && rpDef.Expected != "" {
			mode = core.RepeatModeUntil
		} else if rpDef.Condition != "" || len(rpDef.ExitCode) > 0 {
			mode = core.RepeatModeWhile
		}
	}

	// No repeat if mode is not determined
	if mode == "" {
		return nil
	}

	// Validate that explicit while/until modes have appropriate conditions
	if rpDef.Repeat != nil {
		switch v := rpDef.Repeat.(type) {
		case string:
			if (v == "while" || v == "until") && rpDef.Condition == "" && len(rpDef.ExitCode) == 0 {
				return fmt.Errorf("repeat mode '%s' requires either 'condition' or 'exitCode' to be specified", v)
			}
		}
	}

	result.RepeatPolicy.RepeatMode = mode
	if rpDef.IntervalSec > 0 {
		result.RepeatPolicy.Interval = time.Second * time.Duration(rpDef.IntervalSec)
	}
	result.RepeatPolicy.Limit = rpDef.Limit

	if rpDef.Condition != "" {
		result.RepeatPolicy.Condition = &core.Condition{
			Condition: rpDef.Condition,
			Expected:  rpDef.Expected,
		}
	}
	result.RepeatPolicy.ExitCode = rpDef.ExitCode

	// Parse backoff field
	if rpDef.Backoff != nil {
		switch v := rpDef.Backoff.(type) {
		case bool:
			if v {
				result.RepeatPolicy.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			result.RepeatPolicy.Backoff = float64(v)
		case int64:
			result.RepeatPolicy.Backoff = float64(v)
		case float64:
			result.RepeatPolicy.Backoff = v
		default:
			return fmt.Errorf("invalid value for backoff: '%v'. It must be a boolean or number", v)
		}

		// Validate backoff value
		if result.RepeatPolicy.Backoff > 0 && result.RepeatPolicy.Backoff <= 1.0 {
			return fmt.Errorf("backoff must be greater than 1.0 for exponential growth, got: %v",
				result.RepeatPolicy.Backoff)
		}
	}

	// Parse maxIntervalSec
	if rpDef.MaxIntervalSec > 0 {
		result.RepeatPolicy.MaxInterval = time.Second * time.Duration(rpDef.MaxIntervalSec)
	}

	return nil
}

func (s *step) buildSignalOnStop(_ StepBuildContext, result *core.Step) error {
	if s.SignalOnStop != nil {
		sigDef := *s.SignalOnStop
		sig := signal.GetSignalNum(sigDef, 0)
		if sig == 0 {
			return fmt.Errorf("%w: %s", ErrInvalidSignal, sigDef)
		}
		result.SignalOnStop = sigDef
	}
	return nil
}

func (s *step) buildOutput(_ StepBuildContext, result *core.Step) error {
	if s.Output == "" {
		return nil
	}

	if strings.HasPrefix(s.Output, "$") {
		result.Output = strings.TrimPrefix(s.Output, "$")
		return nil
	}

	result.Output = strings.TrimSpace(s.Output)
	return nil
}

func (s *step) buildEnvs(_ StepBuildContext, result *core.Step) error {
	if s.Env.IsZero() {
		return nil
	}
	for _, entry := range s.Env.Entries() {
		result.Env = append(result.Env, fmt.Sprintf("%s=%s", entry.Key, entry.Value))
	}
	return nil
}

func (s *step) buildPrecondition(ctx StepBuildContext, result *core.Step) error {
	// Parse both `preconditions` and `precondition` fields.
	conditions, err := parsePrecondition(ctx.BuildContext, s.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx.BuildContext, s.Precondition)
	if err != nil {
		return err
	}
	result.Preconditions = conditions
	result.Preconditions = append(result.Preconditions, condition...)
	return nil
}

// buildCommand parses the command field in the step definition.
func (s *step) buildCommand(_ StepBuildContext, result *core.Step) error {
	command := s.Command

	// Case 1: command is nil
	if command == nil {
		return nil
	}

	switch val := command.(type) {
	case string:
		// Case 2: command is a string
		val = strings.TrimSpace(val)
		if val == "" {
			return core.NewValidationError("command", val, ErrStepCommandIsEmpty)
		}

		// If the value is multi-line, treat it as a script
		if strings.Contains(val, "\n") {
			result.Script = val
			return nil
		}

		// We need to split the command into command and args.
		result.CmdWithArgs = val
		cmd, args, err := cmdutil.SplitCommand(val)
		if err != nil {
			return core.NewValidationError("command", val, fmt.Errorf("failed to parse command: %w", err))
		}
		result.Command = strings.TrimSpace(cmd)
		result.Args = args

	case []any:
		// Case 3: command is an array
		var cmd string
		var args []string
		for _, v := range val {
			strVal, ok := v.(string)
			if !ok {
				// If the value is not a string, convert it to a string.
				strVal = fmt.Sprintf("%v", v)
			}
			strVal = strings.TrimSpace(strVal)
			if cmd == "" {
				cmd = strVal
				continue
			}
			args = append(args, strVal)
		}

		// Setup CmdWithArgs
		var sb strings.Builder
		for i, arg := range args {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(fmt.Sprintf("%q", arg))
		}

		result.Command = cmd
		result.Args = args
		result.CmdWithArgs = fmt.Sprintf("%s %s", result.Command, sb.String())
		result.CmdArgsSys = cmdutil.JoinCommandArgs(result.Command, result.Args)

	default:
		return core.NewValidationError("command", val, ErrStepCommandMustBeArrayOrString)
	}

	return nil
}

func (s *step) buildParams(ctx StepBuildContext, result *core.Step) error {
	if s.Params == nil {
		return nil
	}

	// Parse params using existing parseParamValue function
	paramPairs, err := parseParamValue(ctx.BuildContext, s.Params)
	if err != nil {
		return core.NewValidationError("params", s.Params, err)
	}

	// Convert to map[string]string
	paramsData := make(map[string]string)
	for _, pair := range paramPairs {
		paramsData[pair.Name] = pair.Value
	}

	result.Params = core.NewSimpleParams(paramsData)
	return nil
}

// buildExecutor parses the executor field in the step definition.
func (s *step) buildExecutor(ctx StepBuildContext, result *core.Step) error {
	const (
		executorKeyType   = "type"
		executorKeyConfig = "config"
	)

	executor := s.Executor

	// Case 1: executor is nil
	if executor == nil {
		if ctx.dag != nil && ctx.dag.Container != nil {
			// Translate the container configuration to executor config
			result.ExecutorConfig.Type = "container"
			return nil
		} else if ctx.dag != nil && ctx.dag.SSH != nil {
			result.ExecutorConfig.Type = "ssh"
			return nil
		}
		return nil
	}

	switch val := executor.(type) {
	case string:
		// Case 2: executor is a string
		result.ExecutorConfig.Type = strings.TrimSpace(val)

	case map[string]any:
		// Case 3: executor is a struct
		for key, v := range val {
			switch key {
			case executorKeyType:
				typ, ok := v.(string)
				if !ok {
					return core.NewValidationError("executor.type", v, ErrExecutorTypeMustBeString)
				}
				result.ExecutorConfig.Type = strings.TrimSpace(typ)

			case executorKeyConfig:
				executorConfig, ok := v.(map[string]any)
				if !ok {
					return core.NewValidationError("executor.config", v, ErrExecutorConfigValueMustBeMap)
				}
				for configKey, cv := range executorConfig {
					result.ExecutorConfig.Config[configKey] = cv
				}

			default:
				return core.NewValidationError("executor.config", key, fmt.Errorf("%w: %s", ErrExecutorHasInvalidKey, key))
			}
		}

	default:
		return core.NewValidationError("executor", val, ErrExecutorConfigMustBeStringOrMap)
	}

	return nil
}

// buildParallel parses the parallel field in the step definition.
func (s *step) buildParallel(_ StepBuildContext, result *core.Step) error {
	if s.Parallel == nil {
		return nil
	}

	result.Parallel = &core.ParallelConfig{
		MaxConcurrent: core.DefaultMaxConcurrent,
	}

	switch v := s.Parallel.(type) {
	case string:
		// Direct variable reference like: parallel: ${ITEMS}
		result.Parallel.Variable = v

	case []any:
		// Static array: parallel: [item1, item2]
		items, err := parseParallelItems(v)
		if err != nil {
			return core.NewValidationError("parallel", v, err)
		}
		result.Parallel.Items = items

	case map[string]any:
		// Object configuration
		for key, val := range v {
			switch key {
			case "items":
				switch itemsVal := val.(type) {
				case string:
					result.Parallel.Variable = itemsVal
				case []any:
					items, err := parseParallelItems(itemsVal)
					if err != nil {
						return core.NewValidationError("parallel.items", itemsVal, err)
					}
					result.Parallel.Items = items
				default:
					return core.NewValidationError("parallel.items", val, fmt.Errorf("parallel.items must be string or array, got %T", val))
				}

			case "maxConcurrent":
				switch mc := val.(type) {
				case int:
					result.Parallel.MaxConcurrent = mc
				case int64:
					result.Parallel.MaxConcurrent = int(mc)
				case uint64:
					result.Parallel.MaxConcurrent = int(mc)
				case float64:
					result.Parallel.MaxConcurrent = int(mc)
				default:
					return core.NewValidationError("parallel.maxConcurrent", val, fmt.Errorf("parallel.maxConcurrent must be int, got %T", val))
				}
			}
		}

	default:
		return core.NewValidationError("parallel", v, fmt.Errorf("parallel must be string, array, or object, got %T", v))
	}

	return nil
}

// parseParallelItems converts an array of any type to core.ParallelItem slice
func parseParallelItems(items []any) ([]core.ParallelItem, error) {
	var result []core.ParallelItem

	for _, item := range items {
		switch v := item.(type) {
		case string:
			result = append(result, core.ParallelItem{Value: v})

		case int, int64, uint64, float64:
			result = append(result, core.ParallelItem{Value: fmt.Sprintf("%v", v)})

		case map[string]any:
			params := make(collections.DeterministicMap)
			for key, val := range v {
				var strVal string
				switch pv := val.(type) {
				case string:
					strVal = pv
				case int:
					strVal = fmt.Sprintf("%d", pv)
				case int64:
					strVal = fmt.Sprintf("%d", pv)
				case uint64:
					strVal = fmt.Sprintf("%d", pv)
				case float64:
					strVal = fmt.Sprintf("%g", pv)
				case bool:
					strVal = fmt.Sprintf("%t", pv)
				default:
					return nil, fmt.Errorf("parameter values must be strings, numbers, or booleans, got %T for key %s", val, key)
				}
				params[key] = strVal
			}
			result = append(result, core.ParallelItem{Params: params})

		default:
			return nil, fmt.Errorf("parallel items must be strings, numbers, or objects, got %T", v)
		}
	}

	return result, nil
}

// buildSubDAG parses the child core.DAG definition and sets up the step to run a sub DAG.
func (s *step) buildSubDAG(ctx StepBuildContext, result *core.Step) error {
	name := strings.TrimSpace(s.Call)
	if name == "" {
		// TODO: remove legacy support in future major version
		if legacyName := strings.TrimSpace(s.Run); legacyName != "" {
			name = legacyName
			message := "Step field 'run' is deprecated, use 'call' instead"
			logger.Warn(ctx.ctx, message)
			if ctx.dag != nil {
				ctx.dag.BuildWarnings = append(ctx.dag.BuildWarnings, message)
			}
		}
	}

	// if the run field is not set, return nil.
	if name == "" {
		return nil
	}

	// Parse params similar to how core.DAG params are parsed
	var paramsStr string
	if s.Params != nil {
		// Parse the params to convert them to string format
		ctxCopy := ctx
		ctxCopy.opts.Flags |= BuildFlagNoEval // Disable evaluation for params parsing
		paramPairs, err := parseParamValue(ctxCopy.BuildContext, s.Params)
		if err != nil {
			return core.NewValidationError("params", s.Params, err)
		}

		// Convert to string format "key=value key=value ..."
		var paramsToJoin []string
		for _, paramPair := range paramPairs {
			paramsToJoin = append(paramsToJoin, paramPair.Escaped())
		}
		paramsStr = strings.Join(paramsToJoin, " ")
	}

	result.SubDAG = &core.SubDAG{Name: name, Params: paramsStr}

	// Set executor type based on whether parallel execution is configured
	if result.Parallel != nil {
		result.ExecutorConfig.Type = core.ExecutorTypeParallel
	} else {
		result.ExecutorConfig.Type = core.ExecutorTypeDAG
	}

	result.Command = "call"
	result.Args = []string{name, paramsStr}
	result.CmdWithArgs = strings.TrimSpace(fmt.Sprintf("%s %s", name, paramsStr))
	return nil
}
