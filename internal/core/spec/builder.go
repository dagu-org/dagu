package spec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// BuilderFn is a function that builds a part of the DAG.
type BuilderFn func(ctx BuildContext, spec *definition, dag *core.DAG) error

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
	// Flags stores all boolean options controlling build behaviour.
	Flags BuildFlag
}

// Has reports whether the flag is enabled on the current BuildOpts.
func (o BuildOpts) Has(flag BuildFlag) bool {
	return o.Flags&flag != 0
}

var builderRegistry = []builderEntry{
	{metadata: true, name: "env", fn: buildEnvs},
	{metadata: true, name: "schedule", fn: buildSchedule},
	{metadata: true, name: "skipIfSuccessful", fn: skipIfSuccessful},
	{metadata: true, name: "params", fn: buildParams},
	{metadata: true, name: "name", fn: buildName},
	{metadata: true, name: "type", fn: buildType},
	{metadata: true, name: "runConfig", fn: buildRunConfig},
	{metadata: true, name: "maxActiveRuns", fn: buildMaxActiveRuns},
	{metadata: true, name: "workerSelector", fn: buildWorkerSelector},
	{name: "shell", fn: buildShell},
	{name: "workingDir", fn: buildWorkingDir},
	{name: "container", fn: buildContainer},
	{name: "registryAuths", fn: buildRegistryAuths},
	{name: "ssh", fn: buildSSH},
	{name: "secrets", fn: buildSecrets},
	{name: "dotenv", fn: buildDotenv},
	{name: "mailOn", fn: buildMailOn},
	{name: "logDir", fn: buildLogDir},
	{name: "handlers", fn: buildHandlers},
	{name: "smtpConfig", fn: buildSMTPConfig},
	{name: "errMailConfig", fn: buildErrMailConfig},
	{name: "infoMailConfig", fn: buildInfoMailConfig},
	{name: "maxHistoryRetentionDays", fn: maxHistoryRetentionDays},
	{name: "maxCleanUpTime", fn: maxCleanUpTime},
	{name: "preconditions", fn: buildPrecondition},
	{name: "otel", fn: buildOTel},
	{name: "steps", fn: buildSteps},
}

type builderEntry struct {
	metadata bool
	name     string
	fn       BuilderFn
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
type StepBuilderFn func(ctx StepBuildContext, def stepDef, step *core.Step) error

// build builds a core.DAG from the specification.
func build(ctx BuildContext, spec *definition) (*core.DAG, error) {
	dag := &core.DAG{
		Location:       ctx.file,
		Group:          strings.TrimSpace(spec.Group),
		Description:    strings.TrimSpace(spec.Description),
		Type:           strings.TrimSpace(spec.Type),
		Timeout:        time.Second * time.Duration(spec.TimeoutSec),
		Delay:          time.Second * time.Duration(spec.DelaySec),
		RestartWait:    time.Second * time.Duration(spec.RestartWaitSec),
		Tags:           parseTags(spec.Tags),
		MaxActiveSteps: spec.MaxActiveSteps,
		Queue:          strings.TrimSpace(spec.Queue),
		MaxOutputSize:  spec.MaxOutputSize,
	}

	var errs core.ErrorList
	for _, builder := range builderRegistry {
		if !builder.metadata && ctx.opts.Has(BuildFlagOnlyMetadata) {
			continue
		}
		if err := builder.fn(ctx, spec, dag); err != nil {
			// Avoid duplicating field prefixes like "field 'steps': field 'steps': ..."
			var le *core.ValidationError
			if errors.As(err, &le) && le.Field == builder.name {
				errs = append(errs, err)
			} else {
				errs = append(errs, core.NewValidationError(builder.name, nil, err))
			}
		}
	}

	if err := core.ValidateSteps(dag); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		if ctx.opts.Has(BuildFlagAllowBuildErrors) {
			// If we are allowing build errors, return the core.DAG with the errors.
			dag.BuildErrors = errs
		} else {
			// If we are not allowing build errors, return an error.
			return nil, fmt.Errorf("failed to build DAG: %w", errs)
		}
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
func buildSchedule(_ BuildContext, spec *definition, dag *core.DAG) error {
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
				return core.NewValidationError("schedule", s, ErrScheduleMustBeStringOrArray)
			}
			starts = append(starts, s)
		}

	case map[string]any:
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
		return core.NewValidationError("schedule", spec.Schedule, ErrInvalidScheduleType)

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

func buildContainer(ctx BuildContext, spec *definition, dag *core.DAG) error {
	if spec.Container == nil {
		return nil
	}

	// Validate required fields
	if spec.Container.Image == "" {
		return core.NewValidationError("container.image", spec.Container.Image, fmt.Errorf("image is required when container is specified"))
	}

	pullPolicy, err := core.ParsePullPolicy(spec.Container.PullPolicy)
	if err != nil {
		return core.NewValidationError("container.pullPolicy", spec.Container.PullPolicy, err)
	}

	vars, err := loadVariables(ctx, spec.Container.Env)
	if err != nil {
		return core.NewValidationError("container.env", spec.Container.Env, err)
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	container := core.Container{
		Image:         spec.Container.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       spec.Container.Volumes,
		User:          spec.Container.User,
		Platform:      spec.Container.Platform,
		Ports:         spec.Container.Ports,
		Network:       spec.Container.Network,
		KeepContainer: spec.Container.KeepContainer,
		Startup:       core.ContainerStartup(strings.ToLower(strings.TrimSpace(spec.Container.Startup))),
		Command:       append([]string{}, spec.Container.Command...),
		WaitFor:       core.ContainerWaitFor(strings.ToLower(strings.TrimSpace(spec.Container.WaitFor))),
		LogPattern:    spec.Container.LogPattern,
		RestartPolicy: strings.TrimSpace(spec.Container.RestartPolicy),
	}

	// Backward compatibility
	if spec.Container.WorkDir != "" {
		container.WorkingDir = spec.Container.WorkDir
	} else {
		container.WorkingDir = spec.Container.WorkingDir
	}

	dag.Container = &container

	return nil
}

func buildWorkerSelector(ctx BuildContext, spec *definition, dag *core.DAG) error {
	if len(spec.WorkerSelector) == 0 {
		return nil
	}

	ret := make(map[string]string)

	for key, val := range spec.WorkerSelector {
		ret[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}

	dag.WorkerSelector = ret
	return nil
}

func buildRegistryAuths(ctx BuildContext, spec *definition, dag *core.DAG) error {
	if spec.RegistryAuths == nil {
		return nil
	}

	dag.RegistryAuths = make(map[string]*core.AuthConfig)

	switch v := spec.RegistryAuths.(type) {
	case string:
		// Handle as a JSON string (e.g., from environment variable)
		expandedJSON := v
		if !ctx.opts.Has(BuildFlagNoEval) {
			expandedJSON = os.ExpandEnv(v)
		}

		// For now, store the entire JSON string as a single entry
		// The actual parsing will be done by the registry manager
		dag.RegistryAuths["_json"] = &core.AuthConfig{
			Auth: expandedJSON,
		}

	case map[string]any:
		// Handle as a map of registry to auth config
		for registry, authData := range v {
			authConfig := &core.AuthConfig{}

			switch auth := authData.(type) {
			case string:
				// Simple string value - treat as JSON auth config
				if !ctx.opts.Has(BuildFlagNoEval) {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
				// Auth config object with username/password fields
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Auth = os.ExpandEnv(authConfig.Auth)
					}
				}
			}

			dag.RegistryAuths[registry] = authConfig
		}

	case map[any]any:
		// Handle YAML's default map type
		for registryKey, authData := range v {
			registry, ok := registryKey.(string)
			if !ok {
				continue
			}

			authConfig := &core.AuthConfig{}

			switch auth := authData.(type) {
			case string:
				// Simple string value - treat as JSON auth config
				if !ctx.opts.Has(BuildFlagNoEval) {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
				// Auth config object with username/password fields
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Auth = os.ExpandEnv(authConfig.Auth)
					}
				}

			case map[any]any:
				// Handle nested YAML map type
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.Has(BuildFlagNoEval) {
						authConfig.Auth = os.ExpandEnv(authConfig.Auth)
					}
				}
			}

			dag.RegistryAuths[registry] = authConfig
		}

	default:
		return core.NewValidationError("registryAuths", spec.RegistryAuths, fmt.Errorf("invalid type: %T", spec.RegistryAuths))
	}

	return nil
}

func buildDotenv(ctx BuildContext, spec *definition, dag *core.DAG) error {
	switch v := spec.Dotenv.(type) {
	case nil:
		dag.Dotenv = append(dag.Dotenv, ".env")

	case string:
		dag.Dotenv = append(dag.Dotenv, v)

	case []any:
		for _, e := range v {
			switch e := e.(type) {
			case string:
				dag.Dotenv = append(dag.Dotenv, e)
			default:
				return core.NewValidationError("dotenv", e, ErrDotEnvMustBeStringOrArray)
			}
		}
	default:
		return core.NewValidationError("dotenv", v, ErrDotEnvMustBeStringOrArray)
	}

	if !ctx.opts.Has(BuildFlagNoEval) {
		dag.LoadDotEnv(ctx.ctx)
	}

	return nil
}

func buildMailOn(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.MailOn == nil {
		return nil
	}
	dag.MailOn = &core.MailOn{
		Failure: spec.MailOn.Failure,
		Success: spec.MailOn.Success,
	}
	return nil
}

// buildName set the name if name is specified by the option but if Name is defined
// it does not override
func buildName(ctx BuildContext, spec *definition, dag *core.DAG) error {
	dag.Name = findName(ctx, spec)
	if dag.Name == "" {
		return nil
	}
	if err := core.ValidateDAGName(dag.Name); err != nil {
		return core.NewValidationError("name", dag.Name, err)
	}
	return nil
}

func findName(ctx BuildContext, spec *definition) string {
	if ctx.opts.Name != "" {
		return strings.TrimSpace(ctx.opts.Name)
	}
	if spec.Name != "" {
		return strings.TrimSpace(spec.Name)
	}
	if ctx.file != "" && ctx.index == 0 {
		// Use the filename if the core.DAG is from a file and it's the first document
		filename := filepath.Base(ctx.file)
		return strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	return ""
}

// parseShellValue parses a shell value (string or array) into shell command and args.
// If expandEnv is true, environment variables are expanded in the values.
// Returns shell command, args, and error.
func parseShellValue(val any, expandEnv bool) (string, []string, error) {
	if val == nil {
		return "", nil, nil
	}

	switch v := val.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return "", nil, nil
		}
		if expandEnv {
			v = os.ExpandEnv(v)
		}
		cmd, args, err := cmdutil.SplitCommand(v)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse shell command: %w", err)
		}
		return strings.TrimSpace(cmd), args, nil

	case []any:
		if len(v) == 0 {
			return "", nil, nil
		}
		var shell string
		var args []string
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				s = fmt.Sprintf("%v", item)
			}
			s = strings.TrimSpace(s)
			if expandEnv {
				s = os.ExpandEnv(s)
			}
			if i == 0 {
				shell = s
			} else {
				args = append(args, s)
			}
		}
		return shell, args, nil

	default:
		return "", nil, fmt.Errorf("shell must be a string or array, got %T", val)
	}
}

func buildShell(ctx BuildContext, spec *definition, dag *core.DAG) error {
	shell, args, err := parseShellValue(spec.Shell, !ctx.opts.Has(BuildFlagNoEval))
	if err != nil {
		return core.NewValidationError("shell", spec.Shell, err)
	}
	if shell == "" {
		dag.Shell = cmdutil.GetShellCommand("")
	} else {
		dag.Shell = shell
		dag.ShellArgs = args
	}
	return nil
}

func buildWorkingDir(ctx BuildContext, spec *definition, dag *core.DAG) error {
	switch {
	case spec.WorkingDir != "":
		wd := spec.WorkingDir
		if !ctx.opts.Has(BuildFlagNoEval) {
			wd = os.ExpandEnv(wd)
			switch {
			case filepath.IsAbs(wd) || strings.HasPrefix(wd, "~"):
				wd = fileutil.ResolvePathOrBlank(wd)
			case ctx.file != "":
				wd = filepath.Join(filepath.Dir(ctx.file), wd)
			default:
				wd = fileutil.ResolvePathOrBlank(wd)
			}
		}
		dag.WorkingDir = wd

	case ctx.file != "":
		wd := filepath.Dir(ctx.file)
		dag.WorkingDir = wd

	default:
		dir, _ := os.Getwd()
		if dir == "" {
			// try to get home dir
			var err error
			dir, err = os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
		}
		dag.WorkingDir = dir
	}

	return nil
}

// buildType validates and sets the execution type for the DAG
func buildType(_ BuildContext, spec *definition, dag *core.DAG) error {
	// Set default if not specified
	if dag.Type == "" {
		dag.Type = core.TypeChain
	}

	// Validate the type
	switch dag.Type {
	case core.TypeGraph, core.TypeChain:
		// Valid types
		return nil
	case core.TypeAgent:
		return core.NewValidationError("type", dag.Type, fmt.Errorf("agent type is not yet implemented"))
	default:
		return core.NewValidationError("type", dag.Type, fmt.Errorf("invalid type: %s (must be one of: graph, chain, agent)", dag.Type))
	}
}

// buildEnvs builds the environment variables for the DAG.
// Case 1: env is an array of maps with string keys and string values.
// Case 2: env is a map with string keys and string values.
func buildEnvs(ctx BuildContext, spec *definition, dag *core.DAG) error {
	vars, err := loadVariables(ctx, spec.Env)
	if err != nil {
		return err
	}

	// Store env vars in core.DAG temporarily for params to reference (e.g., P2=${A001})
	// This is cleared after params are built
	if ctx.buildEnv == nil {
		ctx.buildEnv = make(map[string]string)
	}
	for k, v := range vars {
		dag.Env = append(dag.Env, fmt.Sprintf("%s=%s", k, v))
		ctx.buildEnv[k] = v
	}

	return nil
}

// buildLogDir builds the log directory for the DAG.
func buildLogDir(_ BuildContext, spec *definition, dag *core.DAG) (err error) {
	dag.LogDir = spec.LogDir
	return err
}

// buildHandlers builds the handlers for the DAG.
// The handlers are executed when the core.DAG is stopped, succeeded, failed, or
// cancelled.
func buildHandlers(ctx BuildContext, spec *definition, dag *core.DAG) (err error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: dag}

	if spec.HandlerOn.Init != nil {
		spec.HandlerOn.Init.Name = core.HandlerOnInit.String()
		if dag.HandlerOn.Init, err = buildStep(buildCtx, *spec.HandlerOn.Init); err != nil {
			return err
		}
	}

	if spec.HandlerOn.Exit != nil {
		spec.HandlerOn.Exit.Name = core.HandlerOnExit.String()
		if dag.HandlerOn.Exit, err = buildStep(buildCtx, *spec.HandlerOn.Exit); err != nil {
			return err
		}
	}

	if spec.HandlerOn.Success != nil {
		spec.HandlerOn.Success.Name = core.HandlerOnSuccess.String()
		if dag.HandlerOn.Success, err = buildStep(buildCtx, *spec.HandlerOn.Success); err != nil {
			return
		}
	}

	if spec.HandlerOn.Failure != nil {
		spec.HandlerOn.Failure.Name = core.HandlerOnFailure.String()
		if dag.HandlerOn.Failure, err = buildStep(buildCtx, *spec.HandlerOn.Failure); err != nil {
			return
		}
	}

	// Handle Abort (canonical) and Cancel (deprecated, for backward compatibility)
	// Error if both are specified
	if spec.HandlerOn.Abort != nil && spec.HandlerOn.Cancel != nil {
		return fmt.Errorf("cannot specify both 'abort' and 'cancel' in handlerOn; use 'abort' (cancel is deprecated)")
	}
	var abortDef *stepDef
	switch {
	case spec.HandlerOn.Abort != nil:
		abortDef = spec.HandlerOn.Abort
	case spec.HandlerOn.Cancel != nil:
		abortDef = spec.HandlerOn.Cancel
	}
	if abortDef != nil {
		abortDef.Name = core.HandlerOnCancel.String()
		if dag.HandlerOn.Cancel, err = buildStep(buildCtx, *abortDef); err != nil {
			return
		}
	}

	return nil
}

func buildPrecondition(ctx BuildContext, spec *definition, dag *core.DAG) error {
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

func maxCleanUpTime(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.MaxCleanUpTimeSec != nil {
		dag.MaxCleanUpTime = time.Second * time.Duration(*spec.MaxCleanUpTimeSec)
	}
	return nil
}

func buildMaxActiveRuns(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.MaxActiveRuns != 0 {
		// Preserve the value if it's set (including negative values)
		// MaxActiveRuns < 0 means queueing is disabled for this DAG
		dag.MaxActiveRuns = spec.MaxActiveRuns
	} else {
		// Default to 1 only when not specified (0)
		dag.MaxActiveRuns = 1
	}
	return nil
}

func maxHistoryRetentionDays(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.HistRetentionDays != nil {
		dag.HistRetentionDays = *spec.HistRetentionDays
	}
	return nil
}

// skipIfSuccessful sets the skipIfSuccessful field for the DAG.
func skipIfSuccessful(_ BuildContext, spec *definition, dag *core.DAG) error {
	dag.SkipIfSuccessful = spec.SkipIfSuccessful
	return nil
}

// buildRunConfig builds the run configuration for the DAG.
func buildRunConfig(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.RunConfig == nil {
		return nil
	}
	dag.RunConfig = &core.RunConfig{
		DisableParamEdit: spec.RunConfig.DisableParamEdit,
		DisableRunIdEdit: spec.RunConfig.DisableRunIdEdit,
	}
	return nil
}

// buildSSH builds the SSH configuration for the DAG.
func buildSSH(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.SSH == nil {
		return nil
	}

	// Parse port - can be string or number
	port := ""
	switch v := spec.SSH.Port.(type) {
	case string:
		port = v
	case int:
		port = fmt.Sprintf("%d", v)
	case int64:
		port = fmt.Sprintf("%d", v)
	case uint64:
		port = fmt.Sprintf("%d", v)
	case float64:
		port = fmt.Sprintf("%.0f", v)
	case nil:
		port = ""
	default:
		return fmt.Errorf("invalid SSH port type: %T", v)
	}

	// Set default port if not specified
	if port == "" {
		port = "22"
	}

	// Default strictHostKey to true if not explicitly set
	strictHostKey := true
	if spec.SSH.StrictHostKey != nil {
		strictHostKey = *spec.SSH.StrictHostKey
	}

	dag.SSH = &core.SSHConfig{
		User:          spec.SSH.User,
		Host:          spec.SSH.Host,
		Port:          port,
		Key:           spec.SSH.Key,
		Password:      spec.SSH.Password,
		StrictHostKey: strictHostKey,
		KnownHostFile: spec.SSH.KnownHostFile,
	}

	return nil
}

// buildSecrets builds the secrets references from the spec.
func buildSecrets(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.Secrets == nil {
		return nil
	}

	secrets, err := parseSecretRefs(spec.Secrets)
	if err != nil {
		return err
	}

	dag.Secrets = secrets
	return nil
}

// parseSecretRefs parses secret references from the YAML definition.
func parseSecretRefs(secretDefs []secretRefDef) ([]core.SecretRef, error) {

	// Convert secretRefDef to core.SecretRef and validate
	secrets := make([]core.SecretRef, 0, len(secretDefs))
	names := make(map[string]bool)

	for i, def := range secretDefs {
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

// buildSteps builds the steps for the DAG.
func buildSteps(ctx BuildContext, spec *definition, dag *core.DAG) error {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: dag}
	names := make(map[string]struct{})

	switch v := spec.Steps.(type) {
	case nil:
		return nil

	case []any:
		// Convert string steps to map format for shorthand syntax support
		normalized := normalizeStepData(ctx, v)

		var builtSteps []*core.Step
		var prevSteps []*core.Step
		for i, raw := range normalized {
			switch v := raw.(type) {
			case map[string]any:
				step, err := buildStepFromRaw(buildCtx, i, v, names)
				if err != nil {
					return err
				}

				injectChainDependencies(dag, prevSteps, step)
				builtSteps = append(builtSteps, step)

				// prepare for the next step
				prevSteps = []*core.Step{step}

			case []any:
				var tempSteps []*core.Step
				var normalized = normalizeStepData(ctx, v)
				for _, nested := range normalized {
					switch vv := nested.(type) {
					case map[string]any:
						step, err := buildStepFromRaw(buildCtx, i, vv, names)
						if err != nil {
							return err
						}

						injectChainDependencies(dag, prevSteps, step)

						builtSteps = append(builtSteps, step)
						tempSteps = append(tempSteps, step)

					default:
						return core.NewValidationError("steps", raw, ErrInvalidStepData)
					}
				}

				// prepare for the next step
				prevSteps = tempSteps

			default:
				return core.NewValidationError("steps", raw, ErrInvalidStepData)
			}
		}

		// Add all built steps to the DAG
		for _, step := range builtSteps {
			dag.Steps = append(dag.Steps, *step)
		}

		return nil

	case map[string]any:
		stepDefs := make(map[string]stepDef)
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepDefs,
		})
		if err := md.Decode(v); err != nil {
			return core.NewValidationError("steps", v, err)
		}
		for name, stepDef := range stepDefs {
			stepDef.Name = name
			names[stepDef.Name] = struct{}{}
			step, err := buildStep(buildCtx, stepDef)
			if err != nil {
				return err
			}
			dag.Steps = append(dag.Steps, *step)
		}

		return nil

	default:
		return core.NewValidationError("steps", v, ErrStepsMustBeArrayOrMap)

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
	var stepDef stepDef
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      &stepDef,
	})
	if err := md.Decode(raw); err != nil {
		return nil, core.NewValidationError("steps", raw, err)
	}
	step, err := buildStep(ctx, stepDef)
	if err != nil {
		return nil, err
	}
	if step.Name == "" {
		step.Name = generateTypedStepName(names, step, idx)
	}
	return step, nil
}

// buildSMTPConfig builds the SMTP configuration for the DAG.
func buildSMTPConfig(_ BuildContext, spec *definition, dag *core.DAG) (err error) {
	// Convert port to string if it's provided as a number
	var portStr string
	if spec.SMTP.Port != nil {
		switch v := spec.SMTP.Port.(type) {
		case string:
			portStr = v
		case int:
			portStr = strconv.Itoa(v)
		case float64:
			portStr = strconv.Itoa(int(v))
		default:
			if spec.SMTP.Port != "" {
				portStr = fmt.Sprintf("%v", spec.SMTP.Port)
			}
		}
	}

	if spec.SMTP.Host == "" && portStr == "" {
		return nil
	}

	dag.SMTP = &core.SMTPConfig{
		Host:     spec.SMTP.Host,
		Port:     portStr,
		Username: spec.SMTP.Username,
		Password: spec.SMTP.Password,
	}

	return nil
}

// buildErrMailConfig builds the error mail configuration for the DAG.
func buildErrMailConfig(_ BuildContext, spec *definition, dag *core.DAG) (err error) {
	dag.ErrorMail, err = buildMailConfig(spec.ErrorMail)

	return
}

// buildInfoMailConfig builds the info mail configuration for the DAG.
func buildInfoMailConfig(_ BuildContext, spec *definition, dag *core.DAG) (err error) {
	dag.InfoMail, err = buildMailConfig(spec.InfoMail)

	return
}

// buildMailConfig builds a core.MailConfig from the definition.
func buildMailConfig(def mailConfigDef) (*core.MailConfig, error) {
	// Handle case where To is not specified
	if def.From == "" && def.To == nil {
		return nil, nil
	}

	// Convert To field to []string
	var toAddresses []string
	switch v := def.To.(type) {
	case nil:
		// To field not specified
	case string:
		// Single recipient
		v = strings.TrimSpace(v)
		if v != "" {
			toAddresses = []string{v}
		}
	case []any:
		// Multiple recipients
		for _, addr := range v {
			if str, ok := addr.(string); ok {
				str = strings.TrimSpace(str)
				if str != "" {
					toAddresses = append(toAddresses, str)
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid type for 'to' field: expected string or array, got %T", v)
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

// buildStep builds a step from the step definition.
func buildStep(ctx StepBuildContext, def stepDef) (*core.Step, error) {
	step := &core.Step{
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
		if err := entry.fn(ctx, def, step); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
	}

	return step, nil
}

func buildStepWorkingDir(_ StepBuildContext, def stepDef, step *core.Step) error {
	switch {
	case def.WorkingDir != "":
		step.Dir = strings.TrimSpace(def.WorkingDir)
	case def.Dir != "":
		step.Dir = strings.TrimSpace(def.Dir)
	default:
		step.Dir = ""
	}
	return nil
}

func buildStepShell(_ StepBuildContext, def stepDef, step *core.Step) error {
	// Step shell is NOT evaluated here - it's evaluated at runtime
	shell, args, err := parseShellValue(def.Shell, false)
	if err != nil {
		return core.NewValidationError("shell", def.Shell, err)
	}
	step.Shell = shell
	step.ShellArgs = args
	return nil
}

func buildStepTimeout(_ StepBuildContext, def stepDef, step *core.Step) error {
	if def.TimeoutSec < 0 {
		return core.NewValidationError("timeoutSec", def.TimeoutSec, ErrTimeoutSecMustBeNonNegative)
	}
	if def.TimeoutSec == 0 {
		// Zero means no timeout; leave unset.
		return nil
	}
	step.Timeout = time.Second * time.Duration(def.TimeoutSec)
	return nil
}

func buildContinueOn(_ StepBuildContext, def stepDef, step *core.Step) error {
	if def.ContinueOn == nil {
		return nil
	}

	switch v := def.ContinueOn.(type) {
	case string:
		// Shorthand syntax: "skipped" or "failed"
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "skipped":
			step.ContinueOn.Skipped = true
		case "failed":
			step.ContinueOn.Failure = true
		default:
			return core.NewValidationError("continueOn", v, ErrContinueOnInvalidStringValue)
		}

	case map[string]any:
		// Object syntax with detailed configuration
		if val, exists := v["failure"]; exists {
			b, ok := val.(bool)
			if !ok {
				return core.NewValidationError("continueOn.failure", val, ErrContinueOnFieldMustBeBool)
			}
			step.ContinueOn.Failure = b
		}
		if val, exists := v["skipped"]; exists {
			b, ok := val.(bool)
			if !ok {
				return core.NewValidationError("continueOn.skipped", val, ErrContinueOnFieldMustBeBool)
			}
			step.ContinueOn.Skipped = b
		}
		if val, exists := v["markSuccess"]; exists {
			b, ok := val.(bool)
			if !ok {
				return core.NewValidationError("continueOn.markSuccess", val, ErrContinueOnFieldMustBeBool)
			}
			step.ContinueOn.MarkSuccess = b
		}

		exitCodes, err := parseIntOrArray(v["exitCode"])
		if err != nil {
			return core.NewValidationError("continueOn.exitCode", v["exitCode"], ErrContinueOnExitCodeMustBeIntOrArray)
		}
		step.ContinueOn.ExitCode = exitCodes

		output, err := parseStringOrArray(v["output"])
		if err != nil {
			return core.NewValidationError("continueOn.output", v["output"], ErrContinueOnOutputMustBeStringOrArray)
		}
		step.ContinueOn.Output = output

	default:
		return core.NewValidationError("continueOn", def.ContinueOn, ErrContinueOnMustBeStringOrMap)
	}

	return nil
}

// buildRetryPolicy builds the retry policy for a step.
func buildRetryPolicy(_ StepBuildContext, def stepDef, step *core.Step) error {
	if def.RetryPolicy != nil {
		switch v := def.RetryPolicy.Limit.(type) {
		case int:
			step.RetryPolicy.Limit = v
			step.RetryPolicy.Limit = int(v)
		case int64:
			step.RetryPolicy.Limit = int(v)
		case uint64:
			step.RetryPolicy.Limit = int(v)
		case string:
			step.RetryPolicy.LimitStr = v
		default:
			return core.NewValidationError("retryPolicy.Limit", v, fmt.Errorf("invalid type: %T", v))
		}

		switch v := def.RetryPolicy.IntervalSec.(type) {
		case int:
			step.RetryPolicy.Interval = time.Second * time.Duration(v)
		case int64:
			step.RetryPolicy.Interval = time.Second * time.Duration(v)
		case uint64:
			step.RetryPolicy.Interval = time.Second * time.Duration(v)
		case string:
			step.RetryPolicy.IntervalSecStr = v
		default:
			return core.NewValidationError("retryPolicy.IntervalSec", v, fmt.Errorf("invalid type: %T", v))
		}

		if def.RetryPolicy.ExitCode != nil {
			step.RetryPolicy.ExitCodes = def.RetryPolicy.ExitCode
		}

		// Parse backoff field
		if def.RetryPolicy.Backoff != nil {
			switch v := def.RetryPolicy.Backoff.(type) {
			case bool:
				if v {
					step.RetryPolicy.Backoff = 2.0 // Default multiplier when true
				}
			case int:
				step.RetryPolicy.Backoff = float64(v)
			case int64:
				step.RetryPolicy.Backoff = float64(v)
			case float64:
				step.RetryPolicy.Backoff = v
			default:
				return core.NewValidationError("retryPolicy.Backoff", v, fmt.Errorf("invalid type: %T", v))
			}

			// Validate backoff value
			if step.RetryPolicy.Backoff > 0 && step.RetryPolicy.Backoff <= 1.0 {
				return core.NewValidationError("retryPolicy.Backoff", step.RetryPolicy.Backoff,
					fmt.Errorf("backoff must be greater than 1.0 for exponential growth"))
			}
		}

		// Parse maxIntervalSec
		if def.RetryPolicy.MaxIntervalSec > 0 {
			step.RetryPolicy.MaxInterval = time.Second * time.Duration(def.RetryPolicy.MaxIntervalSec)
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
func buildRepeatPolicy(_ StepBuildContext, def stepDef, step *core.Step) error {
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

	step.RepeatPolicy.RepeatMode = mode
	if rpDef.IntervalSec > 0 {
		step.RepeatPolicy.Interval = time.Second * time.Duration(rpDef.IntervalSec)
	}
	step.RepeatPolicy.Limit = rpDef.Limit

	if rpDef.Condition != "" {
		step.RepeatPolicy.Condition = &core.Condition{
			Condition: rpDef.Condition,
			Expected:  rpDef.Expected,
		}
	}
	step.RepeatPolicy.ExitCode = rpDef.ExitCode

	// Parse backoff field
	if rpDef.Backoff != nil {
		switch v := rpDef.Backoff.(type) {
		case bool:
			if v {
				step.RepeatPolicy.Backoff = 2.0 // Default multiplier when true
			}
		case int:
			step.RepeatPolicy.Backoff = float64(v)
		case int64:
			step.RepeatPolicy.Backoff = float64(v)
		case float64:
			step.RepeatPolicy.Backoff = v
		default:
			return fmt.Errorf("invalid value for backoff: '%v'. It must be a boolean or number", v)
		}

		// Validate backoff value
		if step.RepeatPolicy.Backoff > 0 && step.RepeatPolicy.Backoff <= 1.0 {
			return fmt.Errorf("backoff must be greater than 1.0 for exponential growth, got: %v",
				step.RepeatPolicy.Backoff)
		}
	}

	// Parse maxIntervalSec
	if rpDef.MaxIntervalSec > 0 {
		step.RepeatPolicy.MaxInterval = time.Second * time.Duration(rpDef.MaxIntervalSec)
	}

	return nil
}

func buildOutput(_ StepBuildContext, def stepDef, step *core.Step) error {
	if def.Output == "" {
		return nil
	}

	if strings.HasPrefix(def.Output, "$") {
		step.Output = strings.TrimPrefix(def.Output, "$")
		return nil
	}

	step.Output = strings.TrimSpace(def.Output)
	return nil
}

func buildStepEnvs(ctx StepBuildContext, def stepDef, step *core.Step) error {
	if def.Env == nil {
		return nil
	}
	// For step environment variables, we load them without evaluation. They will
	// be evaluated later when the step is executed.
	ctx.opts.Flags |= BuildFlagNoEval
	vars, err := loadVariables(ctx.BuildContext, def.Env)
	if err != nil {
		return err
	}
	for k, v := range vars {
		step.Env = append(step.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}

func buildStepPrecondition(ctx StepBuildContext, def stepDef, step *core.Step) error {
	// Parse both `preconditions` and `precondition` fields.
	conditions, err := parsePrecondition(ctx.BuildContext, def.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx.BuildContext, def.Precondition)
	if err != nil {
		return err
	}
	step.Preconditions = conditions
	step.Preconditions = append(step.Preconditions, condition...)
	return nil
}

func buildSignalOnStop(_ StepBuildContext, def stepDef, step *core.Step) error {
	if def.SignalOnStop != nil {
		sigDef := *def.SignalOnStop
		sig := signal.GetSignalNum(sigDef, 0)
		if sig == 0 {
			return fmt.Errorf("%w: %s", ErrInvalidSignal, sigDef)
		}
		step.SignalOnStop = sigDef
	}
	return nil
}

// buildSubDAG parses the child core.DAG definition and sets up the step to run a sub DAG.
func buildSubDAG(ctx StepBuildContext, def stepDef, step *core.Step) error {
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

	step.SubDAG = &core.SubDAG{Name: name, Params: paramsStr}

	// Set executor type based on whether parallel execution is configured
	if step.Parallel != nil {
		step.ExecutorConfig.Type = core.ExecutorTypeParallel
	} else {
		step.ExecutorConfig.Type = core.ExecutorTypeDAG
	}

	step.Command = "call"
	step.Args = []string{name, paramsStr}
	step.CmdWithArgs = strings.TrimSpace(fmt.Sprintf("%s %s", name, paramsStr))
	return nil
}

// buildDepends parses the depends field in the step definition.
func buildDepends(_ StepBuildContext, def stepDef, step *core.Step) error {
	deps, err := parseStringOrArray(def.Depends)
	if err != nil {
		return core.NewValidationError("depends", def.Depends, ErrDependsMustBeStringOrArray)
	}
	step.Depends = deps

	// Check if depends was explicitly set to empty array
	if def.Depends != nil && len(deps) == 0 {
		step.ExplicitlyNoDeps = true
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
func buildExecutor(ctx StepBuildContext, def stepDef, step *core.Step) error {
	const (
		executorKeyType   = "type"
		executorKeyConfig = "config"
	)

	executor := def.Executor

	// Case 1: executor is nil
	if executor == nil {
		if ctx.dag.Container != nil {
			// Translate the container configuration to executor config
			return translateExecutorConfig(ctx, def, step)
		} else if ctx.dag.SSH != nil {
			return translateSSHConfig(ctx, def, step)
		}
		return nil
	}

	switch val := executor.(type) {
	case string:
		// Case 2: executor is a string
		// This can be an executor with default configuration.
		step.ExecutorConfig.Type = strings.TrimSpace(val)

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
				step.ExecutorConfig.Type = strings.TrimSpace(typ)

			case executorKeyConfig:
				// Executor config is a map of string keys and values.
				// The values can be of any type.
				// It is up to the executor to parse the values.
				executorConfig, ok := v.(map[string]any)
				if !ok {
					return core.NewValidationError("executor.config", v, ErrExecutorConfigValueMustBeMap)
				}
				for configKey, v := range executorConfig {
					step.ExecutorConfig.Config[configKey] = v
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

func translateExecutorConfig(ctx StepBuildContext, def stepDef, step *core.Step) error {
	// If the executor is nil, but the core.DAG has a container field,
	// we translate the container configuration to executor config.
	if ctx.dag.Container == nil {
		return nil // No container configuration to translate
	}

	// Translate container fields to executor config
	step.ExecutorConfig.Type = "container"

	// The other fields will be retrieved from the container configuration on
	// execution time, so we don't need to set them here.

	return nil
}

func translateSSHConfig(ctx StepBuildContext, def stepDef, step *core.Step) error {
	if ctx.dag.SSH == nil {
		return nil // No container configuration to translate
	}

	step.ExecutorConfig.Type = "ssh"

	// The other fields will be retrieved from the container configuration on
	// execution time, so we don't need to set them here.

	return nil
}

func parseIntOrArray(v any) ([]int, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case int64:
		return []int{int(v)}, nil
	case uint64:
		return []int{int(v)}, nil
	case int:
		return []int{v}, nil

	case []any:
		var ret []int
		for _, vv := range v {
			switch vv := vv.(type) {
			case int:
				ret = append(ret, vv)
			case int64:
				ret = append(ret, int(vv))
			case uint64:
				ret = append(ret, int(vv))
			default:
				return nil, fmt.Errorf("int or array expected, got %T", vv)
			}
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

// buildParallel parses the parallel field in the step definition.
// MVP supports:
// - Direct array reference: parallel: ${ITEMS}
// - Static array: parallel: [item1, item2]
// - Object configuration: parallel: {items: [...], maxConcurrent: 5}
func buildParallel(ctx StepBuildContext, def stepDef, step *core.Step) error {
	if def.Parallel == nil {
		return nil
	}

	step.Parallel = &core.ParallelConfig{
		MaxConcurrent: core.DefaultMaxConcurrent,
	}

	switch v := def.Parallel.(type) {
	case string:
		// Direct variable reference like: parallel: ${ITEMS}
		// The actual items will be resolved at runtime
		// It should be resolved to a json array of items
		// e.g. ["item1", "item2"] or [{"SOURCE": "s3://..."}]
		step.Parallel.Variable = v

	case []any:
		// Static array: parallel: [item1, item2] or parallel: [{SOURCE: s3://...}, ...]
		items, err := parseParallelItems(v)
		if err != nil {
			return core.NewValidationError("parallel", v, err)
		}
		step.Parallel.Items = items

	case map[string]any:
		// Object configuration
		for key, val := range v {
			switch key {
			case "items":
				switch itemsVal := val.(type) {
				case string:
					// Variable reference in object form
					step.Parallel.Variable = itemsVal
				case []any:
					// Direct array in object form
					items, err := parseParallelItems(itemsVal)
					if err != nil {
						return core.NewValidationError("parallel.items", itemsVal, err)
					}
					step.Parallel.Items = items
				default:
					return core.NewValidationError("parallel.items", val, fmt.Errorf("parallel.items must be string or array, got %T", val))
				}

			case "maxConcurrent":
				switch mc := val.(type) {
				case int:
					step.Parallel.MaxConcurrent = mc
				case int64:
					step.Parallel.MaxConcurrent = int(mc)
				case uint64:
					step.Parallel.MaxConcurrent = int(mc)
				case float64:
					step.Parallel.MaxConcurrent = int(mc)
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

// buildOTel builds the OpenTelemetry configuration for the DAG.
func buildOTel(_ BuildContext, spec *definition, dag *core.DAG) error {
	if spec.OTel == nil {
		return nil
	}

	switch v := spec.OTel.(type) {
	case map[string]any:
		config := &core.OTelConfig{}

		// Parse enabled flag
		if enabled, ok := v["enabled"].(bool); ok {
			config.Enabled = enabled
		}

		// Parse endpoint
		if endpoint, ok := v["endpoint"].(string); ok {
			config.Endpoint = endpoint
		}

		// Parse headers
		if headers, ok := v["headers"].(map[string]any); ok {
			config.Headers = make(map[string]string)
			for key, val := range headers {
				if strVal, ok := val.(string); ok {
					config.Headers[key] = strVal
				}
			}
		}

		// Parse insecure flag
		if insecure, ok := v["insecure"].(bool); ok {
			config.Insecure = insecure
		}

		// Parse timeout
		if timeout, ok := v["timeout"].(string); ok {
			duration, err := time.ParseDuration(timeout)
			if err != nil {
				return core.NewValidationError("otel.timeout", timeout, err)
			}
			config.Timeout = duration
		}

		// Parse resource attributes
		if resource, ok := v["resource"].(map[string]any); ok {
			config.Resource = resource
		}

		dag.OTel = config
		return nil

	default:
		return core.NewValidationError("otel", v, fmt.Errorf("otel must be a map"))
	}
}
