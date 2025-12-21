package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// LogFile is the file to write the log.
	LogDir string
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

// build transforms the dag specification into a core.DAG.
// Simple field mappings are done inline; complex transformations call dedicated methods.
func (d *dag) build(ctx BuildContext) (*core.DAG, error) {
	// Initialize with simple field mappings
	result := &core.DAG{
		Location:          ctx.file,
		Name:              d.buildName(ctx),
		Group:             strings.TrimSpace(d.Group),
		Description:       strings.TrimSpace(d.Description),
		Timeout:           time.Second * time.Duration(d.TimeoutSec),
		Delay:             time.Second * time.Duration(d.DelaySec),
		RestartWait:       time.Second * time.Duration(d.RestartWaitSec),
		Tags:              d.buildTags(),
		MaxActiveSteps:    d.MaxActiveSteps,
		MaxActiveRuns:     d.buildMaxActiveRuns(),
		Queue:             strings.TrimSpace(d.Queue),
		MaxOutputSize:     d.MaxOutputSize,
		LogDir:            d.LogDir,
		SkipIfSuccessful:  d.SkipIfSuccessful,
		MailOn:            d.buildMailOn(),
		RunConfig:         d.buildRunConfig(),
		HistRetentionDays: d.buildHistRetentionDays(),
		MaxCleanUpTime:    d.buildMaxCleanUpTime(),
	}

	// Complex transformations that may return errors
	var errs core.ErrorList
	transformers := []struct {
		name string
		fn   func(ctx BuildContext, result *core.DAG) error
		meta bool // metadata-only (runs even with BuildFlagOnlyMetadata)
	}{
		{name: "type", fn: d.buildType, meta: true},
		{name: "env", fn: d.buildEnvs, meta: true},
		{name: "schedule", fn: d.buildSchedule, meta: true},
		{name: "params", fn: d.buildParams, meta: true},
		{name: "workerSelector", fn: d.buildWorkerSelector, meta: true},
		{name: "shell", fn: d.buildShell},
		{name: "workingDir", fn: d.buildWorkingDir},
		{name: "container", fn: d.buildContainer},
		{name: "registryAuths", fn: d.buildRegistryAuths},
		{name: "ssh", fn: d.buildSSH},
		{name: "secrets", fn: d.buildSecrets},
		{name: "dotenv", fn: d.buildDotenv},
		{name: "handlers", fn: d.buildHandlers},
		{name: "smtpConfig", fn: d.buildSMTPConfig},
		{name: "errMailConfig", fn: d.buildErrMailConfig},
		{name: "infoMailConfig", fn: d.buildInfoMailConfig},
		{name: "preconditions", fn: d.buildPreconditions},
		{name: "otel", fn: d.buildOTel},
		{name: "steps", fn: d.buildSteps},
	}

	for _, t := range transformers {
		if !t.meta && ctx.opts.Has(BuildFlagOnlyMetadata) {
			continue
		}
		if err := t.fn(ctx, result); err != nil {
			var ve *core.ValidationError
			if errors.As(err, &ve) && ve.Field == t.name {
				errs = append(errs, err)
			} else {
				errs = append(errs, core.NewValidationError(t.name, nil, err))
			}
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

// Simple inline transformations

func (d *dag) buildName(ctx BuildContext) string {
	if ctx.opts.Name != "" {
		return strings.TrimSpace(ctx.opts.Name)
	}
	return strings.TrimSpace(d.Name)
}

func (d *dag) buildType(_ BuildContext, result *core.DAG) error {
	t := strings.TrimSpace(d.Type)
	if t == "" {
		result.Type = core.TypeChain
		return nil
	}
	switch t {
	case core.TypeGraph, core.TypeChain, core.TypeAgent:
		result.Type = t
		return nil
	default:
		return core.NewValidationError("type", t, fmt.Errorf("invalid type: %s (must be one of: graph, chain, agent)", t))
	}
}

func (d *dag) buildTags() []string {
	if d.Tags.IsZero() {
		return nil
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
	return ret
}

func (d *dag) buildMaxActiveRuns() int {
	if d.MaxActiveRuns != 0 {
		return d.MaxActiveRuns
	}
	return 1 // Default
}

func (d *dag) buildMailOn() *core.MailOn {
	if d.MailOn == nil {
		return nil
	}
	return &core.MailOn{
		Failure: d.MailOn.Failure,
		Success: d.MailOn.Success,
	}
}

func (d *dag) buildRunConfig() *core.RunConfig {
	if d.RunConfig == nil {
		return nil
	}
	return &core.RunConfig{
		DisableParamEdit: d.RunConfig.DisableParamEdit,
		DisableRunIdEdit: d.RunConfig.DisableRunIdEdit,
	}
}

func (d *dag) buildHistRetentionDays() int {
	if d.HistRetentionDays != nil {
		return *d.HistRetentionDays
	}
	return 0
}

func (d *dag) buildMaxCleanUpTime() time.Duration {
	if d.MaxCleanUpTimeSec != nil {
		return time.Second * time.Duration(*d.MaxCleanUpTimeSec)
	}
	return 0
}

// Complex transformations (methods on dag)

// buildEnvs builds the environment variables for the DAG.
func (d *dag) buildEnvs(ctx BuildContext, result *core.DAG) error {
	vars, err := loadVariablesFromEnvValue(ctx, d.Env)
	if err != nil {
		return err
	}

	// Store env vars in core.DAG temporarily for params to reference (e.g., P2=${A001})
	if ctx.buildEnv == nil {
		ctx.buildEnv = make(map[string]string)
	}
	for k, v := range vars {
		result.Env = append(result.Env, fmt.Sprintf("%s=%s", k, v))
		ctx.buildEnv[k] = v
	}

	return nil
}

// buildSchedule parses the schedule in different formats and builds the schedule.
func (d *dag) buildSchedule(_ BuildContext, result *core.DAG) error {
	if d.Schedule.IsZero() {
		return nil
	}

	var err error
	result.Schedule, err = buildScheduler(d.Schedule.Starts())
	if err != nil {
		return err
	}
	result.StopSchedule, err = buildScheduler(d.Schedule.Stops())
	if err != nil {
		return err
	}
	result.RestartSchedule, err = buildScheduler(d.Schedule.Restarts())
	return err
}

// buildParams builds the parameters for the DAG.
func (d *dag) buildParams(ctx BuildContext, result *core.DAG) error {
	var (
		paramPairs []paramPair
		envs       []string
	)

	if err := parseParams(ctx, d.Params, &paramPairs, &envs); err != nil {
		return err
	}

	// Create default parameters string in the form of "key=value key=value ..."
	var paramsToJoin []string
	for _, paramPair := range paramPairs {
		paramsToJoin = append(paramsToJoin, paramPair.Escaped())
	}
	result.DefaultParams = strings.Join(paramsToJoin, " ")

	if ctx.opts.Parameters != "" {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.Parameters, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		overrideParams(&paramPairs, overridePairs)
	}

	if len(ctx.opts.ParametersList) > 0 {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.ParametersList, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		overrideParams(&paramPairs, overridePairs)
	}

	// Validate the parameters against a resolved schema (if declared)
	if !ctx.opts.Has(BuildFlagSkipSchemaValidation) {
		if resolvedSchema, err := resolveSchemaFromParams(d.Params, d.WorkingDir, result.Location); err != nil {
			return fmt.Errorf("failed to get JSON schema: %w", err)
		} else if resolvedSchema != nil {
			updatedPairs, err := validateParams(paramPairs, resolvedSchema)
			if err != nil {
				return err
			}
			paramPairs = updatedPairs
		}
	}

	for _, paramPair := range paramPairs {
		result.Params = append(result.Params, paramPair.String())
	}

	result.Env = append(result.Env, envs...)

	return nil
}

// buildWorkerSelector builds the worker selector for the DAG.
func (d *dag) buildWorkerSelector(_ BuildContext, result *core.DAG) error {
	if len(d.WorkerSelector) == 0 {
		return nil
	}

	ret := make(map[string]string)
	for key, val := range d.WorkerSelector {
		ret[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}

	result.WorkerSelector = ret
	return nil
}

// buildShell builds the shell configuration for the DAG.
func (d *dag) buildShell(ctx BuildContext, result *core.DAG) error {
	if d.Shell.IsZero() {
		result.Shell = cmdutil.GetShellCommand("")
		return nil
	}

	// For array form, Command() returns first element, Arguments() returns rest
	if d.Shell.IsArray() {
		shell := d.Shell.Command()
		// Empty array should fall back to default shell
		if shell == "" {
			result.Shell = cmdutil.GetShellCommand("")
			return nil
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
		result.Shell = shell
		result.ShellArgs = args
		return nil
	}

	// For string form, need to split command and args
	command := d.Shell.Command()
	if command == "" {
		result.Shell = cmdutil.GetShellCommand("")
		return nil
	}

	if !ctx.opts.Has(BuildFlagNoEval) {
		command = os.ExpandEnv(command)
	}

	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return core.NewValidationError("shell", d.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	result.Shell = strings.TrimSpace(shell)
	result.ShellArgs = args
	return nil
}

// buildWorkingDir builds the working directory for the DAG.
func (d *dag) buildWorkingDir(ctx BuildContext, result *core.DAG) error {
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
		result.WorkingDir = wd

	case ctx.opts.DefaultWorkingDir != "":
		result.WorkingDir = ctx.opts.DefaultWorkingDir

	case ctx.file != "":
		result.WorkingDir = filepath.Dir(ctx.file)

	default:
		dir, _ := os.Getwd()
		if dir == "" {
			var err error
			dir, err = os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
		}
		result.WorkingDir = dir
	}

	return nil
}

// buildContainer builds the container configuration for the DAG.
func (d *dag) buildContainer(ctx BuildContext, result *core.DAG) error {
	// If no container is specified, there's no container config
	if d.Container == nil {
		return nil
	}

	// If container is specified but image is empty, return an error
	if d.Container.Image == "" {
		return core.NewValidationError("container.image", d.Container.Image, fmt.Errorf("image is required when container is specified"))
	}

	pullPolicy, err := core.ParsePullPolicy(d.Container.PullPolicy)
	if err != nil {
		return core.NewValidationError("container.pullPolicy", d.Container.PullPolicy, err)
	}

	vars, err := loadVariables(ctx, d.Container.Env)
	if err != nil {
		return core.NewValidationError("container.env", d.Container.Env, err)
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	container := core.Container{
		Name:          strings.TrimSpace(d.Container.Name),
		Image:         d.Container.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       d.Container.Volumes,
		User:          d.Container.User,
		Platform:      d.Container.Platform,
		Ports:         d.Container.Ports,
		Network:       d.Container.Network,
		KeepContainer: d.Container.KeepContainer,
		Startup:       core.ContainerStartup(strings.ToLower(strings.TrimSpace(d.Container.Startup))),
		Command:       append([]string{}, d.Container.Command...),
		WaitFor:       core.ContainerWaitFor(strings.ToLower(strings.TrimSpace(d.Container.WaitFor))),
		LogPattern:    d.Container.LogPattern,
		RestartPolicy: strings.TrimSpace(d.Container.RestartPolicy),
	}

	// Backward compatibility
	if d.Container.WorkDir != "" {
		container.WorkingDir = d.Container.WorkDir
	} else {
		container.WorkingDir = d.Container.WorkingDir
	}

	result.Container = &container

	return nil
}

// buildRegistryAuths builds the registry authentication configuration.
func (d *dag) buildRegistryAuths(ctx BuildContext, result *core.DAG) error {
	if d.RegistryAuths == nil {
		return nil
	}

	result.RegistryAuths = make(map[string]*core.AuthConfig)

	switch v := d.RegistryAuths.(type) {
	case string:
		expandedJSON := v
		if !ctx.opts.Has(BuildFlagNoEval) {
			expandedJSON = os.ExpandEnv(v)
		}
		result.RegistryAuths["_json"] = &core.AuthConfig{
			Auth: expandedJSON,
		}

	case map[string]any:
		for registry, authData := range v {
			authConfig := &core.AuthConfig{}

			switch auth := authData.(type) {
			case string:
				if !ctx.opts.Has(BuildFlagNoEval) {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
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

			result.RegistryAuths[registry] = authConfig
		}

	case map[any]any:
		for registryKey, authData := range v {
			registry, ok := registryKey.(string)
			if !ok {
				continue
			}

			authConfig := &core.AuthConfig{}

			switch auth := authData.(type) {
			case string:
				if !ctx.opts.Has(BuildFlagNoEval) {
					auth = os.ExpandEnv(auth)
				}
				authConfig.Auth = auth

			case map[string]any:
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

			result.RegistryAuths[registry] = authConfig
		}

	default:
		return core.NewValidationError("registryAuths", d.RegistryAuths, fmt.Errorf("invalid type: %T", d.RegistryAuths))
	}

	return nil
}

// buildSSH builds the SSH configuration for the DAG.
func (d *dag) buildSSH(_ BuildContext, result *core.DAG) error {
	if d.SSH == nil {
		return nil
	}

	port := d.SSH.Port.String()
	if port == "" {
		port = "22"
	}

	strictHostKey := true
	if d.SSH.StrictHostKey != nil {
		strictHostKey = *d.SSH.StrictHostKey
	}

	result.SSH = &core.SSHConfig{
		User:          d.SSH.User,
		Host:          d.SSH.Host,
		Port:          port,
		Key:           d.SSH.Key,
		Password:      d.SSH.Password,
		StrictHostKey: strictHostKey,
		KnownHostFile: d.SSH.KnownHostFile,
	}

	return nil
}

// buildSecrets builds the secrets references from the spec.
func (d *dag) buildSecrets(_ BuildContext, result *core.DAG) error {
	if len(d.Secrets) == 0 {
		return nil
	}

	secrets, err := parseSecretRefs(d.Secrets)
	if err != nil {
		return err
	}

	result.Secrets = secrets
	return nil
}

// buildDotenv builds the dotenv configuration for the DAG.
func (d *dag) buildDotenv(ctx BuildContext, result *core.DAG) error {
	if d.Dotenv.IsZero() {
		result.Dotenv = []string{".env"}
	} else {
		result.Dotenv = d.Dotenv.Values()
	}

	if !ctx.opts.Has(BuildFlagNoEval) {
		result.LoadDotEnv(ctx.ctx)
	}

	return nil
}

// buildHandlers builds the handlers for the DAG.
func (d *dag) buildHandlers(ctx BuildContext, result *core.DAG) (err error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}

	if d.HandlerOn.Init != nil {
		d.HandlerOn.Init.Name = core.HandlerOnInit.String()
		if result.HandlerOn.Init, err = d.HandlerOn.Init.build(buildCtx); err != nil {
			return err
		}
	}

	if d.HandlerOn.Exit != nil {
		d.HandlerOn.Exit.Name = core.HandlerOnExit.String()
		if result.HandlerOn.Exit, err = d.HandlerOn.Exit.build(buildCtx); err != nil {
			return err
		}
	}

	if d.HandlerOn.Success != nil {
		d.HandlerOn.Success.Name = core.HandlerOnSuccess.String()
		if result.HandlerOn.Success, err = d.HandlerOn.Success.build(buildCtx); err != nil {
			return
		}
	}

	if d.HandlerOn.Failure != nil {
		d.HandlerOn.Failure.Name = core.HandlerOnFailure.String()
		if result.HandlerOn.Failure, err = d.HandlerOn.Failure.build(buildCtx); err != nil {
			return
		}
	}

	// Handle Abort (canonical) and Cancel (deprecated, for backward compatibility)
	if d.HandlerOn.Abort != nil && d.HandlerOn.Cancel != nil {
		return fmt.Errorf("cannot specify both 'abort' and 'cancel' in handlerOn; use 'abort' (cancel is deprecated)")
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
		if result.HandlerOn.Cancel, err = abortStep.build(buildCtx); err != nil {
			return
		}
	}

	return nil
}

// buildSMTPConfig builds the SMTP configuration for the DAG.
func (d *dag) buildSMTPConfig(_ BuildContext, result *core.DAG) error {
	portStr := d.SMTP.Port.String()

	if d.SMTP.Host == "" && portStr == "" {
		return nil
	}

	result.SMTP = &core.SMTPConfig{
		Host:     d.SMTP.Host,
		Port:     portStr,
		Username: d.SMTP.Username,
		Password: d.SMTP.Password,
	}

	return nil
}

// buildErrMailConfig builds the error mail configuration for the DAG.
func (d *dag) buildErrMailConfig(_ BuildContext, result *core.DAG) error {
	var err error
	result.ErrorMail, err = buildMailConfig(d.ErrorMail)
	return err
}

// buildInfoMailConfig builds the info mail configuration for the DAG.
func (d *dag) buildInfoMailConfig(_ BuildContext, result *core.DAG) error {
	var err error
	result.InfoMail, err = buildMailConfig(d.InfoMail)
	return err
}

// buildPreconditions builds the preconditions for the DAG.
func (d *dag) buildPreconditions(ctx BuildContext, result *core.DAG) error {
	conditions, err := parsePrecondition(ctx, d.Preconditions)
	if err != nil {
		return err
	}
	condition, err := parsePrecondition(ctx, d.Precondition)
	if err != nil {
		return err
	}

	result.Preconditions = append(conditions, condition...)

	return nil
}

// buildOTel builds the OpenTelemetry configuration for the DAG.
func (d *dag) buildOTel(_ BuildContext, result *core.DAG) error {
	if d.OTel == nil {
		return nil
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
				return core.NewValidationError("otel.timeout", timeout, err)
			}
			config.Timeout = duration
		}
		if resource, ok := v["resource"].(map[string]any); ok {
			config.Resource = resource
		}

		result.OTel = config
		return nil

	default:
		return core.NewValidationError("otel", v, fmt.Errorf("otel must be a map"))
	}
}

// buildSteps builds the steps for the DAG.
func (d *dag) buildSteps(ctx BuildContext, result *core.DAG) error {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}
	names := make(map[string]struct{})

	switch v := d.Steps.(type) {
	case nil:
		return nil

	case []any:
		normalized := normalizeStepData(ctx, v)

		var builtSteps []*core.Step
		var prevSteps []*core.Step
		for i, raw := range normalized {
			switch v := raw.(type) {
			case map[string]any:
				st, err := buildStepFromRaw(buildCtx, i, v, names)
				if err != nil {
					return err
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
							return err
						}

						injectChainDependencies(result, prevSteps, st)
						builtSteps = append(builtSteps, st)
						tempSteps = append(tempSteps, st)

					default:
						return core.NewValidationError("steps", raw, ErrInvalidStepData)
					}
				}
				prevSteps = tempSteps

			default:
				return core.NewValidationError("steps", raw, ErrInvalidStepData)
			}
		}

		for _, st := range builtSteps {
			result.Steps = append(result.Steps, *st)
		}

		return nil

	case map[string]any:
		stepsMap := make(map[string]step)
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepsMap,
			DecodeHook:  TypedUnionDecodeHook(),
		})
		if err := md.Decode(v); err != nil {
			return core.NewValidationError("steps", v, err)
		}
		for name, st := range stepsMap {
			st.Name = name
			names[st.Name] = struct{}{}
			builtStep, err := st.build(buildCtx)
			if err != nil {
				return err
			}
			result.Steps = append(result.Steps, *builtStep)
		}

		return nil

	default:
		return core.NewValidationError("steps", v, ErrStepsMustBeArrayOrMap)
	}
}
