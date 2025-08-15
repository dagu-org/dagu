package digraph

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
)

// BuilderFn is a function that builds a part of the DAG.
type BuilderFn func(ctx BuildContext, spec *definition, dag *DAG) error

// BuildContext is the context for building a DAG.
type BuildContext struct {
	ctx  context.Context
	file string
	opts BuildOpts
}

// StepBuildContext is the context for building a step.
type StepBuildContext struct {
	BuildContext
	dag *DAG
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

// BuildOpts is used to control the behavior of the builder.
type BuildOpts struct {
	// Base specifies the Base configuration file for the DAG.
	Base string
	// OnlyMetadata specifies whether to build only the metadata.
	OnlyMetadata bool
	// Parameters specifies the Parameters to the DAG.
	// Parameters are used to override the default Parameters in the DAG.
	Parameters string
	// ParametersList specifies the parameters to the DAG.
	ParametersList []string
	// NoEval specifies whether to evaluate dynamic fields.
	NoEval bool
	// Name of the DAG if it's not defined in the spec
	Name string
	// DAGsDir is the directory containing the DAG files.
	DAGsDir string
	// AllowBuildErrors specifies whether to allow build errors.
	AllowBuildErrors bool
}

var builderRegistry = []builderEntry{
	{metadata: true, name: "env", fn: buildEnvs},
	{metadata: true, name: "schedule", fn: buildSchedule},
	{metadata: true, name: "skipIfSuccessful", fn: skipIfSuccessful},
	{metadata: true, name: "params", fn: buildParams},
	{metadata: true, name: "name", fn: buildName},
	{metadata: true, name: "type", fn: buildType},
	{metadata: true, name: "runConfig", fn: buildRunConfig},
	{name: "container", fn: buildContainer},
	{name: "registryAuths", fn: buildRegistryAuths},
	{name: "ssh", fn: buildSSH},
	{name: "dotenv", fn: buildDotenv},
	{name: "mailOn", fn: buildMailOn},
	{name: "logDir", fn: buildLogDir},
	{name: "handlers", fn: buildHandlers},
	{name: "smtpConfig", fn: buildSMTPConfig},
	{name: "errMailConfig", fn: buildErrMailConfig},
	{name: "infoMailConfig", fn: buildInfoMailConfig},
	{name: "maxHistoryRetentionDays", fn: maxHistoryRetentionDays},
	{name: "maxCleanUpTime", fn: maxCleanUpTime},
	{name: "maxActiveRuns", fn: buildMaxActiveRuns},
	{name: "preconditions", fn: buildPrecondition},
	{name: "otel", fn: buildOTel},
	{name: "steps", fn: buildSteps},
	{name: "validateSteps", fn: validateSteps},
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
	{name: "parallel", fn: buildParallel}, // Must be before childDAG to set executor type correctly
	{name: "childDAG", fn: buildChildDAG},
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
type StepBuilderFn func(ctx StepBuildContext, def stepDef, step *Step) error

// build builds a DAG from the specification.
func build(ctx BuildContext, spec *definition) (*DAG, error) {
	dag := &DAG{
		Location:       ctx.file,
		Name:           spec.Name,
		Group:          spec.Group,
		Description:    spec.Description,
		Type:           spec.Type,
		Timeout:        time.Second * time.Duration(spec.TimeoutSec),
		Delay:          time.Second * time.Duration(spec.DelaySec),
		RestartWait:    time.Second * time.Duration(spec.RestartWaitSec),
		Tags:           parseTags(spec.Tags),
		MaxActiveSteps: spec.MaxActiveSteps,
		Queue:          spec.Queue,
		MaxOutputSize:  spec.MaxOutputSize,
		WorkerSelector: spec.WorkerSelector,
	}

	var errs ErrorList
	for _, builder := range builderRegistry {
		if !builder.metadata && ctx.opts.OnlyMetadata {
			continue
		}
		if err := builder.fn(ctx, spec, dag); err != nil {
			errs.Add(wrapError(builder.name, nil, err))
		}
	}

	if len(errs) > 0 {
		if ctx.opts.AllowBuildErrors {
			// If we are allowing build errors, return the DAG with the errors.
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
				return wrapError("schedule", s, ErrScheduleMustBeStringOrArray)
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
		return wrapError("schedule", spec.Schedule, ErrInvalidScheduleType)

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

func buildContainer(ctx BuildContext, spec *definition, dag *DAG) error {
	if spec.Container == nil {
		return nil
	}

	// Validate required fields
	if spec.Container.Image == "" {
		return wrapError("container.image", spec.Container.Image, fmt.Errorf("image is required when container is specified"))
	}

	pullPolicy, err := ParsePullPolicy(spec.Container.PullPolicy)
	if err != nil {
		return wrapError("container.pullPolicy", spec.Container.PullPolicy, err)
	}

	vars, err := loadVariables(ctx, spec.Container.Env)
	if err != nil {
		return wrapError("container.env", spec.Container.Env, err)
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	container := Container{
		Image:         spec.Container.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       spec.Container.Volumes,
		User:          spec.Container.User,
		WorkDir:       spec.Container.WorkDir,
		Platform:      spec.Container.Platform,
		Ports:         spec.Container.Ports,
		Network:       spec.Container.Network,
		KeepContainer: spec.Container.KeepContainer,
	}

	dag.Container = &container

	return nil
}

func buildRegistryAuths(ctx BuildContext, spec *definition, dag *DAG) error {
	if spec.RegistryAuths == nil {
		return nil
	}

	dag.RegistryAuths = make(map[string]*AuthConfig)

	switch v := spec.RegistryAuths.(type) {
	case string:
		// Handle as a JSON string (e.g., from environment variable)
		expandedJSON := v
		if !ctx.opts.NoEval {
			expandedJSON = os.ExpandEnv(v)
		}

		// For now, store the entire JSON string as a single entry
		// The actual parsing will be done by the registry manager
		dag.RegistryAuths["_json"] = &AuthConfig{
			Auth: expandedJSON,
		}

	case map[string]any:
		// Handle as a map of registry to auth config
		for registry, authData := range v {
			authConfig := &AuthConfig{}

			switch auth := authData.(type) {
			case string:
				// Simple string value - treat as JSON auth config
				if !ctx.opts.NoEval {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
				// Auth config object with username/password fields
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.NoEval {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.NoEval {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.NoEval {
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

			authConfig := &AuthConfig{}

			switch auth := authData.(type) {
			case string:
				// Simple string value - treat as JSON auth config
				if !ctx.opts.NoEval {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
				// Auth config object with username/password fields
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.NoEval {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.NoEval {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.NoEval {
						authConfig.Auth = os.ExpandEnv(authConfig.Auth)
					}
				}

			case map[any]any:
				// Handle nested YAML map type
				if username, ok := auth["username"].(string); ok {
					authConfig.Username = username
					if !ctx.opts.NoEval {
						authConfig.Username = os.ExpandEnv(authConfig.Username)
					}
				}

				if password, ok := auth["password"].(string); ok {
					authConfig.Password = password
					if !ctx.opts.NoEval {
						authConfig.Password = os.ExpandEnv(authConfig.Password)
					}
				}

				if authStr, ok := auth["auth"].(string); ok {
					authConfig.Auth = authStr
					if !ctx.opts.NoEval {
						authConfig.Auth = os.ExpandEnv(authConfig.Auth)
					}
				}
			}

			dag.RegistryAuths[registry] = authConfig
		}

	default:
		return wrapError("registryAuths", spec.RegistryAuths, fmt.Errorf("invalid type: %T", spec.RegistryAuths))
	}

	return nil
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
				return wrapError("dotenv", e, ErrDotEnvMustBeStringOrArray)
			}
		}
	default:
		return wrapError("dotenv", v, ErrDotEnvMustBeStringOrArray)
	}

	if !ctx.opts.NoEval {
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
			if err := godotenv.Overload(resolvedPath); err != nil {
				return wrapError("dotenv", filePath, fmt.Errorf("failed to load dotenv file %s: %w", filePath, err))
			}
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

// buildName set the name if name is specified by the option but if Name is defined
// it does not override
func buildName(ctx BuildContext, spec *definition, dag *DAG) error {
	if spec.Name != "" {
		return nil
	}

	dag.Name = ctx.opts.Name

	// Validate the name
	if dag.Name == "" {
		return nil
	}

	if len(dag.Name) > maxNameLen {
		return wrapError("name", dag.Name, ErrNameTooLong)
	}
	if !regexName.MatchString(dag.Name) {
		return wrapError("name", dag.Name, ErrNameInvalidChars)
	}

	return nil
}

// buildType validates and sets the execution type for the DAG
func buildType(_ BuildContext, spec *definition, dag *DAG) error {
	// Set default if not specified
	if dag.Type == "" {
		dag.Type = TypeChain
	}

	// Validate the type
	switch dag.Type {
	case TypeGraph, TypeChain:
		// Valid types
		return nil
	case TypeAgent:
		return wrapError("type", dag.Type, fmt.Errorf("agent type is not yet implemented"))
	default:
		return wrapError("type", dag.Type, fmt.Errorf("invalid type: %s (must be one of: graph, chain, agent)", dag.Type))
	}
}

// regexName is a regular expression that matches valid names.
// It allows alphanumeric characters, underscores, hyphens, and dots.
var regexName = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// maxNameLen is the maximum length of a name.
var maxNameLen = 40

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
	buildCtx := StepBuildContext{BuildContext: ctx, dag: dag}

	if spec.HandlerOn.Exit != nil {
		spec.HandlerOn.Exit.Name = HandlerOnExit.String()
		if dag.HandlerOn.Exit, err = buildStep(buildCtx, *spec.HandlerOn.Exit); err != nil {
			return err
		}
	}

	if spec.HandlerOn.Success != nil {
		spec.HandlerOn.Success.Name = HandlerOnSuccess.String()
		if dag.HandlerOn.Success, err = buildStep(buildCtx, *spec.HandlerOn.Success); err != nil {
			return
		}
	}

	if spec.HandlerOn.Failure != nil {
		spec.HandlerOn.Failure.Name = HandlerOnFailure.String()
		if dag.HandlerOn.Failure, err = buildStep(buildCtx, *spec.HandlerOn.Failure); err != nil {
			return
		}
	}

	if spec.HandlerOn.Cancel != nil {
		spec.HandlerOn.Cancel.Name = HandlerOnCancel.String()
		if dag.HandlerOn.Cancel, err = buildStep(buildCtx, *spec.HandlerOn.Cancel); err != nil {
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

func parsePrecondition(ctx BuildContext, precondition any) ([]*Condition, error) {
	switch v := precondition.(type) {
	case nil:
		return nil, nil

	case string:
		return []*Condition{{Condition: v}}, nil

	case map[string]any:
		var ret Condition
		for key, vv := range v {
			switch strings.ToLower(key) {
			case "condition":
				val, ok := vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			case "expected":
				val, ok := vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Expected = val

			case "command":
				val, ok := vv.(string)
				if !ok {
					return nil, wrapError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			default:
				return nil, wrapError("preconditions", key, fmt.Errorf("%w: %s", ErrPreconditionHasInvalidKey, key))

			}
		}

		if err := ret.Validate(); err != nil {
			return nil, wrapError("preconditions", v, err)
		}

		return []*Condition{&ret}, nil

	case []any:
		var ret []*Condition
		for _, vv := range v {
			parsed, err := parsePrecondition(ctx, vv)
			if err != nil {
				return nil, err
			}
			ret = append(ret, parsed...)
		}
		return ret, nil

	default:
		return nil, wrapError("preconditions", v, ErrPreconditionMustBeArrayOrString)

	}
}

func maxCleanUpTime(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.MaxCleanUpTimeSec != nil {
		dag.MaxCleanUpTime = time.Second * time.Duration(*spec.MaxCleanUpTimeSec)
	}
	return nil
}

func buildMaxActiveRuns(_ BuildContext, spec *definition, dag *DAG) error {
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

// buildRunConfig builds the run configuration for the DAG.
func buildRunConfig(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.RunConfig == nil {
		return nil
	}
	dag.RunConfig = &RunConfig{
		DisableParamEdit: spec.RunConfig.DisableParamEdit,
		DisableRunIdEdit: spec.RunConfig.DisableRunIdEdit,
	}
	return nil
}

// buildSSH builds the SSH configuration for the DAG.
func buildSSH(_ BuildContext, spec *definition, dag *DAG) error {
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

	dag.SSH = &SSHConfig{
		User:          spec.SSH.User,
		Host:          spec.SSH.Host,
		Port:          port,
		Key:           spec.SSH.Key,
		StrictHostKey: strictHostKey,
		KnownHostFile: spec.SSH.KnownHostFile,
	}

	return nil
}

// generateTypedStepName generates a type-based name for a step after it's been built
func generateTypedStepName(existingNames map[string]struct{}, step *Step, index int) string {
	var prefix string

	// Determine prefix based on the built step's properties
	if step.ExecutorConfig.Type != "" {
		switch step.ExecutorConfig.Type {
		case ExecutorTypeDAG, ExecutorTypeDAGLegacy:
			prefix = "dag"
		case ExecutorTypeParallel:
			prefix = "parallel"
		case "http":
			prefix = "http"
		case "docker":
			prefix = "container"
		case "ssh":
			prefix = "ssh"
		case "mail":
			prefix = "mail"
		case "jq":
			prefix = "jq"
		default:
			prefix = "exec"
		}
	} else if step.Parallel != nil {
		prefix = "parallel"
	} else if step.ChildDAG != nil {
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
func buildSteps(ctx BuildContext, spec *definition, dag *DAG) error {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: dag}
	existingNames := make(map[string]struct{})

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

		var builtSteps []*Step
		for i, stepDef := range stepDefs {
			step, err := buildStep(buildCtx, stepDef)
			if err != nil {
				return err
			}
			if step.Name == "" {
				step.Name = generateTypedStepName(existingNames, step, i)
			}
			if err := validateStep(buildCtx, stepDef, step); err != nil {
				return err
			}
			builtSteps = append(builtSteps, step)
		}

		// Add all built steps to the DAG
		for _, step := range builtSteps {
			dag.Steps = append(dag.Steps, *step)
		}

		// Inject chain dependencies if type is chain
		injectChainDependencies(dag)

		return nil

	case map[string]any:
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
			existingNames[stepDef.Name] = struct{}{}
			step, err := buildStep(buildCtx, stepDef)
			if err != nil {
				return err
			}
			dag.Steps = append(dag.Steps, *step)
		}

		// Inject chain dependencies if type is chain
		injectChainDependencies(dag)

		return nil

	default:
		return wrapError("steps", v, ErrStepsMustBeArrayOrMap)

	}
}

// stepIDPattern defines the valid format for step IDs
var stepIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// isValidStepID checks if the given ID matches the required pattern
func isValidStepID(id string) bool {
	return stepIDPattern.MatchString(id)
}

// isReservedWord checks if the given ID is a reserved word
func isReservedWord(id string) bool {
	reservedWords := map[string]bool{
		"env":     true,
		"params":  true,
		"args":    true,
		"stdout":  true,
		"stderr":  true,
		"output":  true,
		"outputs": true,
	}
	return reservedWords[strings.ToLower(id)]
}

// validateSteps validates the steps in the DAG.
func validateSteps(ctx BuildContext, spec *definition, dag *DAG) error {
	// First pass: collect all names and IDs
	stepNames := make(map[string]struct{})
	stepIDs := make(map[string]struct{})

	for _, step := range dag.Steps {
		// Names should always exist at this point (explicit or auto-generated)
		if step.Name == "" {
			// This should not happen if generation works correctly
			return wrapError("steps", step, fmt.Errorf("internal error: step name not generated"))
		}

		if _, exists := stepNames[step.Name]; exists {
			return wrapError("steps", step.Name, ErrStepNameDuplicate)
		}
		stepNames[step.Name] = struct{}{}

		// Collect IDs if present
		if step.ID != "" {
			// Check ID format
			if !isValidStepID(step.ID) {
				return wrapError("steps", step.ID, fmt.Errorf("invalid step ID format: must match pattern ^[a-zA-Z][a-zA-Z0-9_-]*$"))
			}

			// Check for duplicate IDs
			if _, exists := stepIDs[step.ID]; exists {
				return wrapError("steps", step.ID, fmt.Errorf("duplicate step ID: %s", step.ID))
			}
			stepIDs[step.ID] = struct{}{}

			// Check for reserved words
			if isReservedWord(step.ID) {
				return wrapError("steps", step.ID, fmt.Errorf("step ID '%s' is a reserved word", step.ID))
			}
		}
	}

	// Second pass: check for conflicts between names and IDs
	for _, step := range dag.Steps {
		if step.ID != "" {
			// Check that ID doesn't conflict with any step name
			if _, exists := stepNames[step.ID]; exists && step.ID != step.Name {
				return wrapError("steps", step.ID, fmt.Errorf("step ID '%s' conflicts with another step's name", step.ID))
			}
		}

		// Check that name doesn't conflict with any ID (unless it's the same step)
		if _, exists := stepIDs[step.Name]; exists {
			// Find if this is the same step
			sameStep := false
			for _, s := range dag.Steps {
				if s.Name == step.Name && s.ID == step.Name {
					sameStep = true
					break
				}
			}
			if !sameStep {
				return wrapError("steps", step.Name, fmt.Errorf("step name '%s' conflicts with another step's ID", step.Name))
			}
		}
	}

	// Third pass: resolve step IDs to names in depends fields
	if err := resolveStepDependencies(dag); err != nil {
		return err
	}

	// Fourth pass: validate dependencies exist
	for _, step := range dag.Steps {
		for _, dep := range step.Depends {
			if _, exists := stepNames[dep]; !exists {
				return wrapError("depends", dep, fmt.Errorf("step %s depends on non-existent step %s", step.Name, dep))
			}
		}
	}

	return nil
}

// resolveStepDependencies resolves step IDs to step names in the depends field
func resolveStepDependencies(dag *DAG) error {
	// Build a map from ID to step name for quick lookup
	idToName := make(map[string]string)
	for i := range dag.Steps {
		step := &dag.Steps[i]
		if step.ID != "" {
			idToName[step.ID] = step.Name
		}
	}

	// Resolve dependencies for each step
	for i := range dag.Steps {
		step := &dag.Steps[i]
		for j, dep := range step.Depends {
			// Check if this dependency is an ID that needs to be resolved
			if name, exists := idToName[dep]; exists {
				// Replace the ID with the actual step name
				step.Depends[j] = name
			}
			// If not found in idToName, it's either already a name or will be caught
			// as an error during dependency validation
		}
	}

	return nil
}

// buildSMTPConfig builds the SMTP configuration for the DAG.
func buildSMTPConfig(_ BuildContext, spec *definition, dag *DAG) (err error) {
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

	dag.SMTP = &SMTPConfig{
		Host:     spec.SMTP.Host,
		Port:     portStr,
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
		if v != "" {
			toAddresses = []string{v}
		}
	case []any:
		// Multiple recipients
		for _, addr := range v {
			if str, ok := addr.(string); ok && str != "" {
				toAddresses = append(toAddresses, str)
			}
		}
	default:
		return nil, fmt.Errorf("invalid type for 'to' field: expected string or array, got %T", v)
	}

	// Return nil if no valid configuration
	if def.From == "" && len(toAddresses) == 0 {
		return nil, nil
	}

	return &MailConfig{
		From:       def.From,
		To:         toAddresses,
		Prefix:     def.Prefix,
		AttachLogs: def.AttachLogs,
	}, nil
}

// buildStep builds a step from the step definition.
func buildStep(ctx StepBuildContext, def stepDef) (*Step, error) {
	step := &Step{
		Name:           def.Name,
		ID:             def.ID,
		Description:    def.Description,
		Shell:          def.Shell,
		ShellPackages:  def.ShellPackages,
		Script:         def.Script,
		Stdout:         def.Stdout,
		Stderr:         def.Stderr,
		Dir:            def.Dir,
		MailOnError:    def.MailOnError,
		ExecutorConfig: ExecutorConfig{Config: make(map[string]any)},
	}

	for _, entry := range stepBuilderRegistry {
		if err := entry.fn(ctx, def, step); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
	}

	return step, nil
}

func buildContinueOn(_ StepBuildContext, def stepDef, step *Step) error {
	if def.ContinueOn == nil {
		return nil
	}
	step.ContinueOn.Skipped = def.ContinueOn.Skipped
	step.ContinueOn.Failure = def.ContinueOn.Failure
	step.ContinueOn.MarkSuccess = def.ContinueOn.MarkSuccess

	exitCodes, err := parseIntOrArray(def.ContinueOn.ExitCode)
	if err != nil {
		return wrapError("continueOn.exitCode", def.ContinueOn.ExitCode, ErrContinueOnExitCodeMustBeIntOrArray)
	}
	step.ContinueOn.ExitCode = exitCodes

	output, err := parseStringOrArray(def.ContinueOn.Output)
	if err != nil {
		return wrapError("continueOn.stdout", def.ContinueOn.Output, ErrContinueOnOutputMustBeStringOrArray)
	}
	step.ContinueOn.Output = output

	return nil
}

// buildRetryPolicy builds the retry policy for a step.
func buildRetryPolicy(_ StepBuildContext, def stepDef, step *Step) error {
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
			return wrapError("retryPolicy.Limit", v, fmt.Errorf("invalid type: %T", v))
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
			return wrapError("retryPolicy.IntervalSec", v, fmt.Errorf("invalid type: %T", v))
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
				return wrapError("retryPolicy.Backoff", v, fmt.Errorf("invalid type: %T", v))
			}

			// Validate backoff value
			if step.RetryPolicy.Backoff > 0 && step.RetryPolicy.Backoff <= 1.0 {
				return wrapError("retryPolicy.Backoff", step.RetryPolicy.Backoff,
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
func buildRepeatPolicy(_ StepBuildContext, def stepDef, step *Step) error {
	if def.RepeatPolicy == nil {
		return nil
	}
	rpDef := def.RepeatPolicy

	// Determine repeat mode
	var mode RepeatMode
	if rpDef.Repeat != nil {
		switch v := rpDef.Repeat.(type) {
		case bool:
			if v {
				mode = RepeatModeWhile
			}
		case string:
			switch v {
			case "while":
				mode = RepeatModeWhile
			case "until":
				mode = RepeatModeUntil
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
			mode = RepeatModeUntil
		} else if rpDef.Condition != "" || len(rpDef.ExitCode) > 0 {
			mode = RepeatModeWhile
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
		step.RepeatPolicy.Condition = &Condition{
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

func buildOutput(_ StepBuildContext, def stepDef, step *Step) error {
	if def.Output == "" {
		return nil
	}

	if strings.HasPrefix(def.Output, "$") {
		step.Output = strings.TrimPrefix(def.Output, "$")
		return nil
	}

	step.Output = def.Output
	return nil
}

func buildStepEnvs(ctx StepBuildContext, def stepDef, step *Step) error {
	if def.Env == nil {
		return nil
	}
	// For step environment variables, we load them without evaluation. They will
	// be evaluated later when the step is executed.
	ctx.opts.NoEval = true
	vars, err := loadVariables(ctx.BuildContext, def.Env)
	if err != nil {
		return err
	}
	for k, v := range vars {
		step.Env = append(step.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}

func validateStep(_ StepBuildContext, def stepDef, step *Step) error {
	if step.Name == "" {
		return wrapError("name", step.Name, ErrStepNameRequired)
	}

	if len(step.Name) > maxStepNameLen {
		return wrapError("name", step.Name, ErrStepNameTooLong)
	}

	// TODO: Validate executor config for each executor type.

	if step.Command == "" {
		if step.ExecutorConfig.Type == "" && step.Script == "" && step.ChildDAG == nil {
			return ErrStepCommandIsRequired
		}
	}

	// Validate parallel configuration
	if step.Parallel != nil {
		// Parallel steps must have a run field (child-DAG only for MVP)
		if step.ChildDAG == nil {
			return wrapError("parallel", step.Parallel, fmt.Errorf("parallel execution is only supported for child-DAGs (must have 'run' field)"))
		}

		// MaxConcurrent must be positive
		if step.Parallel.MaxConcurrent <= 0 {
			return wrapError("parallel.maxConcurrent", step.Parallel.MaxConcurrent, fmt.Errorf("maxConcurrent must be greater than 0"))
		}

		// Must have either items or variable reference
		if len(step.Parallel.Items) == 0 && step.Parallel.Variable == "" {
			return wrapError("parallel", step.Parallel, fmt.Errorf("parallel must have either items array or variable reference"))
		}
	}

	return nil
}

// maxStepNameLen is the maximum length of a step name.
const maxStepNameLen = 40

func buildStepPrecondition(ctx StepBuildContext, def stepDef, step *Step) error {
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

func buildSignalOnStop(_ StepBuildContext, def stepDef, step *Step) error {
	if def.SignalOnStop != nil {
		sigDef := *def.SignalOnStop
		sig := getSignalNum(sigDef)
		if sig == 0 {
			return fmt.Errorf("%w: %s", ErrInvalidSignal, sigDef)
		}
		step.SignalOnStop = sigDef
	}
	return nil
}

// buildChildDAG parses the child DAG definition and sets up the step to run a child DAG.
func buildChildDAG(ctx StepBuildContext, def stepDef, step *Step) error {
	name := def.Run

	// if the run field is not set, return nil.
	if name == "" {
		return nil
	}

	// Parse params similar to how DAG params are parsed
	var paramsStr string
	if def.Params != nil {
		// Parse the params to convert them to string format
		ctxCopy := ctx
		ctxCopy.opts.NoEval = true // Disable evaluation for params parsing
		paramPairs, err := parseParamValue(ctxCopy.BuildContext, def.Params)
		if err != nil {
			return wrapError("params", def.Params, err)
		}

		// Convert to string format "key=value key=value ..."
		var paramsToJoin []string
		for _, paramPair := range paramPairs {
			paramsToJoin = append(paramsToJoin, paramPair.Escaped())
		}
		paramsStr = strings.Join(paramsToJoin, " ")
	}

	step.ChildDAG = &ChildDAG{Name: name, Params: paramsStr}

	// Set executor type based on whether parallel execution is configured
	if step.Parallel != nil {
		step.ExecutorConfig.Type = ExecutorTypeParallel
	} else {
		step.ExecutorConfig.Type = ExecutorTypeDAG
	}

	step.Command = "run"
	step.Args = []string{name, paramsStr}
	step.CmdWithArgs = fmt.Sprintf("%s %s", name, paramsStr)
	return nil
}

// buildDepends parses the depends field in the step definition.
func buildDepends(_ StepBuildContext, def stepDef, step *Step) error {
	deps, err := parseStringOrArray(def.Depends)
	if err != nil {
		return wrapError("depends", def.Depends, ErrDependsMustBeStringOrArray)
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
//	Case 1.1: DAG level 'container' field is set
//	Case 1.2: DAG 'ssh' field is set
//	Case 1.3: No executor is set, use default executor
//
// Case 2: executor is a string
// Case 3: executor is a struct
func buildExecutor(ctx StepBuildContext, def stepDef, step *Step) error {
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
		step.ExecutorConfig.Type = val

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
					return wrapError("executor.type", v, ErrExecutorTypeMustBeString)
				}
				step.ExecutorConfig.Type = typ

			case executorKeyConfig:
				// Executor config is a map of string keys and values.
				// The values can be of any type.
				// It is up to the executor to parse the values.
				executorConfig, ok := v.(map[string]any)
				if !ok {
					return wrapError("executor.config", v, ErrExecutorConfigValueMustBeMap)
				}
				for configKey, v := range executorConfig {
					step.ExecutorConfig.Config[configKey] = v
				}

			default:
				// Unknown key in the executor config.
				return wrapError("executor.config", key, fmt.Errorf("%w: %s", ErrExecutorHasInvalidKey, key))

			}
		}

	default:
		// Unknown key for executor field.
		return wrapError("executor", val, ErrExecutorConfigMustBeStringOrMap)

	}

	return nil
}

func translateExecutorConfig(ctx StepBuildContext, def stepDef, step *Step) error {
	// If the executor is nil, but the DAG has a container field,
	// we translate the container configuration to executor config.
	if ctx.dag.Container == nil {
		return nil // No container configuration to translate
	}

	// Translate container fields to executor config
	step.ExecutorConfig.Type = "docker"

	// The other fields will be retrieved from the container configuration on
	// execution time, so we don't need to set them here.

	return nil
}

func translateSSHConfig(ctx StepBuildContext, def stepDef, step *Step) error {
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
func buildParallel(ctx StepBuildContext, def stepDef, step *Step) error {
	if def.Parallel == nil {
		return nil
	}

	step.Parallel = &ParallelConfig{
		MaxConcurrent: DefaultMaxConcurrent,
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
			return wrapError("parallel", v, err)
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
						return wrapError("parallel.items", itemsVal, err)
					}
					step.Parallel.Items = items
				default:
					return wrapError("parallel.items", val, fmt.Errorf("parallel.items must be string or array, got %T", val))
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
					return wrapError("parallel.maxConcurrent", val, fmt.Errorf("parallel.maxConcurrent must be int, got %T", val))
				}

			default:
				// Ignore unknown keys for now (future extensibility)
			}
		}

	default:
		return wrapError("parallel", v, fmt.Errorf("parallel must be string, array, or object, got %T", v))
	}

	return nil
}

// parseParallelItems converts an array of any type to ParallelItem slice
func parseParallelItems(items []any) ([]ParallelItem, error) {
	var result []ParallelItem

	for _, item := range items {
		switch v := item.(type) {
		case string:
			// Simple string item
			result = append(result, ParallelItem{Value: v})

		case int, int64, uint64, float64:
			// Numeric items, convert to string
			result = append(result, ParallelItem{Value: fmt.Sprintf("%v", v)})

		case map[string]any:
			// Object with parameters
			params := make(DeterministicMap)
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
			result = append(result, ParallelItem{Params: params})

		default:
			return nil, fmt.Errorf("parallel items must be strings, numbers, or objects, got %T", v)
		}
	}

	return result, nil
}

// injectChainDependencies adds implicit dependencies for chain type execution
func injectChainDependencies(dag *DAG) {
	// Only inject dependencies for chain type
	if dag.Type != TypeChain {
		return
	}

	// Need at least 2 steps to create a chain
	if len(dag.Steps) < 2 {
		return
	}

	// For each step starting from the second one
	for i := 1; i < len(dag.Steps); i++ {
		step := &dag.Steps[i]
		prevStep := &dag.Steps[i-1]

		// Only add implicit dependency if the step doesn't already have dependencies
		// and wasn't explicitly set to have no dependencies
		if len(step.Depends) == 0 && !step.ExplicitlyNoDeps {
			step.Depends = []string{prevStep.Name}
		}
	}
}

// buildOTel builds the OpenTelemetry configuration for the DAG.
func buildOTel(_ BuildContext, spec *definition, dag *DAG) error {
	if spec.OTel == nil {
		return nil
	}

	switch v := spec.OTel.(type) {
	case map[string]any:
		config := &OTelConfig{}

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
				return wrapError("otel.timeout", timeout, err)
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
		return wrapError("otel", v, fmt.Errorf("otel must be a map"))
	}
}
