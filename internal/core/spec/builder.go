package spec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// BuildContext is the context for building a DAG.
type BuildContext struct {
	ctx   context.Context
	file  string
	opts  BuildOpts
	index int

	// buildEnv is a temporary map used during core.DAG building to pass env vars to params
	// This is not serialized and is cleared after build completes
	buildEnv map[string]string
}

// StepBuildContext is the context for building a step.
type StepBuildContext struct {
	BuildContext
	dag *core.DAG
}

func (c BuildContext) WithOpts(opts BuildOpts) BuildContext {
	copy := c
	copy.opts = opts
	return copy
}

func (c BuildContext) WithFile(file string) BuildContext {
	copy := c
	copy.file = file
	return copy
}

// BuildFlag represents a bitmask option that influences DAG building behaviour.
type BuildFlag uint32

const (
	BuildFlagNone BuildFlag = 0

	BuildFlagNoEval BuildFlag = 1 << iota
	BuildFlagOnlyMetadata
	BuildFlagAllowBuildErrors
	BuildFlagSkipSchemaValidation
	BuildFlagSkipBaseHandlers // Skip merging handlerOn from base config (for sub-DAG runs)
)

// BuildOpts is used to control the behavior of the builder.
type BuildOpts struct {
	// Base specifies the Base configuration file for the DAG.
	Base string
	// Parameters specifies the Parameters to the DAG.
	// Parameters are used to override the default Parameters in the DAG.
	Parameters string
	// ParametersList specifies the parameters to the DAG.
	ParametersList []string
	// Name of the core.DAG if it's not defined in the spec
	Name string
	// DAGsDir is the directory containing the core.DAG files.
	DAGsDir string
	// DefaultWorkingDir is the default working directory for DAGs without explicit workingDir.
	// This is used for sub-DAG execution to inherit the parent's working directory.
	DefaultWorkingDir string
	// Flags stores all boolean options controlling build behaviour.
	Flags BuildFlag
}

// Has reports whether the flag is enabled on the current BuildOpts.
func (o BuildOpts) Has(flag BuildFlag) bool {
	return o.Flags&flag != 0
}

var stepBuilderRegistry = []stepBuilderEntry{
	{name: "workingDir", fn: buildStepWorkingDir},
	{name: "shell", fn: buildStepShell},
	{name: "executor", fn: buildExecutor},
	{name: "command", fn: buildCommand},
	{name: "params", fn: buildStepParams},
	{name: "timeout", fn: buildStepTimeout},
	{name: "depends", fn: buildDepends},
	{name: "parallel", fn: buildParallel}, // Must be before subDAG to set executor type correctly
	{name: "subDAG", fn: buildSubDAG},
	{name: "continueOn", fn: buildContinueOn},
	{name: "retryPolicy", fn: buildRetryPolicy},
	{name: "repeatPolicy", fn: buildRepeatPolicy},
	{name: "signalOnStop", fn: buildSignalOnStop},
	{name: "precondition", fn: buildStepPrecondition},
	{name: "output", fn: buildOutput},
	{name: "env", fn: buildStepEnvs},
}

type stepBuilderEntry struct {
	name string
	fn   StepBuilderFn
}

// StepBuilderFn is a function that builds a part of the step.
type StepBuilderFn func(ctx StepBuildContext, def step, s *core.Step) error

// parsePrecondition parses the precondition field.
func parsePrecondition(ctx BuildContext, precondition any) ([]*core.Condition, error) {
	switch v := precondition.(type) {
	case nil:
		return nil, nil

	case string:
		return []*core.Condition{{Condition: v}}, nil

	case map[string]any:
		var ret core.Condition
		for key, vv := range v {
			switch strings.ToLower(key) {
			case "condition":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			case "expected":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Expected = val

			case "command":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			case "negate":
				val, ok := vv.(bool)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionNegateMustBeBool)
				}
				ret.Negate = val

			default:
				return nil, core.NewValidationError("preconditions", key, fmt.Errorf("%w: %s", ErrPreconditionHasInvalidKey, key))

			}
		}

		if err := ret.Validate(); err != nil {
			return nil, core.NewValidationError("preconditions", v, err)
		}

		return []*core.Condition{&ret}, nil

	case []any:
		var ret []*core.Condition
		for _, vv := range v {
			parsed, err := parsePrecondition(ctx, vv)
			if err != nil {
				return nil, err
			}
			ret = append(ret, parsed...)
		}
		return ret, nil

	default:
		return nil, core.NewValidationError("preconditions", v, ErrPreconditionMustBeArrayOrString)

	}
}

// parseSecretRefs parses secret references from the YAML definition.
func parseSecretRefs(secretRefs []secretRef) ([]core.SecretRef, error) {

	// Convert secretRef to core.SecretRef and validate
	secrets := make([]core.SecretRef, 0, len(secretRefs))
	names := make(map[string]bool)

	for i, def := range secretRefs {
		// Validate required fields
		if def.Name == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'name' field is required", i))
		}
		if def.Provider == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'provider' field is required", i))
		}
		if def.Key == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'key' field is required", i))
		}

		// Check for duplicate names
		if names[def.Name] {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("duplicate secret name %q", def.Name))
		}
		names[def.Name] = true

		secrets = append(secrets, core.SecretRef{
			Name:     def.Name,
			Provider: def.Provider,
			Key:      def.Key,
			Options:  def.Options,
		})
	}

	return secrets, nil
}

// generateTypedStepName generates a type-based name for a step after it's been built
func generateTypedStepName(existingNames map[string]struct{}, step *core.Step, index int) string {
	var prefix string

	// Determine prefix based on the built step's properties
	if step.ExecutorConfig.Type != "" {
		prefix = step.ExecutorConfig.Type
	} else if step.Parallel != nil {
		prefix = "parallel"
	} else if step.SubDAG != nil {
		prefix = "dag"
	} else if step.Script != "" {
		prefix = "script"
	} else if step.Command != "" {
		prefix = "cmd"
	} else {
		prefix = "step"
	}

	// Generate unique name with the prefix
	counter := index + 1
	name := fmt.Sprintf("%s_%d", prefix, counter)

	for {
		if _, exists := existingNames[name]; !exists {
			existingNames[name] = struct{}{}
			return name
		}
		counter++
		name = fmt.Sprintf("%s_%d", prefix, counter)
	}
}

// normalizedStepData converts string to map[string]any for subsequent process
func normalizeStepData(ctx BuildContext, data []any) []any {
	// Convert string steps to map format for shorthand syntax support
	normalized := make([]any, len(data))
	for i, item := range data {
		switch step := item.(type) {
		case string:
			// Shorthand: convert string to map with command field
			normalized[i] = map[string]any{"command": step}
		default:
			// Keep as-is (already a map or other structure)
			normalized[i] = item
		}
	}
	return normalized
}

// buildStepFromRaw build core.Step from give raw data (map[string]any)
func buildStepFromRaw(ctx StepBuildContext, idx int, raw map[string]any, names map[string]struct{}) (*core.Step, error) {
	var st step
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      &st,
		DecodeHook:  TypedUnionDecodeHook(),
	})
	if err := md.Decode(raw); err != nil {
		return nil, core.NewValidationError("steps", raw, err)
	}
	builtStep, err := buildStep(ctx, st)
	if err != nil {
		return nil, err
	}
	if builtStep.Name == "" {
		builtStep.Name = generateTypedStepName(names, builtStep, idx)
	}
	return builtStep, nil
}

// buildMailConfig builds a core.MailConfig from the mail configuration.
func buildMailConfig(def mailConfig) (*core.MailConfig, error) {
	// StringOrArray already parsed during YAML unmarshal
	toAddresses := def.To.Values()

	// Trim whitespace from addresses
	for i, addr := range toAddresses {
		toAddresses[i] = strings.TrimSpace(addr)
	}

	// Return nil if no valid configuration
	if def.From == "" && len(toAddresses) == 0 {
		return nil, nil
	}

	return &core.MailConfig{
		From:       strings.TrimSpace(def.From),
		To:         toAddresses,
		Prefix:     strings.TrimSpace(def.Prefix),
		AttachLogs: def.AttachLogs,
	}, nil
}

// buildStep builds a step from the step specification.
func buildStep(ctx StepBuildContext, def step) (*core.Step, error) {
	result := &core.Step{
		Name:           strings.TrimSpace(def.Name),
		ID:             strings.TrimSpace(def.ID),
		Description:    strings.TrimSpace(def.Description),
		ShellPackages:  def.ShellPackages,
		Script:         strings.TrimSpace(def.Script),
		Stdout:         strings.TrimSpace(def.Stdout),
		Stderr:         strings.TrimSpace(def.Stderr),
		MailOnError:    def.MailOnError,
		ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)},
	}

	for _, entry := range stepBuilderRegistry {
		if err := entry.fn(ctx, def, result); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
	}

	return result, nil
}

func buildStepWorkingDir(_ StepBuildContext, def step, st *core.Step) error {
	switch {
	case def.WorkingDir != "":
		st.Dir = strings.TrimSpace(def.WorkingDir)
	case def.Dir != "":
		st.Dir = strings.TrimSpace(def.Dir)
	default:
		st.Dir = ""
	}
	return nil
}

func buildStepShell(_ StepBuildContext, def step, st *core.Step) error {
	// ShellValue already parsed during YAML unmarshal
	// Step shell is NOT evaluated here - it's evaluated at runtime
	if def.Shell.IsZero() {
		return nil
	}

	if def.Shell.IsArray() {
		st.Shell = def.Shell.Command()
		st.ShellArgs = def.Shell.Arguments()
		return nil
	}

	// For string form, need to split command and args
	command := def.Shell.Command()
	if command == "" {
		return nil
	}

	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return core.NewValidationError("shell", def.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	st.Shell = strings.TrimSpace(shell)
	st.ShellArgs = args
	return nil
}

func buildStepTimeout(_ StepBuildContext, def step, st *core.Step) error {
	if def.TimeoutSec < 0 {
		return core.NewValidationError("timeoutSec", def.TimeoutSec, ErrTimeoutSecMustBeNonNegative)
	}
	if def.TimeoutSec == 0 {
		// Zero means no timeout; leave unset.
		return nil
	}
	st.Timeout = time.Second * time.Duration(def.TimeoutSec)
	return nil
}

func buildContinueOn(_ StepBuildContext, def step, st *core.Step) error {
	// ContinueOnValue already parsed and validated during YAML unmarshal
	if def.ContinueOn.IsZero() {
		return nil
	}

	st.ContinueOn.Skipped = def.ContinueOn.Skipped()
	st.ContinueOn.Failure = def.ContinueOn.Failed()
	st.ContinueOn.MarkSuccess = def.ContinueOn.MarkSuccess()
	st.ContinueOn.ExitCode = def.ContinueOn.ExitCode()
	st.ContinueOn.Output = def.ContinueOn.Output()

	return nil
}

// buildRetryPolicy builds the retry policy for a step.
func buildRetryPolicy(_ StepBuildContext, def step, st *core.Step) error {
	if def.RetryPolicy != nil {
		switch v := def.RetryPolicy.Limit.(type) {
		case int:
			st.RetryPolicy.Limit = v
			st.RetryPolicy.Limit = int(v)
		case int64:
			st.RetryPolicy.Limit = int(v)
		case uint64:
			st.RetryPolicy.Limit = int(v)
		case string:
			st.RetryPolicy.LimitStr = v
		default:
			return core.NewValidationError("retryPolicy.Limit", v, fmt.Errorf("invalid type: %T", v))
		}

		switch v := def.RetryPolicy.IntervalSec.(type) {
		case int:
			st.RetryPolicy.Interval = time.Second * time.Duration(v)
		case int64:
			st.RetryPolicy.Interval = time.Second * time.Duration(v)
		case uint64:
			st.RetryPolicy.Interval = time.Second * time.Duration(v)
		case string:
			st.RetryPolicy.IntervalSecStr = v
		default:
			return core.NewValidationError("retryPolicy.IntervalSec", v, fmt.Errorf("invalid type: %T", v))
		}

		if def.RetryPolicy.ExitCode != nil {
			st.RetryPolicy.ExitCodes = def.RetryPolicy.ExitCode
		}

		// Parse backoff field
		if def.RetryPolicy.Backoff != nil {
			switch v := def.RetryPolicy.Backoff.(type) {
			case bool:
				if v {
					st.RetryPolicy.Backoff = 2.0 // Default multiplier when true
				}
			case int:
				st.RetryPolicy.Backoff = float64(v)
			case int64:
				st.RetryPolicy.Backoff = float64(v)
			case float64:
				st.RetryPolicy.Backoff = v
			default:
				return core.NewValidationError("retryPolicy.Backoff", v, fmt.Errorf("invalid type: %T", v))
			}

			// Validate backoff value
			if st.RetryPolicy.Backoff > 0 && st.RetryPolicy.Backoff <= 1.0 {
				return core.NewValidationError("retryPolicy.Backoff", st.RetryPolicy.Backoff,
					fmt.Errorf("backoff must be greater than 1.0 for exponential growth"))
			}
		}

		// Parse maxIntervalSec
		if def.RetryPolicy.MaxIntervalSec > 0 {
			st.RetryPolicy.MaxInterval = time.Second * time.Duration(def.RetryPolicy.MaxIntervalSec)
		}
	}
	return nil
}

// buildRepeatPolicy sets up the repeat policy for a step.
//
// The repeat policy supports two modes: "while" and "until", which determine when repetition stops:
// - "while": Repeat as long as the condition is true (continues while condition matches)
// - "until": Repeat as long as the condition is false (stops when condition matches)
//
// Configuration options:
//
//  1. Explicit mode with condition:
//     repeatPolicy:
//     repeat: "while"  # or "until"
//     condition: "echo test"
//     expected: "test"  # optional, defaults to exit code 0 check
//     intervalSec: 30
//     limit: 10
//
//  2. Explicit mode with exit codes:
//     repeatPolicy:
//     repeat: "while"  # or "until"
//     exitCode: [0, 1]  # repeat while/until exit code matches any in list
//     intervalSec: 30
//
//  3. Boolean mode (backward compatibility):
//     repeatPolicy:
//     repeat: true  # equivalent to "while" mode, repeats unconditionally
//     intervalSec: 30
//
//  4. Backward compatibility (mode inferred from configuration):
//     repeatPolicy:
//     condition: "echo test"
//     expected: "test"     # inferred as "until" mode
//     intervalSec: 30
//     OR
//     repeatPolicy:
//     condition: "echo test"  # inferred as "while" mode (condition only)
//     intervalSec: 30
//     OR
//     repeatPolicy:
//     exitCode: [1, 2]       # inferred as "while" mode
//     intervalSec: 30
//
// Validation rules:
// - Explicit "while"/"until" modes require either 'condition' or 'exitCode' to be specified
// - If both 'condition' and 'expected' are set, the mode defaults to "until"
// - If only 'condition' or only 'exitCode' is set, the mode defaults to "while"
// - Boolean true is equivalent to "while" mode with unconditional repetition
//
// Precedence: condition > exitCode > unconditional repeat
func buildRepeatPolicy(_ StepBuildContext, def step, st *core.Step) error {
	if def.RepeatPolicy == nil {
		return nil
	}
	rpDef := def.RepeatPolicy

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
			return fmt.Errorf("invalid value for repeat: '%s'. It must be 'while', 'until', or a boolean", v)
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
		// Check if mode was explicitly set (not inferred from backward compatibility)
		switch v := rpDef.Repeat.(type) {
		case string:
			if (v == "while" || v == "until") && rpDef.Condition == "" && len(rpDef.ExitCode) == 0 {
				return fmt.Errorf("repeat mode '%s' requires either 'condition' or 'exitCode' to be specified", v)
			}
		}
	}

	st.RepeatPolicy.RepeatMode = mode
	if rpDef.IntervalSec > 0 {
		st.RepeatPolicy.Interval = time.Second * time.Duration(rpDef.IntervalSec)
	}
	st.RepeatPolicy.Limit = rpDef.Limit

	if rpDef.Condition != "" {
		st.RepeatPolicy.Condition = &core.Condition{
			Condition: rpDef.Condition,
			Expected:  rpDef.Expected,
		}
	}
	st.RepeatPolicy.ExitCode = rpDef.ExitCode

	// Parse backoff field
	if rpDef.Backoff != nil {
		switch v := rpDef.Backoff.(type) {
		case bool:
			if v {
				st.RepeatPolicy.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			st.RepeatPolicy.Backoff = float64(v)
		case int64:
			st.RepeatPolicy.Backoff = float64(v)
		case float64:
			st.RepeatPolicy.Backoff = v
		default:
			return fmt.Errorf("invalid value for backoff: '%v'. It must be a boolean or number", v)
		}

		// Validate backoff value
		if st.RepeatPolicy.Backoff > 0 && st.RepeatPolicy.Backoff <= 1.0 {
			return fmt.Errorf("backoff must be greater than 1.0 for exponential growth, got: %v",
				st.RepeatPolicy.Backoff)
		}
	}

	// Parse maxIntervalSec
	if rpDef.MaxIntervalSec > 0 {
		st.RepeatPolicy.MaxInterval = time.Second * time.Duration(rpDef.MaxIntervalSec)
	}

	return nil
}

func buildOutput(_ StepBuildContext, def step, st *core.Step) error {
	if def.Output == "" {
		return nil
	}

	if strings.HasPrefix(def.Output, "$") {
		st.Output = strings.TrimPrefix(def.Output, "$")
		return nil
	}

	st.Output = strings.TrimSpace(def.Output)
	return nil
}

func buildStepEnvs(_ StepBuildContext, def step, st *core.Step) error {
	// EnvValue already parsed during YAML unmarshal
	if def.Env.IsZero() {
		return nil
	}
	// For step environment variables, we don't evaluate them here.
	// They will be evaluated later when the step is executed.
	for _, entry := range def.Env.Entries() {
		st.Env = append(st.Env, fmt.Sprintf("%s=%s", entry.Key, entry.Value))
	}
	return nil
}

func buildStepPrecondition(ctx StepBuildContext, def step, st *core.Step) error {
	// Parse both `preconditions` and `precondition` fields.
	conditions, err := parsePrecondition(ctx.BuildContext, def.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx.BuildContext, def.Precondition)
	if err != nil {
		return err
	}
	st.Preconditions = conditions
	st.Preconditions = append(st.Preconditions, condition...)
	return nil
}

func buildSignalOnStop(_ StepBuildContext, def step, st *core.Step) error {
	if def.SignalOnStop != nil {
		sigDef := *def.SignalOnStop
		sig := signal.GetSignalNum(sigDef, 0)
		if sig == 0 {
			return fmt.Errorf("%w: %s", ErrInvalidSignal, sigDef)
		}
		st.SignalOnStop = sigDef
	}
	return nil
}

// buildSubDAG parses the child core.DAG definition and sets up the step to run a sub DAG.
func buildSubDAG(ctx StepBuildContext, def step, st *core.Step) error {
	name := strings.TrimSpace(def.Call)
	if name == "" {
		// TODO: remove legacy support in future major version
		if legacyName := strings.TrimSpace(def.Run); legacyName != "" {
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
	if def.Params != nil {
		// Parse the params to convert them to string format
		ctxCopy := ctx
		ctxCopy.opts.Flags |= BuildFlagNoEval // Disable evaluation for params parsing
		paramPairs, err := parseParamValue(ctxCopy.BuildContext, def.Params)
		if err != nil {
			return core.NewValidationError("params", def.Params, err)
		}

		// Convert to string format "key=value key=value ..."
		var paramsToJoin []string
		for _, paramPair := range paramPairs {
			paramsToJoin = append(paramsToJoin, paramPair.Escaped())
		}
		paramsStr = strings.Join(paramsToJoin, " ")
	}

	st.SubDAG = &core.SubDAG{Name: name, Params: paramsStr}

	// Set executor type based on whether parallel execution is configured
	if st.Parallel != nil {
		st.ExecutorConfig.Type = core.ExecutorTypeParallel
	} else {
		st.ExecutorConfig.Type = core.ExecutorTypeDAG
	}

	st.Command = "call"
	st.Args = []string{name, paramsStr}
	st.CmdWithArgs = strings.TrimSpace(fmt.Sprintf("%s %s", name, paramsStr))
	return nil
}

// buildDepends parses the depends field in the step specification.
func buildDepends(_ StepBuildContext, def step, st *core.Step) error {
	// StringOrArray already parsed during YAML unmarshal
	st.Depends = def.Depends.Values()

	// Check if depends was explicitly set to empty array
	if !def.Depends.IsZero() && def.Depends.IsEmpty() {
		st.ExplicitlyNoDeps = true
	}

	return nil
}

// buildExecutor parses the executor field in the step definition.
// Case 1: executor is nil
//
//	Case 1.1: core.DAG level 'container' field is set
//	Case 1.2: core.DAG 'ssh' field is set
//	Case 1.3: No executor is set, use default executor
//
// Case 2: executor is a string
// Case 3: executor is a struct
func buildExecutor(ctx StepBuildContext, def step, st *core.Step) error {
	const (
		executorKeyType   = "type"
		executorKeyConfig = "config"
	)

	executor := def.Executor

	// Case 1: executor is nil
	if executor == nil {
		if ctx.dag.Container != nil {
			// Translate the container configuration to executor config
			return translateExecutorConfig(ctx, def, st)
		} else if ctx.dag.SSH != nil {
			return translateSSHConfig(ctx, def, st)
		}
		return nil
	}

	switch val := executor.(type) {
	case string:
		// Case 2: executor is a string
		// This can be an executor with default configuration.
		st.ExecutorConfig.Type = strings.TrimSpace(val)

	case map[string]any:
		// Case 3: executor is a struct
		// In this case, the executor is a struct with type and config fields.
		// Config is a map of string keys and values.
		for key, v := range val {
			switch key {
			case executorKeyType:
				// Executor type is a string.
				typ, ok := v.(string)
				if !ok {
					return core.NewValidationError("executor.type", v, ErrExecutorTypeMustBeString)
				}
				st.ExecutorConfig.Type = strings.TrimSpace(typ)

			case executorKeyConfig:
				// Executor config is a map of string keys and values.
				// The values can be of any type.
				// It is up to the executor to parse the values.
				executorConfig, ok := v.(map[string]any)
				if !ok {
					return core.NewValidationError("executor.config", v, ErrExecutorConfigValueMustBeMap)
				}
				for configKey, v := range executorConfig {
					st.ExecutorConfig.Config[configKey] = v
				}

			default:
				// Unknown key in the executor config.
				return core.NewValidationError("executor.config", key, fmt.Errorf("%w: %s", ErrExecutorHasInvalidKey, key))

			}
		}

	default:
		// Unknown key for executor field.
		return core.NewValidationError("executor", val, ErrExecutorConfigMustBeStringOrMap)

	}

	return nil
}

func translateExecutorConfig(ctx StepBuildContext, def step, st *core.Step) error {
	// If the executor is nil, but the core.DAG has a container field,
	// we translate the container configuration to executor config.
	if ctx.dag.Container == nil {
		return nil // No container configuration to translate
	}

	// Translate container fields to executor config
	st.ExecutorConfig.Type = "container"

	// The other fields will be retrieved from the container configuration on
	// execution time, so we don't need to set them here.

	return nil
}

func translateSSHConfig(ctx StepBuildContext, def step, st *core.Step) error {
	if ctx.dag.SSH == nil {
		return nil // No container configuration to translate
	}

	st.ExecutorConfig.Type = "ssh"

	// The other fields will be retrieved from the container configuration on
	// execution time, so we don't need to set them here.

	return nil
}

// buildParallel parses the parallel field in the step definition.
// MVP supports:
// - Direct array reference: parallel: ${ITEMS}
// - Static array: parallel: [item1, item2]
// - Object configuration: parallel: {items: [...], maxConcurrent: 5}
func buildParallel(ctx StepBuildContext, def step, st *core.Step) error {
	if def.Parallel == nil {
		return nil
	}

	st.Parallel = &core.ParallelConfig{
		MaxConcurrent: core.DefaultMaxConcurrent,
	}

	switch v := def.Parallel.(type) {
	case string:
		// Direct variable reference like: parallel: ${ITEMS}
		// The actual items will be resolved at runtime
		// It should be resolved to a json array of items
		// e.g. ["item1", "item2"] or [{"SOURCE": "s3://..."}]
		st.Parallel.Variable = v

	case []any:
		// Static array: parallel: [item1, item2] or parallel: [{SOURCE: s3://...}, ...]
		items, err := parseParallelItems(v)
		if err != nil {
			return core.NewValidationError("parallel", v, err)
		}
		st.Parallel.Items = items

	case map[string]any:
		// Object configuration
		for key, val := range v {
			switch key {
			case "items":
				switch itemsVal := val.(type) {
				case string:
					// Variable reference in object form
					st.Parallel.Variable = itemsVal
				case []any:
					// Direct array in object form
					items, err := parseParallelItems(itemsVal)
					if err != nil {
						return core.NewValidationError("parallel.items", itemsVal, err)
					}
					st.Parallel.Items = items
				default:
					return core.NewValidationError("parallel.items", val, fmt.Errorf("parallel.items must be string or array, got %T", val))
				}

			case "maxConcurrent":
				switch mc := val.(type) {
				case int:
					st.Parallel.MaxConcurrent = mc
				case int64:
					st.Parallel.MaxConcurrent = int(mc)
				case uint64:
					st.Parallel.MaxConcurrent = int(mc)
				case float64:
					st.Parallel.MaxConcurrent = int(mc)
				default:
					return core.NewValidationError("parallel.maxConcurrent", val, fmt.Errorf("parallel.maxConcurrent must be int, got %T", val))
				}

			default:
				// Ignore unknown keys for now (future extensibility)
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
			// Simple string item
			result = append(result, core.ParallelItem{Value: v})

		case int, int64, uint64, float64:
			// Numeric items, convert to string
			result = append(result, core.ParallelItem{Value: fmt.Sprintf("%v", v)})

		case map[string]any:
			// Object with parameters
			params := make(collections.DeterministicMap)
			for key, val := range v {
				var strVal string
				switch v := val.(type) {
				case string:
					strVal = v
				case int:
					strVal = fmt.Sprintf("%d", v)
				case int64:
					strVal = fmt.Sprintf("%d", v)
				case uint64:
					strVal = fmt.Sprintf("%d", v)
				case float64:
					strVal = fmt.Sprintf("%g", v)
				case bool:
					strVal = fmt.Sprintf("%t", v)
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

// injectChainDependencies adds implicit dependencies for chain type execution.
// In chain execution, each step depends on all previous steps unless explicitly configured otherwise.
func injectChainDependencies(dag *core.DAG, prevSteps []*core.Step, step *core.Step) {
	// Early returns for cases where we shouldn't inject dependencies
	if dag.Type != core.TypeChain || step.ExplicitlyNoDeps || len(prevSteps) == 0 {
		return
	}

	// Build a set of existing dependencies for efficient lookup
	existingDeps := make(map[string]struct{}, len(step.Depends))
	for _, dep := range step.Depends {
		existingDeps[dep] = struct{}{}
	}

	// Add each previous step as a dependency if not already present
	for _, prevStep := range prevSteps {
		depKey := getStepKey(prevStep)

		// Skip if this dependency already exists
		if _, exists := existingDeps[depKey]; exists {
			continue
		}

		// Also check alternate key (ID vs Name) to avoid duplicates
		altKey := getStepAlternateKey(prevStep, depKey)
		if altKey != "" {
			if _, exists := existingDeps[altKey]; exists {
				continue
			}
		}

		step.Depends = append(step.Depends, depKey)
		existingDeps[depKey] = struct{}{}
	}
}

// getStepKey returns the preferred identifier for a step (ID if available, otherwise Name)
func getStepKey(step *core.Step) string {
	if step.ID != "" {
		return step.ID
	}
	return step.Name
}

// getStepAlternateKey returns the alternate identifier for a step, or empty string if none
func getStepAlternateKey(step *core.Step, primaryKey string) string {
	if step.ID != "" && primaryKey == step.ID {
		return step.Name
	}
	if step.ID != "" && primaryKey == step.Name {
		return step.ID
	}
	return ""
}

