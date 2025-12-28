package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/go-viper/mapstructure/v2"
)

// dag is the intermediate representation of a DAG specification.
// It mirrors the YAML structure and gets validated/transformed into core.DAG.
type dag struct {
	// Name is the name of the DAG.
	Name string
	// Group is the group of the DAG for grouping DAGs on the UI.
	Group string
	// Description is the description of the DAG.
	Description string
	// Type is the execution type for steps (graph, chain, or agent).
	// Default is "graph" which uses dependency-based execution.
	// "chain" executes steps in the order they are defined.
	// "agent" is reserved for future agent-based execution.
	Type string
	// Shell is the default shell to use for all steps in this DAG.
	// If not specified, the system default shell is used.
	// Can be overridden at the step level.
	// Can be a string (e.g., "bash -e") or an array (e.g., ["bash", "-e"]).
	Shell types.ShellValue
	// WorkingDir is working directory for DAG execution
	WorkingDir string
	// Dotenv is the path to the dotenv file (string or []string).
	Dotenv types.StringOrArray
	// Schedule is the cron schedule to run the DAG.
	Schedule types.ScheduleValue
	// SkipIfSuccessful is the flag to skip the DAG on schedule when it is
	// executed manually before the schedule.
	SkipIfSuccessful bool
	// LogDir is the directory where the logs are stored.
	LogDir string
	// LogOutput specifies how stdout and stderr are handled in log files.
	// Can be "separate" (default) for separate .out and .err files,
	// or "merged" for a single combined .log file.
	LogOutput types.LogOutputValue
	// Env is the environment variables setting.
	Env types.EnvValue
	// HandlerOn is the handler configuration.
	HandlerOn handlerOn
	// Steps is the list of steps to run.
	Steps any // []step or map[string]step
	// SMTP is the SMTP configuration.
	SMTP smtpConfig
	// MailOn is the mail configuration.
	MailOn *mailOn
	// ErrorMail is the mail configuration for error.
	ErrorMail mailConfig
	// InfoMail is the mail configuration for information.
	InfoMail mailConfig
	// TimeoutSec is the timeout in seconds to finish the DAG.
	TimeoutSec int
	// DelaySec is the delay in seconds to start the first node.
	DelaySec int
	// RestartWaitSec is the wait in seconds to when the DAG is restarted.
	RestartWaitSec int
	// HistRetentionDays is the retention days of the dag-runs history.
	HistRetentionDays *int
	// Precondition is the condition to run the DAG.
	Precondition any
	// Preconditions is the condition to run the DAG.
	Preconditions any
	// maxActiveRuns is the maximum number of concurrent dag-runs.
	MaxActiveRuns int
	// MaxActiveSteps is the maximum number of concurrent steps.
	MaxActiveSteps int
	// Params is the default parameters for the steps.
	Params any
	// MaxCleanUpTimeSec is the maximum time in seconds to clean up the DAG.
	// It is a wait time to kill the processes when it is requested to stop.
	// If the time is exceeded, the process is killed.
	MaxCleanUpTimeSec *int
	// Tags is the tags for the DAG.
	Tags types.StringOrArray
	// Queue is the name of the queue to assign this DAG to.
	Queue string
	// MaxOutputSize is the maximum size of the output for each step.
	MaxOutputSize int
	// OTel is the OpenTelemetry configuration.
	OTel any
	// WorkerSelector specifies required worker labels for execution.
	WorkerSelector map[string]string
	// Container is the container definition for the DAG.
	Container *container
	// RunConfig contains configuration for controlling user interactions during DAG runs.
	RunConfig *runConfig
	// RegistryAuths maps registry hostnames to authentication configs.
	// Can be either a JSON string or a map of registry to auth config.
	RegistryAuths any
	// SSH is the default SSH configuration for the DAG.
	SSH *ssh
	// Secrets contains references to external secrets.
	Secrets []secretRef
}

// handlerOn defines the steps to be executed on different events.
type handlerOn struct {
	Init    *step // Step to execute before steps (after preconditions pass)
	Failure *step // Step to execute on failure
	Success *step // Step to execute on success
	Abort   *step // Step to execute on abort (canonical field)
	Cancel  *step // Step to execute on cancel (deprecated: use Abort instead)
	Exit    *step // Step to execute on exit
}

// smtpConfig defines the SMTP configuration.
type smtpConfig struct {
	Host     string          // SMTP host
	Port     types.PortValue // SMTP port (can be string or number)
	Username string          // SMTP username
	Password string          // SMTP password
}

// mailConfig defines the mail configuration.
type mailConfig struct {
	From       string              // Sender email address
	To         types.StringOrArray // Recipient email address(es) - can be string or []string
	Prefix     string              // Prefix for the email subject
	AttachLogs bool                // Flag to attach logs to the email
}

// mailOn defines the conditions to send mail.
type mailOn struct {
	Failure bool // Send mail on failure
	Success bool // Send mail on success
}

// container defines the container configuration for the DAG.
type container struct {
	// Name is the container name to use. If empty, Docker generates a random name.
	Name string `yaml:"name,omitempty"`
	// Image is the container image to use.
	Image string `yaml:"image,omitempty"`
	// PullPolicy is the policy to pull the image (e.g., "Always", "IfNotPresent").
	PullPolicy any `yaml:"pullPolicy,omitempty"`
	// Env specifies environment variables for the container.
	Env any `yaml:"env,omitempty"` // Can be a map or struct
	// Volumes specifies the volumes to mount in the container.
	Volumes []string `yaml:"volumes,omitempty"` // Map of volume names to volume definitions
	// User is the user to run the container as.
	User string `yaml:"user,omitempty"` // User to run the container as
	// WorkingDir is the working directory inside the container.
	WorkingDir string `yaml:"workingDir,omitempty"` // Working directory inside the container
	// WorkDir is the working directory inside the container.
	// Deprecated: use WorkingDir instead
	WorkDir string `yaml:"workDir,omitempty"` // Working directory inside the container
	// Platform specifies the platform for the container (e.g., "linux/amd64").
	Platform string `yaml:"platform,omitempty"` // Platform for the container
	// Ports specifies the ports to expose from the container.
	Ports []string `yaml:"ports,omitempty"` // List of ports to expose
	// Network is the network configuration for the container.
	Network string `yaml:"network,omitempty"` // Network configuration for the container
	// KeepContainer is the flag to keep the container after the DAG run.
	KeepContainer bool `yaml:"keepContainer,omitempty"` // Keep the container after the DAG run
	// Startup determines how the DAG-level container starts up.
	Startup string `yaml:"startup,omitempty"`
	// Command used when Startup == "command".
	Command []string `yaml:"command,omitempty"`
	// WaitFor readiness condition: running|healthy
	WaitFor string `yaml:"waitFor,omitempty"`
	// LogPattern regex to wait for in container logs.
	LogPattern string `yaml:"logPattern,omitempty"`
	// RestartPolicy: no|always|unless-stopped
	RestartPolicy string `yaml:"restartPolicy,omitempty"`
}

// runConfig defines configuration for controlling user interactions during DAG runs.
type runConfig struct {
	DisableParamEdit bool `yaml:"disableParamEdit,omitempty"` // Disable parameter editing when starting DAG
	DisableRunIdEdit bool `yaml:"disableRunIdEdit,omitempty"` // Disable custom run ID specification
}

// ssh defines the SSH configuration for the DAG.
type ssh struct {
	// User is the SSH user.
	User string `yaml:"user,omitempty"`
	// Host is the SSH host.
	Host string `yaml:"host,omitempty"`
	// Port is the SSH port (can be string or number).
	Port types.PortValue `yaml:"port,omitempty"`
	// Key is the path to the SSH private key.
	Key string `yaml:"key,omitempty"`
	// Password is the SSH password.
	Password string `yaml:"password,omitempty"`
	// StrictHostKey enables strict host key checking. Defaults to true if not specified.
	StrictHostKey *bool `yaml:"strictHostKey,omitempty"`
	// KnownHostFile is the path to the known_hosts file. Defaults to ~/.ssh/known_hosts.
	KnownHostFile string `yaml:"knownHostFile,omitempty"`
}

// secretRef defines a reference to an external secret.
type secretRef struct {
	// Name is the environment variable name (required).
	Name string `yaml:"name"`
	// Provider specifies the secret backend (required).
	Provider string `yaml:"provider"`
	// Key is the provider-specific identifier (required).
	Key string `yaml:"key"`
	// Options contains provider-specific configuration (optional).
	Options map[string]string `yaml:"options,omitempty"`
}

// Transformer transforms a spec field into output field(s).
// C is the context type, T is the input type.
type Transformer[C any, T any] interface {
	// Transform performs the transformation and sets field(s) on out
	Transform(ctx C, in T, out reflect.Value) error
}

// dagTransformer is a generic implementation that provides type safety
// for the builder function while satisfying the DAGTransformer interface.
type dagTransformer[T any] struct {
	fieldName string
	builder   func(ctx BuildContext, d *dag) (T, error)
}

func (t *dagTransformer[T]) Transform(ctx BuildContext, in *dag, out reflect.Value) error {
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

// newTransformer creates a DAGTransformer for a single field transformation
func newTransformer[T any](fieldName string, builder func(BuildContext, *dag) (T, error)) Transformer[BuildContext, *dag] {
	return &dagTransformer[T]{
		fieldName: fieldName,
		builder:   builder,
	}
}

// transform wraps a DAGTransformer with its name for error reporting
type transform struct {
	name        string
	transformer Transformer[BuildContext, *dag]
}

// metadataTransformers are always run (for listing, scheduling, etc.)
var metadataTransformers = []transform{
	{"name", newTransformer("Name", buildName)},
	{"group", newTransformer("Group", buildGroup)},
	{"description", newTransformer("Description", buildDescription)},
	{"type", newTransformer("Type", buildType)},
	{"tags", newTransformer("Tags", buildTags)},
	{"env", newTransformer("Env", buildEnvs)},
	{"schedule", newTransformer("Schedule", buildSchedule)},
	{"stopSchedule", newTransformer("StopSchedule", buildStopSchedule)},
	{"restartSchedule", newTransformer("RestartSchedule", buildRestartSchedule)},
	{"params", newTransformer("Params", buildParams)},
	{"defaultParams", newTransformer("DefaultParams", buildDefaultParams)},
	{"workerSelector", newTransformer("WorkerSelector", buildWorkerSelector)},
	{"timeout", newTransformer("Timeout", buildTimeout)},
	{"delay", newTransformer("Delay", buildDelay)},
	{"restartWait", newTransformer("RestartWait", buildRestartWait)},
	{"maxActiveRuns", newTransformer("MaxActiveRuns", buildMaxActiveRuns)},
	{"maxActiveSteps", newTransformer("MaxActiveSteps", buildMaxActiveSteps)},
	{"queue", newTransformer("Queue", buildQueue)},
	{"maxOutputSize", newTransformer("MaxOutputSize", buildMaxOutputSize)},
	{"skipIfSuccessful", newTransformer("SkipIfSuccessful", buildSkipIfSuccessful)},
}

// fullTransformers are only run when building the full DAG (not metadata-only)
var fullTransformers = []transform{
	{"logDir", newTransformer("LogDir", buildLogDir)},
	{"logOutput", newTransformer("LogOutput", buildLogOutput)},
	{"mailOn", newTransformer("MailOn", buildMailOn)},
	{"runConfig", newTransformer("RunConfig", buildRunConfig)},
	{"histRetentionDays", newTransformer("HistRetentionDays", buildHistRetentionDays)},
	{"maxCleanUpTime", newTransformer("MaxCleanUpTime", buildMaxCleanUpTime)},
	{"shell", newTransformer("Shell", buildShell)},
	{"shellArgs", newTransformer("ShellArgs", buildShellArgs)},
	{"workingDir", newTransformer("WorkingDir", buildWorkingDir)},
	{"container", newTransformer("Container", buildContainer)},
	{"registryAuths", newTransformer("RegistryAuths", buildRegistryAuths)},
	{"ssh", newTransformer("SSH", buildSSH)},
	{"secrets", newTransformer("Secrets", buildSecrets)},
	{"dotenv", newTransformer("Dotenv", buildDotenv)},
	{"smtpConfig", newTransformer("SMTP", buildSMTPConfig)},
	{"errMailConfig", newTransformer("ErrorMail", buildErrMailConfig)},
	{"infoMailConfig", newTransformer("InfoMail", buildInfoMailConfig)},
	{"preconditions", newTransformer("Preconditions", buildPreconditions)},
	{"otel", newTransformer("OTel", buildOTel)},
}

// runTransformers executes all transformers in the pipeline
func runTransformers(ctx BuildContext, spec *dag, result *core.DAG) core.ErrorList {
	var errs core.ErrorList
	out := reflect.ValueOf(result).Elem()

	// Always run metadata transformers
	for _, t := range metadataTransformers {
		if err := t.transformer.Transform(ctx, spec, out); err != nil {
			errs = append(errs, wrapTransformError(t.name, err))
		}
	}

	// Run full transformers only when not in metadata-only mode
	if !ctx.opts.Has(BuildFlagOnlyMetadata) {
		for _, t := range fullTransformers {
			if err := t.transformer.Transform(ctx, spec, out); err != nil {
				errs = append(errs, wrapTransformError(t.name, err))
			}
		}
	}

	return errs
}

// wrapTransformError wraps an error with the transformer name if it's not already a ValidationError
func wrapTransformError(name string, err error) error {
	var ve *core.ValidationError
	if errors.As(err, &ve) {
		return err
	}
	return core.NewValidationError(name, nil, err)
}

// build transforms the dag specification into a core.DAG.
func (d *dag) build(ctx BuildContext) (*core.DAG, error) {
	// Initialize with only Location (set from context, not spec)
	result := &core.DAG{
		Location: ctx.file,
	}

	// Run the transformer pipeline
	errs := runTransformers(ctx, d, result)

	// Build handlers and steps directly (they need access to partially built result)
	if !ctx.opts.Has(BuildFlagOnlyMetadata) {
		if handlerOn, err := buildHandlers(ctx, d, result); err != nil {
			errs = append(errs, core.NewValidationError("handlers", nil, err))
		} else {
			result.HandlerOn = handlerOn
		}

		if steps, err := buildSteps(ctx, d, result); err != nil {
			errs = append(errs, core.NewValidationError("steps", nil, err))
		} else {
			result.Steps = steps
		}
	}

	// Validate steps
	if err := core.ValidateSteps(result); err != nil {
		errs = append(errs, err)
	}

	// Validate name
	if result.Name != "" {
		if err := core.ValidateDAGName(result.Name); err != nil {
			errs = append(errs, core.NewValidationError("name", result.Name, err))
		}
	}

	if len(errs) > 0 {
		if ctx.opts.Has(BuildFlagAllowBuildErrors) {
			result.BuildErrors = errs
		} else {
			return nil, fmt.Errorf("failed to build DAG: %w", errs)
		}
	}

	return result, nil
}

// Builder functions - each returns a value instead of modifying result

func buildType(_ BuildContext, d *dag) (string, error) {
	t := strings.TrimSpace(d.Type)
	if t == "" {
		return core.TypeChain, nil
	}
	switch t {
	case core.TypeGraph, core.TypeChain:
		return t, nil
	case core.TypeAgent:
		return "", core.NewValidationError("type", t, fmt.Errorf("type 'agent' is reserved and not yet supported"))
	default:
		return "", core.NewValidationError("type", t, fmt.Errorf("invalid type: %s (must be one of: graph, chain)", t))
	}
}

// Builder functions - all return values instead of modifying result

func buildName(ctx BuildContext, d *dag) (string, error) {
	if ctx.opts.Name != "" {
		return strings.TrimSpace(ctx.opts.Name), nil
	}
	if name := strings.TrimSpace(d.Name); name != "" {
		return name, nil
	}
	// Fallback to filename without extension only for the main DAG (index 0)
	// Sub-DAGs in multi-DAG files must have explicit names
	if ctx.index == 0 {
		return defaultName(ctx.file), nil
	}
	return "", nil
}

func buildGroup(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Group), nil
}

func buildDescription(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Description), nil
}

func buildTimeout(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.TimeoutSec), nil
}

func buildDelay(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.DelaySec), nil
}

func buildRestartWait(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.RestartWaitSec), nil
}

func buildTags(_ BuildContext, d *dag) ([]string, error) {
	if d.Tags.IsZero() {
		return nil, nil
	}
	var ret []string
	for _, tag := range d.Tags.Values() {
		for _, t := range strings.Split(tag, ",") {
			normalized := strings.ToLower(strings.TrimSpace(t))
			if normalized != "" {
				ret = append(ret, normalized)
			}
		}
	}
	return ret, nil
}

func buildMaxActiveRuns(_ BuildContext, d *dag) (int, error) {
	if d.MaxActiveRuns != 0 {
		return d.MaxActiveRuns, nil
	}
	return 1, nil // Default
}

func buildMaxActiveSteps(_ BuildContext, d *dag) (int, error) {
	return d.MaxActiveSteps, nil
}

func buildQueue(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Queue), nil
}

func buildMaxOutputSize(_ BuildContext, d *dag) (int, error) {
	return d.MaxOutputSize, nil
}

func buildSkipIfSuccessful(_ BuildContext, d *dag) (bool, error) {
	return d.SkipIfSuccessful, nil
}

func buildLogDir(_ BuildContext, d *dag) (string, error) {
	return d.LogDir, nil
}

func buildLogOutput(_ BuildContext, d *dag) (core.LogOutputMode, error) {
	if d.LogOutput.IsZero() {
		return core.LogOutputSeparate, nil
	}
	return d.LogOutput.Mode(), nil
}

func buildMailOn(_ BuildContext, d *dag) (*core.MailOn, error) {
	if d.MailOn == nil {
		return nil, nil
	}
	return &core.MailOn{
		Failure: d.MailOn.Failure,
		Success: d.MailOn.Success,
	}, nil
}

func buildRunConfig(_ BuildContext, d *dag) (*core.RunConfig, error) {
	if d.RunConfig == nil {
		return nil, nil
	}
	return &core.RunConfig{
		DisableParamEdit: d.RunConfig.DisableParamEdit,
		DisableRunIdEdit: d.RunConfig.DisableRunIdEdit,
	}, nil
}

func buildHistRetentionDays(_ BuildContext, d *dag) (int, error) {
	if d.HistRetentionDays != nil {
		return *d.HistRetentionDays, nil
	}
	return 0, nil
}

func buildMaxCleanUpTime(_ BuildContext, d *dag) (time.Duration, error) {
	if d.MaxCleanUpTimeSec != nil {
		return time.Second * time.Duration(*d.MaxCleanUpTimeSec), nil
	}
	return 0, nil
}

func buildEnvs(ctx BuildContext, d *dag) ([]string, error) {
	vars, err := loadVariablesFromEnvValue(ctx, d.Env)
	if err != nil {
		return nil, err
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	return envs, nil
}

func buildSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return buildScheduler(d.Schedule.Starts())
}

func buildStopSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return buildScheduler(d.Schedule.Stops())
}

func buildRestartSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return buildScheduler(d.Schedule.Restarts())
}

// paramsResult holds the result of parsing parameters
type paramsResult struct {
	Params        []string
	DefaultParams string
}

func buildParams(ctx BuildContext, d *dag) ([]string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return nil, err
	}
	return result.Params, nil
}

func buildDefaultParams(ctx BuildContext, d *dag) (string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.DefaultParams, nil
}

func parseParamsInternal(ctx BuildContext, d *dag) (*paramsResult, error) {
	var (
		paramPairs []paramPair
		envs       []string
	)

	if err := parseParams(ctx, d.Params, &paramPairs, &envs); err != nil {
		return nil, err
	}

	// Create default parameters string in the form of "key=value key=value ..."
	var paramsToJoin []string
	for _, paramPair := range paramPairs {
		paramsToJoin = append(paramsToJoin, paramPair.Escaped())
	}
	defaultParams := strings.Join(paramsToJoin, " ")

	if ctx.opts.Parameters != "" {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.Parameters, &overridePairs, &overrideEnvs); err != nil {
			return nil, err
		}
		overrideParams(&paramPairs, overridePairs)
	}

	if len(ctx.opts.ParametersList) > 0 {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.ParametersList, &overridePairs, &overrideEnvs); err != nil {
			return nil, err
		}
		overrideParams(&paramPairs, overridePairs)
	}

	// Validate the parameters against a resolved schema (if declared)
	if !ctx.opts.Has(BuildFlagSkipSchemaValidation) {
		if resolvedSchema, err := resolveSchemaFromParams(d.Params, d.WorkingDir, ctx.file); err != nil {
			return nil, fmt.Errorf("failed to get JSON schema: %w", err)
		} else if resolvedSchema != nil {
			updatedPairs, err := validateParams(paramPairs, resolvedSchema)
			if err != nil {
				return nil, err
			}
			paramPairs = updatedPairs
		}
	}

	var params []string
	for _, paramPair := range paramPairs {
		params = append(params, paramPair.String())
	}

	// Note: envs from params are handled separately - they should be appended to Env
	// This is a limitation of the current transformer design; we may need to handle this specially
	_ = envs

	return &paramsResult{
		Params:        params,
		DefaultParams: defaultParams,
	}, nil
}

func buildWorkerSelector(_ BuildContext, d *dag) (map[string]string, error) {
	if len(d.WorkerSelector) == 0 {
		return nil, nil
	}

	ret := make(map[string]string)
	for key, val := range d.WorkerSelector {
		ret[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return ret, nil
}

// shellResult holds both shell and args for internal use
type shellResult struct {
	Shell string
	Args  []string
}

func parseShellInternal(ctx BuildContext, d *dag) (*shellResult, error) {
	if d.Shell.IsZero() {
		return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
	}

	// For array form, Command() returns first element, Arguments() returns rest
	if d.Shell.IsArray() {
		shell := d.Shell.Command()
		// Empty array should fall back to default shell
		if shell == "" {
			return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
		}
		if !ctx.opts.Has(BuildFlagNoEval) {
			shell = os.ExpandEnv(shell)
		}
		args := d.Shell.Arguments()
		if !ctx.opts.Has(BuildFlagNoEval) {
			for i, arg := range args {
				args[i] = os.ExpandEnv(arg)
			}
		}
		return &shellResult{Shell: shell, Args: args}, nil
	}

	// For string form, need to split command and args
	command := d.Shell.Command()
	if command == "" {
		return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
	}

	if !ctx.opts.Has(BuildFlagNoEval) {
		command = os.ExpandEnv(command)
	}

	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return nil, core.NewValidationError("shell", d.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	return &shellResult{Shell: strings.TrimSpace(shell), Args: args}, nil
}

func buildShell(ctx BuildContext, d *dag) (string, error) {
	result, err := parseShellInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.Shell, nil
}

func buildShellArgs(ctx BuildContext, d *dag) ([]string, error) {
	result, err := parseShellInternal(ctx, d)
	if err != nil {
		return nil, err
	}
	return result.Args, nil
}

func buildWorkingDir(ctx BuildContext, d *dag) (string, error) {
	switch {
	case d.WorkingDir != "":
		wd := d.WorkingDir
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
		return wd, nil

	case ctx.opts.DefaultWorkingDir != "":
		return ctx.opts.DefaultWorkingDir, nil

	case ctx.file != "":
		return filepath.Dir(ctx.file), nil

	default:
		dir, _ := os.Getwd()
		if dir == "" {
			var err error
			dir, err = os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get working directory: %w", err)
			}
		}
		return dir, nil
	}
}

func buildContainer(ctx BuildContext, d *dag) (*core.Container, error) {
	if d.Container == nil {
		return nil, nil
	}
	return buildContainerFromSpec(ctx, d.Container)
}

// buildContainerFromSpec is a shared function that builds a core.Container from a container spec.
// It is used by both DAG-level and step-level container configuration.
func buildContainerFromSpec(ctx BuildContext, c *container) (*core.Container, error) {
	// If container is specified but image is empty, return an error
	if c.Image == "" {
		return nil, core.NewValidationError("container.image", c.Image, fmt.Errorf("image is required when container is specified"))
	}

	pullPolicy, err := core.ParsePullPolicy(c.PullPolicy)
	if err != nil {
		return nil, core.NewValidationError("container.pullPolicy", c.PullPolicy, err)
	}

	vars, err := loadVariables(ctx, c.Env)
	if err != nil {
		return nil, core.NewValidationError("container.env", c.Env, err)
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	result := &core.Container{
		Name:          strings.TrimSpace(c.Name),
		Image:         c.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       c.Volumes,
		User:          c.User,
		Platform:      c.Platform,
		Ports:         c.Ports,
		Network:       c.Network,
		KeepContainer: c.KeepContainer,
		Startup:       core.ContainerStartup(strings.ToLower(strings.TrimSpace(c.Startup))),
		Command:       c.Command,
		WaitFor:       core.ContainerWaitFor(strings.ToLower(strings.TrimSpace(c.WaitFor))),
		LogPattern:    c.LogPattern,
		RestartPolicy: strings.TrimSpace(c.RestartPolicy),
	}

	// Backward compatibility
	if c.WorkDir != "" {
		result.WorkingDir = c.WorkDir
	} else {
		result.WorkingDir = c.WorkingDir
	}

	return result, nil
}

func buildRegistryAuths(ctx BuildContext, d *dag) (map[string]*core.AuthConfig, error) {
	if d.RegistryAuths == nil {
		return nil, nil
	}

	expand := func(s string) string {
		if ctx.opts.Has(BuildFlagNoEval) {
			return s
		}
		return os.ExpandEnv(s)
	}

	// parseAuthConfig parses auth config from a map with string keys
	parseAuthConfig := func(m map[string]any) *core.AuthConfig {
		cfg := &core.AuthConfig{}
		if v, ok := m["username"].(string); ok {
			cfg.Username = expand(v)
		}
		if v, ok := m["password"].(string); ok {
			cfg.Password = expand(v)
		}
		if v, ok := m["auth"].(string); ok {
			cfg.Auth = expand(v)
		}
		return cfg
	}

	// parseAuthData parses auth data which can be a string or a map
	parseAuthData := func(authData any) *core.AuthConfig {
		switch auth := authData.(type) {
		case string:
			return &core.AuthConfig{Auth: expand(auth)}
		case map[string]any:
			return parseAuthConfig(auth)
		case map[any]any:
			// Convert map[any]any to map[string]any
			m := make(map[string]any)
			for k, v := range auth {
				if ks, ok := k.(string); ok {
					m[ks] = v
				}
			}
			return parseAuthConfig(m)
		}
		return &core.AuthConfig{}
	}

	registryAuths := make(map[string]*core.AuthConfig)

	switch v := d.RegistryAuths.(type) {
	case string:
		registryAuths["_json"] = &core.AuthConfig{Auth: expand(v)}

	case map[string]any:
		for registry, authData := range v {
			registryAuths[registry] = parseAuthData(authData)
		}

	case map[any]any:
		for registryKey, authData := range v {
			if registry, ok := registryKey.(string); ok {
				registryAuths[registry] = parseAuthData(authData)
			}
		}

	default:
		return nil, core.NewValidationError("registryAuths", d.RegistryAuths, fmt.Errorf("invalid type: %T", d.RegistryAuths))
	}

	return registryAuths, nil
}

func buildSSH(_ BuildContext, d *dag) (*core.SSHConfig, error) {
	if d.SSH == nil {
		return nil, nil
	}

	port := d.SSH.Port.String()
	if port == "" {
		port = "22"
	}

	strictHostKey := true
	if d.SSH.StrictHostKey != nil {
		strictHostKey = *d.SSH.StrictHostKey
	}

	return &core.SSHConfig{
		User:          d.SSH.User,
		Host:          d.SSH.Host,
		Port:          port,
		Key:           d.SSH.Key,
		Password:      d.SSH.Password,
		StrictHostKey: strictHostKey,
		KnownHostFile: d.SSH.KnownHostFile,
	}, nil
}

func buildSecrets(_ BuildContext, d *dag) ([]core.SecretRef, error) {
	if len(d.Secrets) == 0 {
		return nil, nil
	}
	return parseSecretRefs(d.Secrets)
}

func buildDotenv(_ BuildContext, d *dag) ([]string, error) {
	if d.Dotenv.IsZero() {
		return []string{".env"}, nil
	}
	return d.Dotenv.Values(), nil
}

func buildHandlers(ctx BuildContext, d *dag, result *core.DAG) (core.HandlerOn, error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}
	var handlerOn core.HandlerOn
	var err error

	if d.HandlerOn.Init != nil {
		d.HandlerOn.Init.Name = core.HandlerOnInit.String()
		if handlerOn.Init, err = d.HandlerOn.Init.build(buildCtx); err != nil {
			return handlerOn, err
		}
	}

	if d.HandlerOn.Exit != nil {
		d.HandlerOn.Exit.Name = core.HandlerOnExit.String()
		if handlerOn.Exit, err = d.HandlerOn.Exit.build(buildCtx); err != nil {
			return handlerOn, err
		}
	}

	if d.HandlerOn.Success != nil {
		d.HandlerOn.Success.Name = core.HandlerOnSuccess.String()
		if handlerOn.Success, err = d.HandlerOn.Success.build(buildCtx); err != nil {
			return handlerOn, err
		}
	}

	if d.HandlerOn.Failure != nil {
		d.HandlerOn.Failure.Name = core.HandlerOnFailure.String()
		if handlerOn.Failure, err = d.HandlerOn.Failure.build(buildCtx); err != nil {
			return handlerOn, err
		}
	}

	// Handle Abort (canonical) and Cancel (deprecated, for backward compatibility)
	if d.HandlerOn.Abort != nil && d.HandlerOn.Cancel != nil {
		return handlerOn, fmt.Errorf("cannot specify both 'abort' and 'cancel' in handlerOn; use 'abort' (cancel is deprecated)")
	}
	var abortStep *step
	switch {
	case d.HandlerOn.Abort != nil:
		abortStep = d.HandlerOn.Abort
	case d.HandlerOn.Cancel != nil:
		abortStep = d.HandlerOn.Cancel
	}
	if abortStep != nil {
		abortStep.Name = core.HandlerOnCancel.String()
		if handlerOn.Cancel, err = abortStep.build(buildCtx); err != nil {
			return handlerOn, err
		}
	}

	return handlerOn, nil
}

func buildSMTPConfig(_ BuildContext, d *dag) (*core.SMTPConfig, error) {
	portStr := d.SMTP.Port.String()

	// Return nil only if ALL fields are empty/default
	if d.SMTP.Host == "" && portStr == "" && d.SMTP.Username == "" && d.SMTP.Password == "" {
		return nil, nil
	}

	return &core.SMTPConfig{
		Host:     d.SMTP.Host,
		Port:     portStr,
		Username: d.SMTP.Username,
		Password: d.SMTP.Password,
	}, nil
}

func buildErrMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.ErrorMail)
}

func buildInfoMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.InfoMail)
}

func buildPreconditions(ctx BuildContext, d *dag) ([]*core.Condition, error) {
	conditions, err := parsePrecondition(ctx, d.Preconditions)
	if err != nil {
		return nil, err
	}
	condition, err := parsePrecondition(ctx, d.Precondition)
	if err != nil {
		return nil, err
	}

	return append(conditions, condition...), nil
}

func buildOTel(_ BuildContext, d *dag) (*core.OTelConfig, error) {
	if d.OTel == nil {
		return nil, nil
	}

	switch v := d.OTel.(type) {
	case map[string]any:
		config := &core.OTelConfig{}

		if enabled, ok := v["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if endpoint, ok := v["endpoint"].(string); ok {
			config.Endpoint = endpoint
		}
		if headers, ok := v["headers"].(map[string]any); ok {
			config.Headers = make(map[string]string)
			for key, val := range headers {
				if strVal, ok := val.(string); ok {
					config.Headers[key] = strVal
				}
			}
		}
		if insecure, ok := v["insecure"].(bool); ok {
			config.Insecure = insecure
		}
		if timeout, ok := v["timeout"].(string); ok {
			duration, err := time.ParseDuration(timeout)
			if err != nil {
				return nil, core.NewValidationError("otel.timeout", timeout, err)
			}
			config.Timeout = duration
		}
		if resource, ok := v["resource"].(map[string]any); ok {
			config.Resource = resource
		}

		return config, nil

	default:
		return nil, core.NewValidationError("otel", v, fmt.Errorf("otel must be a map"))
	}
}

func buildSteps(ctx BuildContext, d *dag, result *core.DAG) ([]core.Step, error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}
	names := make(map[string]struct{})

	switch v := d.Steps.(type) {
	case nil:
		return nil, nil

	case []any:
		normalized := normalizeStepData(ctx, v)

		var builtSteps []*core.Step
		var prevSteps []*core.Step
		for i, raw := range normalized {
			switch v := raw.(type) {
			case map[string]any:
				st, err := buildStepFromRaw(buildCtx, i, v, names)
				if err != nil {
					return nil, err
				}

				injectChainDependencies(result, prevSteps, st)
				builtSteps = append(builtSteps, st)
				prevSteps = []*core.Step{st}

			case []any:
				var tempSteps []*core.Step
				var normalizedNested = normalizeStepData(ctx, v)
				for _, nested := range normalizedNested {
					switch vv := nested.(type) {
					case map[string]any:
						st, err := buildStepFromRaw(buildCtx, i, vv, names)
						if err != nil {
							return nil, err
						}

						injectChainDependencies(result, prevSteps, st)
						builtSteps = append(builtSteps, st)
						tempSteps = append(tempSteps, st)

					default:
						return nil, core.NewValidationError("steps", raw, ErrInvalidStepData)
					}
				}
				prevSteps = tempSteps

			default:
				return nil, core.NewValidationError("steps", raw, ErrInvalidStepData)
			}
		}

		var steps []core.Step
		for _, st := range builtSteps {
			steps = append(steps, *st)
		}
		return steps, nil

	case map[string]any:
		stepsMap := make(map[string]step)
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepsMap,
			DecodeHook:  TypedUnionDecodeHook(),
		})
		if err := md.Decode(v); err != nil {
			return nil, core.NewValidationError("steps", v, err)
		}

		var steps []core.Step
		for name, st := range stepsMap {
			st.Name = name
			names[st.Name] = struct{}{}
			builtStep, err := st.build(buildCtx)
			if err != nil {
				return nil, err
			}
			steps = append(steps, *builtStep)
		}
		return steps, nil

	default:
		return nil, core.NewValidationError("steps", v, ErrStepsMustBeArrayOrMap)
	}
}

// buildMailConfigInternal builds a core.MailConfig from the mail configuration.
func buildMailConfigInternal(def mailConfig) (*core.MailConfig, error) {
	// StringOrArray already parsed during YAML unmarshal
	rawAddresses := def.To.Values()

	// Trim whitespace and filter out empty entries
	var toAddresses []string
	for _, addr := range rawAddresses {
		trimmed := strings.TrimSpace(addr)
		if trimmed != "" {
			toAddresses = append(toAddresses, trimmed)
		}
	}

	// Return nil if no valid configuration (all fields are empty/default)
	if def.From == "" && len(toAddresses) == 0 && def.Prefix == "" && !def.AttachLogs {
		return nil, nil
	}

	return &core.MailConfig{
		From:       strings.TrimSpace(def.From),
		To:         toAddresses,
		Prefix:     strings.TrimSpace(def.Prefix),
		AttachLogs: def.AttachLogs,
	}, nil
}
