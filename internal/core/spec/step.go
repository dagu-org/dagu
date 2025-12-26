package spec

import (
	"fmt"
	"math"
	"reflect"
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
	// Container specifies the container configuration for this step.
	// If set, the step runs in its own container instead of the DAG-level container.
	// This uses the same configuration format as the DAG-level container field.
	Container *container `yaml:"container,omitempty"`
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

// stepTransformer is a generic implementation for step field transformations
type stepTransformer[T any] struct {
	fieldName string
	builder   func(ctx StepBuildContext, s *step) (T, error)
}

func (t *stepTransformer[T]) Transform(ctx StepBuildContext, in *step, out reflect.Value) error {
	v, err := t.builder(ctx, in)
	if err != nil {
		return err
	}
	field := out.FieldByName(t.fieldName)
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.ValueOf(v))
	}
	return nil
}

// newStepTransformer creates a step transformer for a single field
func newStepTransformer[T any](fieldName string, builder func(StepBuildContext, *step) (T, error)) Transformer[StepBuildContext, *step] {
	return &stepTransformer[T]{
		fieldName: fieldName,
		builder:   builder,
	}
}

// stepTransform wraps a step transformer with its name for error reporting
type stepTransform struct {
	name        string
	transformer Transformer[StepBuildContext, *step]
}

// stepTransformers defines the ordered sequence of step transformers
var stepTransformers = []stepTransform{
	{"name", newStepTransformer("Name", buildStepName)},
	{"id", newStepTransformer("ID", buildStepID)},
	{"description", newStepTransformer("Description", buildStepDescription)},
	{"shellPackages", newStepTransformer("ShellPackages", buildStepShellPackages)},
	{"script", newStepTransformer("Script", buildStepScript)},
	{"stdout", newStepTransformer("Stdout", buildStepStdout)},
	{"stderr", newStepTransformer("Stderr", buildStepStderr)},
	{"mailOnError", newStepTransformer("MailOnError", buildStepMailOnError)},
	{"workerSelector", newStepTransformer("WorkerSelector", buildStepWorkerSelector)},
	{"workingDir", newStepTransformer("Dir", buildStepWorkingDir)},
	{"shell", newStepTransformer("Shell", buildStepShell)},
	{"shellArgs", newStepTransformer("ShellArgs", buildStepShellArgs)},
	{"timeout", newStepTransformer("Timeout", buildStepTimeout)},
	{"depends", newStepTransformer("Depends", buildStepDepends)},
	{"explicitlyNoDeps", newStepTransformer("ExplicitlyNoDeps", buildStepExplicitlyNoDeps)},
	{"continueOn", newStepTransformer("ContinueOn", buildStepContinueOn)},
	{"retryPolicy", newStepTransformer("RetryPolicy", buildStepRetryPolicy)},
	{"repeatPolicy", newStepTransformer("RepeatPolicy", buildStepRepeatPolicy)},
	{"signalOnStop", newStepTransformer("SignalOnStop", buildStepSignalOnStop)},
	{"output", newStepTransformer("Output", buildStepOutput)},
	{"env", newStepTransformer("Env", buildStepEnvs)},
	{"preconditions", newStepTransformer("Preconditions", buildStepPreconditions)},
}

// runStepTransformers executes all step transformers
func runStepTransformers(ctx StepBuildContext, spec *step, result *core.Step) core.ErrorList {
	var errs core.ErrorList
	out := reflect.ValueOf(result).Elem()

	for _, t := range stepTransformers {
		if err := t.transformer.Transform(ctx, spec, out); err != nil {
			errs = append(errs, wrapTransformError(t.name, err))
		}
	}

	return errs
}

// build transforms the step specification into a core.Step.
func (s *step) build(ctx StepBuildContext) (*core.Step, error) {
	result := &core.Step{
		ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)},
	}

	// Run the transformer pipeline
	errs := runStepTransformers(ctx, s, result)

	// Complex transformations that need access to result or set multiple fields
	// Note: buildStepContainer must be called before buildStepExecutor because
	// buildStepExecutor checks result.Container to determine if the step should
	// use the docker executor.
	if err := buildStepContainer(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("container", err))
	}
	if err := buildStepExecutor(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("executor", err))
	}
	if err := buildStepCommand(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("command", err))
	}
	if err := validateMultipleCommands(result); err != nil {
		errs = append(errs, wrapTransformError("command", err))
	}
	if err := validateScript(result); err != nil {
		errs = append(errs, wrapTransformError("script", err))
	}
	if err := validateShell(result); err != nil {
		errs = append(errs, wrapTransformError("shell", err))
	}
	if err := validateContainer(result); err != nil {
		errs = append(errs, wrapTransformError("container", err))
	}
	if err := validateSubDAG(result); err != nil {
		errs = append(errs, wrapTransformError("call", err))
	}
	if err := buildStepParamsField(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("params", err))
	}
	if err := buildStepParallel(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("parallel", err))
	}
	if err := buildStepSubDAG(ctx, s, result); err != nil {
		errs = append(errs, wrapTransformError("subDAG", err))
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return result, nil
}

// Simple field builders

func buildStepName(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.Name), nil
}

func buildStepID(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.ID), nil
}

func buildStepDescription(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.Description), nil
}

func buildStepShellPackages(_ StepBuildContext, s *step) ([]string, error) {
	return s.ShellPackages, nil
}

func buildStepScript(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.Script), nil
}

func buildStepStdout(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.Stdout), nil
}

func buildStepStderr(_ StepBuildContext, s *step) (string, error) {
	return strings.TrimSpace(s.Stderr), nil
}

func buildStepMailOnError(_ StepBuildContext, s *step) (bool, error) {
	return s.MailOnError, nil
}

func buildStepWorkerSelector(_ StepBuildContext, s *step) (map[string]string, error) {
	return s.WorkerSelector, nil
}

func buildStepWorkingDir(_ StepBuildContext, s *step) (string, error) {
	switch {
	case s.WorkingDir != "":
		return strings.TrimSpace(s.WorkingDir), nil
	case s.Dir != "":
		return strings.TrimSpace(s.Dir), nil
	default:
		return "", nil
	}
}

// stepShellResult holds both shell and args for step
type stepShellResult struct {
	Shell string
	Args  []string
}

func parseStepShellInternal(_ StepBuildContext, s *step) (*stepShellResult, error) {
	if s.Shell.IsZero() {
		return &stepShellResult{}, nil
	}

	if s.Shell.IsArray() {
		return &stepShellResult{
			Shell: s.Shell.Command(),
			Args:  s.Shell.Arguments(),
		}, nil
	}

	// For string form, need to split command and args
	command := s.Shell.Command()
	if command == "" {
		return &stepShellResult{}, nil
	}

	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return nil, core.NewValidationError("shell", s.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	return &stepShellResult{
		Shell: strings.TrimSpace(shell),
		Args:  args,
	}, nil
}

func buildStepShell(ctx StepBuildContext, s *step) (string, error) {
	result, err := parseStepShellInternal(ctx, s)
	if err != nil {
		return "", err
	}
	return result.Shell, nil
}

func buildStepShellArgs(ctx StepBuildContext, s *step) ([]string, error) {
	result, err := parseStepShellInternal(ctx, s)
	if err != nil {
		return nil, err
	}
	return result.Args, nil
}

func buildStepTimeout(_ StepBuildContext, s *step) (time.Duration, error) {
	if s.TimeoutSec < 0 {
		return 0, core.NewValidationError("timeoutSec", s.TimeoutSec, ErrTimeoutSecMustBeNonNegative)
	}
	return time.Second * time.Duration(s.TimeoutSec), nil
}

func buildStepDepends(_ StepBuildContext, s *step) ([]string, error) {
	return s.Depends.Values(), nil
}

func buildStepExplicitlyNoDeps(_ StepBuildContext, s *step) (bool, error) {
	return !s.Depends.IsZero() && s.Depends.IsEmpty(), nil
}

func buildStepContinueOn(_ StepBuildContext, s *step) (core.ContinueOn, error) {
	if s.ContinueOn.IsZero() {
		return core.ContinueOn{}, nil
	}

	return core.ContinueOn{
		Skipped:     s.ContinueOn.Skipped(),
		Failure:     s.ContinueOn.Failed(),
		MarkSuccess: s.ContinueOn.MarkSuccess(),
		ExitCode:    s.ContinueOn.ExitCode(),
		Output:      s.ContinueOn.Output(),
	}, nil
}

func buildStepRetryPolicy(_ StepBuildContext, s *step) (core.RetryPolicy, error) {
	if s.RetryPolicy == nil {
		return core.RetryPolicy{}, nil
	}

	var result core.RetryPolicy

	switch v := s.RetryPolicy.Limit.(type) {
	case int:
		result.Limit = v
	case int64:
		result.Limit = int(v)
	case uint64:
		if v > math.MaxInt {
			return core.RetryPolicy{}, core.NewValidationError("retryPolicy.limit", v, fmt.Errorf("value %d exceeds maximum int", v))
		}
		result.Limit = int(v)
	case string:
		result.LimitStr = v
	case nil:
		return core.RetryPolicy{}, core.NewValidationError("retryPolicy.limit", nil, fmt.Errorf("limit is required when retryPolicy is specified"))
	default:
		return core.RetryPolicy{}, core.NewValidationError("retryPolicy.limit", v, fmt.Errorf("invalid type: %T", v))
	}

	switch v := s.RetryPolicy.IntervalSec.(type) {
	case int:
		result.Interval = time.Second * time.Duration(v)
	case int64:
		result.Interval = time.Second * time.Duration(v)
	case uint64:
		if v > math.MaxInt64 {
			return core.RetryPolicy{}, core.NewValidationError("retryPolicy.intervalSec", v, fmt.Errorf("value %d exceeds maximum int64", v))
		}
		result.Interval = time.Second * time.Duration(v)
	case string:
		result.IntervalSecStr = v
	case nil:
		return core.RetryPolicy{}, core.NewValidationError("retryPolicy.intervalSec", nil, fmt.Errorf("intervalSec is required when retryPolicy is specified"))
	default:
		return core.RetryPolicy{}, core.NewValidationError("retryPolicy.intervalSec", v, fmt.Errorf("invalid type: %T", v))
	}

	if s.RetryPolicy.ExitCode != nil {
		result.ExitCodes = s.RetryPolicy.ExitCode
	}

	// Parse backoff field
	if s.RetryPolicy.Backoff != nil {
		switch v := s.RetryPolicy.Backoff.(type) {
		case bool:
			if v {
				result.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			result.Backoff = float64(v)
		case int64:
			result.Backoff = float64(v)
		case float64:
			result.Backoff = v
		default:
			return core.RetryPolicy{}, core.NewValidationError("retryPolicy.Backoff", v, fmt.Errorf("invalid type: %T", v))
		}

		// Validate backoff value
		if result.Backoff > 0 && result.Backoff <= 1.0 {
			return core.RetryPolicy{}, core.NewValidationError("retryPolicy.Backoff", result.Backoff,
				fmt.Errorf("backoff must be greater than 1.0 for exponential growth"))
		}
	}

	// Parse maxIntervalSec
	if s.RetryPolicy.MaxIntervalSec > 0 {
		result.MaxInterval = time.Second * time.Duration(s.RetryPolicy.MaxIntervalSec)
	}

	return result, nil
}

func buildStepRepeatPolicy(_ StepBuildContext, s *step) (core.RepeatPolicy, error) {
	if s.RepeatPolicy == nil {
		return core.RepeatPolicy{}, nil
	}
	rp := s.RepeatPolicy

	// Determine repeat mode
	var mode core.RepeatMode
	if rp.Repeat != nil {
		switch v := rp.Repeat.(type) {
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
				return core.RepeatPolicy{}, fmt.Errorf("invalid value for repeat: '%s'. It must be 'while', 'until', or a boolean", v)
			}
		default:
			return core.RepeatPolicy{}, fmt.Errorf("invalid value for repeat: '%v'. It must be 'while', 'until', or a boolean", v)
		}
	}

	// Backward compatibility: infer mode if not set
	if mode == "" {
		if rp.Condition != "" && rp.Expected != "" {
			mode = core.RepeatModeUntil
		} else if rp.Condition != "" || len(rp.ExitCode) > 0 {
			mode = core.RepeatModeWhile
		}
	}

	// No repeat if mode is not determined
	if mode == "" {
		return core.RepeatPolicy{}, nil
	}

	// Validate that explicit while/until modes have appropriate conditions
	if rp.Repeat != nil {
		switch v := rp.Repeat.(type) {
		case string:
			if (v == "while" || v == "until") && rp.Condition == "" && len(rp.ExitCode) == 0 {
				return core.RepeatPolicy{}, fmt.Errorf("repeat mode '%s' requires either 'condition' or 'exitCode' to be specified", v)
			}
		}
	}

	var result core.RepeatPolicy
	result.RepeatMode = mode
	if rp.IntervalSec > 0 {
		result.Interval = time.Second * time.Duration(rp.IntervalSec)
	}
	result.Limit = rp.Limit

	if rp.Condition != "" {
		result.Condition = &core.Condition{
			Condition: rp.Condition,
			Expected:  rp.Expected,
		}
	}
	result.ExitCode = rp.ExitCode

	// Parse backoff field
	if rp.Backoff != nil {
		switch v := rp.Backoff.(type) {
		case bool:
			if v {
				result.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			result.Backoff = float64(v)
		case int64:
			result.Backoff = float64(v)
		case float64:
			result.Backoff = v
		default:
			return core.RepeatPolicy{}, fmt.Errorf("invalid value for backoff: '%v'. It must be a boolean or number", v)
		}

		// Validate backoff value
		if result.Backoff > 0 && result.Backoff <= 1.0 {
			return core.RepeatPolicy{}, fmt.Errorf("backoff must be greater than 1.0 for exponential growth, got: %v",
				result.Backoff)
		}
	}

	// Parse maxIntervalSec
	if rp.MaxIntervalSec > 0 {
		result.MaxInterval = time.Second * time.Duration(rp.MaxIntervalSec)
	}

	return result, nil
}

func buildStepSignalOnStop(_ StepBuildContext, s *step) (string, error) {
	if s.SignalOnStop == nil {
		return "", nil
	}
	sigOnStop := *s.SignalOnStop
	sig := signal.GetSignalNum(sigOnStop, 0)
	if sig == 0 {
		return "", fmt.Errorf("%w: %s", ErrInvalidSignal, sigOnStop)
	}
	return sigOnStop, nil
}

func buildStepOutput(_ StepBuildContext, s *step) (string, error) {
	if s.Output == "" {
		return "", nil
	}

	if strings.HasPrefix(s.Output, "$") {
		return strings.TrimPrefix(s.Output, "$"), nil
	}

	return strings.TrimSpace(s.Output), nil
}

func buildStepEnvs(_ StepBuildContext, s *step) ([]string, error) {
	if s.Env.IsZero() {
		return nil, nil
	}
	var envs []string
	for _, entry := range s.Env.Entries() {
		envs = append(envs, fmt.Sprintf("%s=%s", entry.Key, entry.Value))
	}
	return envs, nil
}

func buildStepPreconditions(ctx StepBuildContext, s *step) ([]*core.Condition, error) {
	conditions, err := parsePrecondition(ctx.BuildContext, s.Preconditions)
	if err != nil {
		return nil, err
	}
	condition, err := parsePrecondition(ctx.BuildContext, s.Precondition)
	if err != nil {
		return nil, err
	}
	return append(conditions, condition...), nil
}

// buildStepCommand parses the command field in the step definition.
func buildStepCommand(_ StepBuildContext, s *step, result *core.Step) error {
	command := s.Command

	// Case 1: command is nil
	if command == nil {
		return nil
	}

	switch val := command.(type) {
	case string:
		// Case 2: command is a string (single command)
		return buildSingleCommand(val, result)

	case []any:
		// Case 3: command is an array (multiple commands)
		return buildMultipleCommands(val, result)

	default:
		return core.NewValidationError("command", val, ErrStepCommandMustBeArrayOrString)
	}
}

// buildSingleCommand parses a single command string and populates the Step fields.
func buildSingleCommand(val string, result *core.Step) error {
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
	cmd, args, err := cmdutil.SplitCommand(val)
	if err != nil {
		return core.NewValidationError("command", val, fmt.Errorf("failed to parse command: %w", err))
	}

	cmd = strings.TrimSpace(cmd)

	result.Commands = []core.CommandEntry{
		{
			Command:     cmd,
			Args:        args,
			CmdWithArgs: val,
		},
	}

	return nil
}

// buildMultipleCommands parses an array of commands and populates the Step.Commands field.
// Each array element is treated as a separate command to be executed sequentially.
func buildMultipleCommands(val []any, result *core.Step) error {
	if len(val) == 0 {
		return core.NewValidationError("command", val, ErrStepCommandIsEmpty)
	}

	var commands []core.CommandEntry

	for i, v := range val {
		var strVal string
		switch tv := v.(type) {
		case string:
			strVal = tv
		case int, int64, uint64, float64, bool:
			strVal = fmt.Sprintf("%v", tv)
		default:
			return core.NewValidationError(
				fmt.Sprintf("command[%d]", i),
				v,
				fmt.Errorf("command array elements must be strings or primitive types, got %T", v),
			)
		}
		strVal = strings.TrimSpace(strVal)

		if strVal == "" {
			continue // Skip empty commands
		}

		// Parse the command string to extract command and args
		cmd, args, err := cmdutil.SplitCommand(strVal)
		if err != nil {
			return core.NewValidationError(
				fmt.Sprintf("command[%d]", i),
				strVal,
				fmt.Errorf("failed to parse command: %w", err),
			)
		}

		commands = append(commands, core.CommandEntry{
			Command:     strings.TrimSpace(cmd),
			Args:        args,
			CmdWithArgs: strVal,
		})
	}

	if len(commands) == 0 {
		return core.NewValidationError("command", val, ErrStepCommandIsEmpty)
	}

	result.Commands = commands

	return nil
}

// validateMultipleCommands checks if the executor type supports multiple commands.
// Returns an error if multiple commands are specified for an executor that doesn't support them.
func validateMultipleCommands(result *core.Step) error {
	if len(result.Commands) <= 1 {
		return nil
	}
	if !core.SupportsMultipleCommands(result.ExecutorConfig.Type) {
		return core.NewValidationError(
			"command",
			result.Commands,
			fmt.Errorf("%w: executor type %q only supports a single command", ErrExecutorDoesNotSupportMultipleCmd, result.ExecutorConfig.Type),
		)
	}
	return nil
}

// validateScript checks if the executor type supports the script field.
func validateScript(result *core.Step) error {
	if result.Script == "" {
		return nil
	}
	if !core.SupportsScript(result.ExecutorConfig.Type) {
		return core.NewValidationError(
			"script",
			result.Script,
			fmt.Errorf("executor type %q does not support script field", result.ExecutorConfig.Type),
		)
	}
	return nil
}

// validateShell checks if the executor type supports shell configuration.
func validateShell(result *core.Step) error {
	if result.Shell == "" && len(result.ShellArgs) == 0 && len(result.ShellPackages) == 0 {
		return nil
	}
	if !core.SupportsShell(result.ExecutorConfig.Type) {
		return core.NewValidationError(
			"shell",
			result.Shell,
			fmt.Errorf("executor type %q does not support shell configuration", result.ExecutorConfig.Type),
		)
	}
	return nil
}

// validateContainer checks if the executor type supports the container field.
func validateContainer(result *core.Step) error {
	if result.Container == nil {
		return nil
	}
	if !core.SupportsContainer(result.ExecutorConfig.Type) {
		return core.NewValidationError(
			"container",
			result.Container,
			fmt.Errorf("executor type %q does not support container field", result.ExecutorConfig.Type),
		)
	}
	return nil
}

// validateSubDAG checks if the executor type supports sub-DAG execution.
func validateSubDAG(result *core.Step) error {
	if result.SubDAG == nil {
		return nil
	}
	if !core.SupportsSubDAG(result.ExecutorConfig.Type) {
		return core.NewValidationError(
			"call",
			result.SubDAG,
			fmt.Errorf("executor type %q does not support sub-DAG execution", result.ExecutorConfig.Type),
		)
	}
	return nil
}

func buildStepParamsField(ctx StepBuildContext, s *step, result *core.Step) error {
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

// buildStepExecutor parses the executor field in the step definition.
func buildStepExecutor(ctx StepBuildContext, s *step, result *core.Step) error {
	const (
		executorKeyType   = "type"
		executorKeyConfig = "config"
	)

	executor := s.Executor

	// Validate that container field and executor field are not both set
	// The container field already specifies the execution method, so setting executor is redundant/conflicting
	if result.Container != nil && executor != nil {
		return core.NewValidationError(
			"executor",
			nil,
			ErrContainerAndExecutorConflict,
		)
	}

	// Case 1: executor is nil - determine executor from container/SSH config
	if executor == nil {
		// Priority 1: Step-level container takes precedence
		// This is the new intuitive syntax for running steps in containers
		if result.Container != nil {
			result.ExecutorConfig.Type = "docker"
			return nil
		}
		// Priority 2: DAG-level container
		if ctx.dag != nil && ctx.dag.Container != nil {
			// Translate the container configuration to executor config
			result.ExecutorConfig.Type = "container"
			return nil
		}
		// Priority 3: DAG-level SSH
		if ctx.dag != nil && ctx.dag.SSH != nil {
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

// buildStepParallel parses the parallel field in the step definition.
func buildStepParallel(_ StepBuildContext, s *step, result *core.Step) error {
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
					if mc > math.MaxInt {
						return core.NewValidationError("parallel.maxConcurrent", mc, fmt.Errorf("value %d exceeds maximum int", mc))
					}
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

// buildStepContainer parses the container field in the step definition.
func buildStepContainer(ctx StepBuildContext, s *step, result *core.Step) error {
	if s.Container == nil {
		return nil
	}

	// Validate that script field is not used with container
	// Scripts are not supported in container execution
	if s.Script != "" {
		return core.NewValidationError(
			"script",
			nil,
			ErrContainerAndScriptConflict,
		)
	}

	ct, err := buildContainerFromSpec(ctx.BuildContext, s.Container)
	if err != nil {
		return err
	}

	result.Container = ct
	return nil
}

// buildStepSubDAG parses the child core.DAG definition and sets up the step to run a sub DAG.
func buildStepSubDAG(ctx StepBuildContext, s *step, result *core.Step) error {
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
