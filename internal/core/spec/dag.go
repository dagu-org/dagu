package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/eval"
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
	// CatchupWindow is the lookback horizon for missed intervals (e.g. "6h", "2d12h").
	// If set, enables catch-up on scheduler restart. If omitted, no catch-up.
	CatchupWindow string
	// OverlapPolicy controls how multiple catch-up runs are handled: "skip" or "all".
	OverlapPolicy string
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
	// WaitMail is the mail configuration for wait status.
	WaitMail mailConfig
	// TimeoutSec is the timeout in seconds to finish the DAG.
	TimeoutSec int
	// DelaySec is the delay in seconds to start the first node.
	DelaySec int
	// RestartWaitSec is the wait in seconds to when the DAG is restarted.
	RestartWaitSec int
	// HistRetentionDays is the retention days of the dag-runs history.
	HistRetentionDays *int
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
	Tags types.TagsValue
	// Queue is the name of the queue to assign this DAG to.
	Queue string
	// MaxOutputSize is the maximum size of the output for each step.
	MaxOutputSize int
	// OTel is the OpenTelemetry configuration.
	OTel any
	// WorkerSelector specifies required worker labels for execution.
	// Can be a map of label key-value pairs or the string "local" to force local execution.
	WorkerSelector any
	// Container is the container definition for the DAG.
	// Can be a string (existing container name to exec into) or an object (container configuration).
	Container any
	// RunConfig contains configuration for controlling user interactions during DAG runs.
	RunConfig *runConfig
	// RegistryAuths maps registry hostnames to authentication configs.
	// Can be either a JSON string or a map of registry to auth config.
	RegistryAuths any
	// SSH is the default SSH configuration for the DAG.
	SSH *ssh
	// S3 is the default S3 configuration for the DAG.
	// Steps can inherit these settings without specifying them individually.
	S3 *s3Config `yaml:"s3,omitempty"`
	// LLM is the default LLM configuration for all chat steps in this DAG.
	// Steps can override this configuration by specifying their own llm field.
	LLM *llmConfig `yaml:"llm,omitempty"`
	// Redis is the default Redis configuration for all redis steps in this DAG.
	// Steps can override this configuration by specifying their own config fields.
	Redis *redisConfig `yaml:"redis,omitempty"`
	// Secrets contains references to external secrets.
	Secrets []secretRef
}

// handlerOn defines the steps to be executed on different events.
type handlerOn struct {
	Init    *step // Step to execute before steps (after preconditions pass)
	Failure *step // Step to execute on failure
	Success *step // Step to execute on success
	Abort   *step // Step to execute on abort
	Exit    *step // Step to execute on exit
	Wait    *step // Step to execute when DAG enters wait status (HITL)
}

// smtpConfig defines the SMTP configuration.
type smtpConfig struct {
	Host     string          // SMTP host
	Port     types.PortValue // SMTP port (can be string or number)
	Username string          // SMTP username
	Password string          // SMTP password
}

// IsZero returns true if all fields are empty/default.
func (s smtpConfig) IsZero() bool {
	return s == smtpConfig{}
}

// mailConfig defines the mail configuration.
type mailConfig struct {
	From       string              // Sender email address
	To         types.StringOrArray // Recipient email address(es) - can be string or []string
	Prefix     string              // Prefix for the email subject
	AttachLogs bool                // Flag to attach logs to the email
}

// IsZero returns true if all fields are empty/default.
func (m mailConfig) IsZero() bool {
	return reflect.DeepEqual(m, mailConfig{})
}

// mailOn defines the conditions to send mail.
type mailOn struct {
	Failure bool // Send mail on failure
	Success bool // Send mail on success
	Wait    bool // Send mail on wait status
}

// container defines the container configuration for the DAG.
type container struct {
	// Exec specifies an existing container to exec into.
	// Mutually exclusive with Image.
	Exec string `yaml:"exec,omitempty"`
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
	// Healthcheck defines a custom healthcheck for the container.
	Healthcheck *healthcheck `yaml:"healthcheck,omitempty"`
	// Shell specifies the shell wrapper for executing step commands.
	Shell []string `yaml:"shell,omitempty"`
}

// healthcheck is the spec representation for custom health checks.
// Durations are specified as strings (e.g., "5s", "1m") for YAML convenience.
type healthcheck struct {
	// Test is the command to run. Must start with NONE, CMD, or CMD-SHELL.
	Test []string `yaml:"test,omitempty"`
	// Interval is the time between checks (e.g., "5s").
	Interval string `yaml:"interval,omitempty"`
	// Timeout is how long to wait for the check to complete (e.g., "3s").
	Timeout string `yaml:"timeout,omitempty"`
	// StartPeriod is the grace period for container initialization (e.g., "10s").
	StartPeriod string `yaml:"startPeriod,omitempty"`
	// Retries is the number of consecutive failures needed to mark unhealthy.
	Retries int `yaml:"retries,omitempty"`
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
	// Shell is the shell to use for remote command execution.
	// Supports string or array syntax (e.g., "bash -e" or ["bash", "-e"]).
	// If not specified, commands are executed directly without shell wrapping.
	Shell types.ShellValue `yaml:"shell,omitempty"`
	// Timeout is the connection timeout duration (e.g., "30s", "1m"). Defaults to 30s.
	Timeout string `yaml:"timeout,omitempty"`
	// Bastion is the jump host / bastion server configuration.
	Bastion *bastion `yaml:"bastion,omitempty"`
}

// bastion defines the bastion/jump host configuration.
type bastion struct {
	// Host is the bastion host address.
	Host string `yaml:"host,omitempty"`
	// Port is the bastion SSH port (can be string or number).
	Port types.PortValue `yaml:"port,omitempty"`
	// User is the bastion SSH user.
	User string `yaml:"user,omitempty"`
	// Key is the path to the SSH private key for the bastion.
	Key string `yaml:"key,omitempty"`
	// Password is the SSH password for the bastion.
	Password string `yaml:"password,omitempty"`
}

// s3Config defines the default S3 configuration for the DAG.
// This allows steps to inherit S3 settings without specifying them individually.
type s3Config struct {
	// Region is the AWS region (e.g., us-east-1).
	Region string `yaml:"region,omitempty"`
	// Endpoint is a custom S3-compatible endpoint URL.
	// Use this for S3-compatible services like MinIO, LocalStack, etc.
	Endpoint string `yaml:"endpoint,omitempty"`
	// AccessKeyID is the AWS access key ID.
	AccessKeyID string `yaml:"accessKeyId,omitempty"`
	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `yaml:"secretAccessKey,omitempty"`
	// SessionToken is the AWS session token (for temporary credentials).
	SessionToken string `yaml:"sessionToken,omitempty"`
	// Profile is the AWS credentials profile name.
	Profile string `yaml:"profile,omitempty"`
	// ForcePathStyle enables path-style addressing (required for S3-compatible services).
	ForcePathStyle bool `yaml:"forcePathStyle,omitempty"`
	// DisableSSL disables SSL for the connection (for local testing only).
	DisableSSL bool `yaml:"disableSSL,omitempty"`
	// Bucket is the default S3 bucket name.
	// Can be overridden at the step level.
	Bucket string `yaml:"bucket,omitempty"`
}

// redisConfig defines the default Redis configuration for all redis steps in the DAG.
// Steps can override these settings by specifying their own config fields.
type redisConfig struct {
	// URL is the Redis connection URL (redis://user:pass@host:port/db).
	URL string `yaml:"url,omitempty"`
	// Host is the Redis host (alternative to URL).
	Host string `yaml:"host,omitempty"`
	// Port is the Redis port (default: 6379).
	Port int `yaml:"port,omitempty"`
	// Password is the authentication password.
	Password string `yaml:"password,omitempty"`
	// Username is the ACL username (Redis 6+).
	Username string `yaml:"username,omitempty"`
	// DB is the database number (0-15).
	DB int `yaml:"db,omitempty"`
	// TLS enables TLS connection.
	TLS bool `yaml:"tls,omitempty"`
	// TLSSkipVerify skips TLS certificate verification.
	TLSSkipVerify bool `yaml:"tlsSkipVerify,omitempty"`
	// Mode is the connection mode (standalone, sentinel, cluster).
	Mode string `yaml:"mode,omitempty"`
	// SentinelMaster is the sentinel master name.
	SentinelMaster string `yaml:"sentinelMaster,omitempty"`
	// SentinelAddrs is the list of sentinel addresses.
	SentinelAddrs []string `yaml:"sentinelAddrs,omitempty"`
	// ClusterAddrs is the list of cluster node addresses.
	ClusterAddrs []string `yaml:"clusterAddrs,omitempty"`
	// MaxRetries is the maximum number of retries.
	MaxRetries int `yaml:"maxRetries,omitempty"`
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
	{"paramsJSON", newTransformer("ParamsJSON", buildParamsJSON)},
	{"workerSelector", &workerSelectorTransformer{}},
	{"timeout", newTransformer("Timeout", buildTimeout)},
	{"delay", newTransformer("Delay", buildDelay)},
	{"restartWait", newTransformer("RestartWait", buildRestartWait)},
	{"maxActiveRuns", newTransformer("MaxActiveRuns", buildMaxActiveRuns)},
	{"maxActiveSteps", newTransformer("MaxActiveSteps", buildMaxActiveSteps)},
	{"queue", newTransformer("Queue", buildQueue)},
	{"maxOutputSize", newTransformer("MaxOutputSize", buildMaxOutputSize)},
	{"skipIfSuccessful", newTransformer("SkipIfSuccessful", buildSkipIfSuccessful)},
	{"catchupWindow", newTransformer("CatchupWindow", buildCatchupWindow)},
	{"overlapPolicy", newTransformer("OverlapPolicy", buildOverlapPolicy)},
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
	{"s3", newTransformer("S3", buildS3)},
	{"llm", newTransformer("LLM", buildLLM)},
	{"redis", newTransformer("Redis", buildRedis)},
	{"secrets", newTransformer("Secrets", buildSecrets)},
	{"dotenv", newTransformer("Dotenv", buildDotenv)},
	{"smtpConfig", newTransformer("SMTP", buildSMTPConfig)},
	{"errMailConfig", newTransformer("ErrorMail", buildErrMailConfig)},
	{"infoMailConfig", newTransformer("InfoMail", buildInfoMailConfig)},
	{"waitMailConfig", newTransformer("WaitMail", buildWaitMailConfig)},
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

	// Initialize shared envScope state for thread-safe env var handling.
	// Start with OS environment as base layer.
	baseScope := eval.NewEnvScope(nil, true)

	// Pre-populate with build env from options (for retry with dotenv).
	// This allows YAML to reference env vars that were loaded from .env files
	// before the rebuild.
	buildEnv := make(map[string]string, len(ctx.opts.BuildEnv))
	maps.Copy(buildEnv, ctx.opts.BuildEnv)
	if len(buildEnv) > 0 {
		baseScope = baseScope.WithEntries(buildEnv, eval.EnvSourceDotEnv)
	}

	ctx.envScope = &envScopeState{
		scope:    baseScope,
		buildEnv: buildEnv,
	}

	// Run the transformer pipeline
	errs := runTransformers(ctx, d, result)

	// Add deprecation warning for maxActiveRuns on local queues
	// Both maxActiveRuns > 1 (concurrency) and maxActiveRuns < 0 (queue bypass) are deprecated
	if result.Queue == "" && (result.MaxActiveRuns > 1 || result.MaxActiveRuns < 0) {
		result.BuildWarnings = append(result.BuildWarnings, fmt.Sprintf(
			"maxActiveRuns=%d is deprecated for local queues and will be ignored. "+
				"Use a global queue with 'queue:' field for concurrency control.",
			result.MaxActiveRuns,
		))
	}

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

	// Validate workerSelector compatibility with HITL steps
	if len(result.WorkerSelector) > 0 && result.HasHITLSteps() {
		errs = append(errs, core.NewValidationError(
			"workerSelector",
			result.WorkerSelector,
			fmt.Errorf("DAG with HITL steps cannot be dispatched to workers"),
		))
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

func buildTags(_ BuildContext, d *dag) (core.Tags, error) {
	if d.Tags.IsZero() {
		return nil, nil
	}
	var tags core.Tags
	for _, entry := range d.Tags.Entries() {
		if entry.Key() == "" {
			continue
		}
		tags = append(tags, core.Tag{
			Key:   strings.ToLower(strings.TrimSpace(entry.Key())),
			Value: strings.ToLower(strings.TrimSpace(entry.Value())),
		})
	}
	return tags, nil
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

func buildCatchupWindow(_ BuildContext, d *dag) (time.Duration, error) {
	if d.CatchupWindow == "" {
		return 0, nil
	}
	return core.ParseDuration(d.CatchupWindow)
}

func buildOverlapPolicy(_ BuildContext, d *dag) (core.OverlapPolicy, error) {
	return core.ParseOverlapPolicy(d.OverlapPolicy)
}

func buildLogDir(_ BuildContext, d *dag) (string, error) {
	return d.LogDir, nil
}

func buildLogOutput(_ BuildContext, d *dag) (core.LogOutputMode, error) {
	if d.LogOutput.IsZero() {
		// Return empty to allow inheritance from base config.
		// Default is applied in core.InitializeDefaults.
		return "", nil
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
		Wait:    d.MailOn.Wait,
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

	// Add vars to the shared envScope state so subsequent transformers can use it.
	// This replaces the old pattern of using os.Setenv which caused race conditions.
	if ctx.envScope != nil && len(vars) > 0 {
		ctx.envScope.scope = ctx.envScope.scope.WithEntries(vars, eval.EnvSourceDAGEnv)
		maps.Copy(ctx.envScope.buildEnv, vars)
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
	ParamsJSON    string // JSON representation of resolved params (original payload when provided as JSON)
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

func buildParamsJSON(ctx BuildContext, d *dag) (string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.ParamsJSON, nil
}

// detectJSONParams checks if the input string is valid JSON and returns it if so.
// Returns empty string if the input is not JSON.
func detectJSONParams(input string) string {
	input = strings.TrimSpace(input)
	if (strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")) ||
		(strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")) {
		var js json.RawMessage
		if json.Unmarshal([]byte(input), &js) == nil {
			return input
		}
	}
	return ""
}

// buildResolvedParamsJSON returns a JSON representation of the resolved params.
// If the raw input was JSON, the original payload is returned to preserve structure.
func buildResolvedParamsJSON(paramPairs []paramPair, rawInput string) (string, error) {
	if rawJSON := detectJSONParams(rawInput); rawJSON != "" {
		return rawJSON, nil
	}
	return marshalParamPairs(paramPairs)
}

// marshalParamPairs converts the final param pairs into a JSON object string.
// Returns an empty string when there are no params to serialize.
func marshalParamPairs(paramPairs []paramPair) (string, error) {
	if len(paramPairs) == 0 {
		return "", nil
	}

	payload := make(map[string]string, len(paramPairs))
	for _, pair := range paramPairs {
		if pair.Name == "" {
			continue
		}
		payload[pair.Name] = pair.Value
	}

	if len(payload) == 0 {
		return "", nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params to JSON: %w", err)
	}
	return string(data), nil
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

	paramsJSON, err := buildResolvedParamsJSON(paramPairs, ctx.opts.Parameters)
	if err != nil {
		return nil, err
	}

	// Note: envs from params are handled separately - they should be appended to Env
	// This is a limitation of the current transformer design; we may need to handle this specially
	_ = envs

	return &paramsResult{
		Params:        params,
		DefaultParams: defaultParams,
		ParamsJSON:    paramsJSON,
	}, nil
}

// workerSelectorTransformer is a custom transformer that sets both WorkerSelector and ForceLocal fields.
type workerSelectorTransformer struct{}

func (t *workerSelectorTransformer) Transform(ctx BuildContext, in *dag, out reflect.Value) error {
	ws, forceLocal, err := buildWorkerSelector(ctx, in)
	if err != nil {
		return err
	}

	if ws != nil {
		wsField := out.FieldByName("WorkerSelector")
		if wsField.IsValid() && wsField.CanSet() {
			wsField.Set(reflect.ValueOf(ws))
		}
	}

	if forceLocal {
		flField := out.FieldByName("ForceLocal")
		if flField.IsValid() && flField.CanSet() {
			flField.SetBool(true)
		}
	}

	return nil
}

func buildWorkerSelector(_ BuildContext, d *dag) (map[string]string, bool, error) {
	if d.WorkerSelector == nil {
		return nil, false, nil
	}

	switch v := d.WorkerSelector.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.EqualFold(trimmed, "local") {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("unsupported workerSelector string value %q; the only allowed string value is \"local\"", trimmed)

	case map[string]string:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			ret[strings.TrimSpace(key)] = strings.TrimSpace(val)
		}
		return ret, false, nil

	case map[string]any:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			ret[strings.TrimSpace(key)] = strings.TrimSpace(fmt.Sprint(val))
		}
		return ret, false, nil

	case map[any]any:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			strKey, ok := key.(string)
			if !ok {
				return nil, false, fmt.Errorf("workerSelector keys must be strings, got %T", key)
			}
			ret[strings.TrimSpace(strKey)] = strings.TrimSpace(fmt.Sprint(val))
		}
		return ret, false, nil

	default:
		return nil, false, fmt.Errorf("workerSelector must be a map or \"local\", got %T", d.WorkerSelector)
	}
}

// shellResult holds both shell and args for internal use
type shellResult struct {
	Shell string
	Args  []string
}

func parseShellInternal(_ BuildContext, d *dag) (*shellResult, error) {
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
		// Shell expansion is deferred to runtime - see runtime/env.go Shell()
		args := d.Shell.Arguments()
		return &shellResult{Shell: shell, Args: args}, nil
	}

	// For string form, need to split command and args
	command := d.Shell.Command()
	if command == "" {
		return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
	}

	// Shell expansion is deferred to runtime - see runtime/env.go Shell()
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
	if d.WorkingDir != "" {
		return resolveWorkingDirPath(d.WorkingDir, ctx.file)
	}
	if ctx.opts.DefaultWorkingDir != "" {
		return ctx.opts.DefaultWorkingDir, nil
	}
	// Return empty to allow inheritance from base config.
	// Default is applied post-merge in loadDAG.
	return "", nil
}

// resolveWorkingDirPath resolves the working directory path at build time.
// Absolute paths, home dir (~), and variable ($) paths are stored as-is for runtime expansion.
// Relative paths are resolved against the DAG file location.
func resolveWorkingDirPath(wd, dagFile string) (string, error) {
	if filepath.IsAbs(wd) || strings.HasPrefix(wd, "~") || strings.HasPrefix(wd, "$") {
		return wd, nil
	}
	if dagFile != "" {
		return filepath.Join(filepath.Dir(dagFile), wd), nil
	}
	return wd, nil
}

// getDefaultWorkingDir returns the current working directory or user home as fallback.
func getDefaultWorkingDir() (string, error) {
	if dir, _ := os.Getwd(); dir != "" {
		return dir, nil
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return dir, nil
}

func buildContainer(ctx BuildContext, d *dag) (*core.Container, error) {
	return buildContainerField(ctx, d.Container)
}

// buildContainerField handles both string and object forms of container field.
// String form: "container-name" -> exec into existing container
// Object form: {image: "...", ...} or {exec: "...", ...} -> create new or exec into existing
func buildContainerField(ctx BuildContext, raw any) (*core.Container, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		// String mode: exec into existing container with defaults
		name := strings.TrimSpace(v)
		if name == "" {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("container name cannot be empty"))
		}
		return &core.Container{
			Exec: name,
		}, nil

	case map[string]any:
		// Object mode: decode and validate
		var c container
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:           &c,
			WeaklyTypedInput: true,
		})
		if err != nil {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("failed to create decoder: %w", err))
		}
		if err := decoder.Decode(v); err != nil {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("failed to decode container: %w", err))
		}
		return buildContainerFromSpec(ctx, &c)

	case *container:
		// Already decoded container struct (for backward compatibility)
		if v == nil {
			return nil, nil
		}
		return buildContainerFromSpec(ctx, v)

	default:
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("container must be a string or object, got %T", raw))
	}
}

// buildContainerFromSpec is a shared function that builds a core.Container from a container spec.
// It is used by both DAG-level and step-level container configuration.
func buildContainerFromSpec(ctx BuildContext, c *container) (*core.Container, error) {
	// Validate mutual exclusivity
	if c.Exec != "" && c.Image != "" {
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("'exec' and 'image' are mutually exclusive"))
	}

	// Require one of exec or image
	if c.Exec == "" && c.Image == "" {
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("either 'exec' or 'image' must be specified"))
	}

	// Handle exec mode
	if c.Exec != "" {
		// Validate no incompatible fields in exec mode
		var invalidFields []string
		if c.Name != "" {
			invalidFields = append(invalidFields, "name")
		}
		if c.PullPolicy != nil {
			invalidFields = append(invalidFields, "pullPolicy")
		}
		if len(c.Volumes) > 0 {
			invalidFields = append(invalidFields, "volumes")
		}
		if len(c.Ports) > 0 {
			invalidFields = append(invalidFields, "ports")
		}
		if c.Network != "" {
			invalidFields = append(invalidFields, "network")
		}
		if c.Platform != "" {
			invalidFields = append(invalidFields, "platform")
		}
		if c.Startup != "" {
			invalidFields = append(invalidFields, "startup")
		}
		if len(c.Command) > 0 {
			invalidFields = append(invalidFields, "command")
		}
		if c.WaitFor != "" {
			invalidFields = append(invalidFields, "waitFor")
		}
		if c.LogPattern != "" {
			invalidFields = append(invalidFields, "logPattern")
		}
		if c.RestartPolicy != "" {
			invalidFields = append(invalidFields, "restartPolicy")
		}
		if c.KeepContainer {
			invalidFields = append(invalidFields, "keepContainer")
		}
		if c.Healthcheck != nil {
			invalidFields = append(invalidFields, "healthcheck")
		}

		if len(invalidFields) > 0 {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("fields %v cannot be used with 'exec'", invalidFields))
		}

		// Parse env for exec mode
		vars, err := loadVariables(ctx, c.Env)
		if err != nil {
			return nil, core.NewValidationError("container.env", c.Env, err)
		}

		var envs []string
		for k, v := range vars {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}

		// Build exec-mode container
		return &core.Container{
			Exec:       strings.TrimSpace(c.Exec),
			User:       c.User,
			WorkingDir: c.WorkingDir,
			Env:        envs,
			Shell:      c.Shell,
		}, nil
	}

	// Handle image mode (existing behavior)
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

	// Parse healthcheck if provided
	var hc *core.Healthcheck
	if c.Healthcheck != nil {
		var err error
		hc, err = parseHealthcheck(c.Healthcheck)
		if err != nil {
			return nil, core.NewValidationError("container.healthcheck", c.Healthcheck, err)
		}
	}

	return &core.Container{
		Name:          strings.TrimSpace(c.Name),
		Image:         c.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       c.Volumes,
		User:          c.User,
		WorkingDir:    c.WorkingDir,
		Platform:      c.Platform,
		Ports:         c.Ports,
		Network:       c.Network,
		KeepContainer: c.KeepContainer,
		Startup:       core.ContainerStartup(strings.ToLower(strings.TrimSpace(c.Startup))),
		Command:       c.Command,
		WaitFor:       core.ContainerWaitFor(strings.ToLower(strings.TrimSpace(c.WaitFor))),
		LogPattern:    c.LogPattern,
		RestartPolicy: strings.TrimSpace(c.RestartPolicy),
		Healthcheck:   hc,
		Shell:         c.Shell,
	}, nil
}

// parseHealthcheck converts a spec healthcheck to a core.Healthcheck with validation.
func parseHealthcheck(h *healthcheck) (*core.Healthcheck, error) {
	if h == nil {
		return nil, nil
	}

	// Validate test field
	if len(h.Test) == 0 {
		return nil, fmt.Errorf("test is required")
	}

	// First element must be a valid command type
	validPrefixes := map[string]bool{
		"NONE":      true,
		"CMD":       true,
		"CMD-SHELL": true,
	}
	if !validPrefixes[h.Test[0]] {
		return nil, fmt.Errorf("test must start with NONE, CMD, or CMD-SHELL, got %q", h.Test[0])
	}

	// NONE should be the only element
	if h.Test[0] == "NONE" && len(h.Test) > 1 {
		return nil, fmt.Errorf("NONE healthcheck should not have additional arguments")
	}

	// CMD and CMD-SHELL need at least one more element (the command)
	if (h.Test[0] == "CMD" || h.Test[0] == "CMD-SHELL") && len(h.Test) < 2 {
		return nil, fmt.Errorf("%s healthcheck requires a command", h.Test[0])
	}

	// Validate retries
	if h.Retries < 0 {
		return nil, fmt.Errorf("retries must be non-negative, got %d", h.Retries)
	}

	hc := &core.Healthcheck{
		Test:    h.Test,
		Retries: h.Retries,
	}

	// Parse duration strings
	if h.Interval != "" {
		d, err := time.ParseDuration(h.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", h.Interval, err)
		}
		hc.Interval = d
	}

	if h.Timeout != "" {
		d, err := time.ParseDuration(h.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", h.Timeout, err)
		}
		hc.Timeout = d
	}

	if h.StartPeriod != "" {
		d, err := time.ParseDuration(h.StartPeriod)
		if err != nil {
			return nil, fmt.Errorf("invalid startPeriod %q: %w", h.StartPeriod, err)
		}
		hc.StartPeriod = d
	}

	return hc, nil
}

func buildRegistryAuths(_ BuildContext, d *dag) (map[string]*core.AuthConfig, error) {
	if d.RegistryAuths == nil {
		return nil, nil
	}

	// No expansion at build time - credentials are evaluated at runtime.
	// See runtime/agent/agent.go where RegistryAuths are evaluated before use.

	// parseAuthConfig parses auth config from a map with string keys.
	parseAuthConfig := func(m map[string]any) *core.AuthConfig {
		cfg := &core.AuthConfig{}
		if v, ok := m["username"].(string); ok {
			cfg.Username = v
		}
		if v, ok := m["password"].(string); ok {
			cfg.Password = v
		}
		if v, ok := m["auth"].(string); ok {
			cfg.Auth = v
		}
		return cfg
	}

	// parseAuthData parses auth data which can be a string or a map.
	parseAuthData := func(authData any) *core.AuthConfig {
		switch auth := authData.(type) {
		case string:
			return &core.AuthConfig{Auth: auth}
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
		default:
			return &core.AuthConfig{}
		}
	}

	registryAuths := make(map[string]*core.AuthConfig)

	switch v := d.RegistryAuths.(type) {
	case string:
		registryAuths["_json"] = &core.AuthConfig{Auth: v}

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

	shell, shellArgs, err := parseSSHShell(d.SSH.Shell)
	if err != nil {
		return nil, err
	}

	return &core.SSHConfig{
		User:          d.SSH.User,
		Host:          d.SSH.Host,
		Port:          defaultPort(d.SSH.Port.String(), "22"),
		Key:           d.SSH.Key,
		Password:      d.SSH.Password,
		StrictHostKey: d.SSH.StrictHostKey == nil || *d.SSH.StrictHostKey,
		KnownHostFile: d.SSH.KnownHostFile,
		Shell:         shell,
		ShellArgs:     shellArgs,
		Timeout:       d.SSH.Timeout,
		Bastion:       buildBastionConfig(d.SSH.Bastion),
	}, nil
}

// parseSSHShell parses shell configuration from ShellValue.
func parseSSHShell(shellVal types.ShellValue) (string, []string, error) {
	if shellVal.IsZero() {
		return "", nil, nil
	}

	command := strings.TrimSpace(shellVal.Command())
	if command == "" {
		return "", nil, nil
	}

	if shellVal.IsArray() {
		return command, shellVal.Arguments(), nil
	}

	parsed, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return "", nil, core.NewValidationError("ssh.shell", shellVal.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	return strings.TrimSpace(parsed), args, nil
}

// buildBastionConfig builds bastion configuration from spec.
func buildBastionConfig(bastion *bastion) *core.BastionConfig {
	if bastion == nil {
		return nil
	}
	return &core.BastionConfig{
		Host:     bastion.Host,
		Port:     defaultPort(bastion.Port.String(), "22"),
		User:     bastion.User,
		Key:      bastion.Key,
		Password: bastion.Password,
	}
}

// defaultPort returns the port if non-empty, otherwise returns the default value.
func defaultPort(port, defaultVal string) string {
	if port == "" {
		return defaultVal
	}
	return port
}

func buildS3(_ BuildContext, d *dag) (*core.S3Config, error) {
	if d.S3 == nil {
		return nil, nil
	}

	return &core.S3Config{
		Region:          d.S3.Region,
		Endpoint:        d.S3.Endpoint,
		AccessKeyID:     d.S3.AccessKeyID,
		SecretAccessKey: d.S3.SecretAccessKey,
		SessionToken:    d.S3.SessionToken,
		Profile:         d.S3.Profile,
		ForcePathStyle:  d.S3.ForcePathStyle,
		DisableSSL:      d.S3.DisableSSL,
		Bucket:          d.S3.Bucket,
	}, nil
}

func buildLLM(_ BuildContext, d *dag) (*core.LLMConfig, error) {
	if d.LLM == nil {
		return nil, nil
	}

	cfg := d.LLM

	// Validate provider if specified (optional at DAG level)
	if cfg.Provider != "" {
		validProviders := map[string]bool{
			"openai": true, "anthropic": true, "gemini": true,
			"openrouter": true, "local": true,
			// Aliases for local provider
			"ollama": true, "vllm": true, "llama": true,
		}
		if !validProviders[cfg.Provider] {
			return nil, core.NewValidationError("llm.provider", cfg.Provider,
				fmt.Errorf("invalid provider: must be one of openai, anthropic, gemini, openrouter, local (or aliases: ollama, vllm, llama)"))
		}
	}

	// Get model string or entries (optional at DAG level)
	var modelString string
	var models []core.ModelEntry

	if !cfg.Model.IsZero() {
		if cfg.Model.IsArray() {
			var err error
			models, err = convertModelEntries(cfg.Model.Entries())
			if err != nil {
				return nil, err
			}
		} else {
			modelString = cfg.Model.String()
		}
	}

	// Validate temperature range if specified
	if cfg.Temperature != nil {
		if *cfg.Temperature < 0.0 || *cfg.Temperature > 2.0 {
			return nil, core.NewValidationError("llm.temperature", *cfg.Temperature,
				fmt.Errorf("temperature must be between 0.0 and 2.0"))
		}
	}

	// Validate topP range if specified
	if cfg.TopP != nil {
		if *cfg.TopP < 0.0 || *cfg.TopP > 1.0 {
			return nil, core.NewValidationError("llm.topP", *cfg.TopP,
				fmt.Errorf("topP must be between 0.0 and 1.0"))
		}
	}

	// Validate maxTokens if specified
	if cfg.MaxTokens != nil {
		if *cfg.MaxTokens < 1 {
			return nil, core.NewValidationError("llm.maxTokens", *cfg.MaxTokens,
				fmt.Errorf("maxTokens must be at least 1"))
		}
	}

	thinking, err := buildThinkingConfig(cfg.Thinking)
	if err != nil {
		return nil, err
	}

	return &core.LLMConfig{
		Provider:    cfg.Provider,
		Model:       modelString,
		Models:      models,
		System:      cfg.System,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		TopP:        cfg.TopP,
		BaseURL:     cfg.BaseURL,
		APIKeyName:  cfg.APIKeyName,
		Stream:      cfg.Stream,
		Thinking:    thinking,
	}, nil
}

func buildRedis(_ BuildContext, d *dag) (*core.RedisConfig, error) {
	if d.Redis == nil {
		return nil, nil
	}

	return &core.RedisConfig{
		URL:            d.Redis.URL,
		Host:           d.Redis.Host,
		Port:           d.Redis.Port,
		Password:       d.Redis.Password,
		Username:       d.Redis.Username,
		DB:             d.Redis.DB,
		TLS:            d.Redis.TLS,
		TLSSkipVerify:  d.Redis.TLSSkipVerify,
		Mode:           d.Redis.Mode,
		SentinelMaster: d.Redis.SentinelMaster,
		SentinelAddrs:  d.Redis.SentinelAddrs,
		ClusterAddrs:   d.Redis.ClusterAddrs,
		MaxRetries:     d.Redis.MaxRetries,
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

	// buildHandler is a helper that builds a single handler step.
	buildHandler := func(s *step, name core.HandlerType) (*core.Step, error) {
		if s == nil {
			return nil, nil
		}
		s.Name = name.String()
		return s.build(buildCtx)
	}

	var err error
	if handlerOn.Init, err = buildHandler(d.HandlerOn.Init, core.HandlerOnInit); err != nil {
		return handlerOn, err
	}
	if handlerOn.Exit, err = buildHandler(d.HandlerOn.Exit, core.HandlerOnExit); err != nil {
		return handlerOn, err
	}
	if handlerOn.Success, err = buildHandler(d.HandlerOn.Success, core.HandlerOnSuccess); err != nil {
		return handlerOn, err
	}
	if handlerOn.Failure, err = buildHandler(d.HandlerOn.Failure, core.HandlerOnFailure); err != nil {
		return handlerOn, err
	}

	if handlerOn.Cancel, err = buildHandler(d.HandlerOn.Abort, core.HandlerOnCancel); err != nil {
		return handlerOn, err
	}

	if handlerOn.Wait, err = buildHandler(d.HandlerOn.Wait, core.HandlerOnWait); err != nil {
		return handlerOn, err
	}

	return handlerOn, nil
}

func buildSMTPConfig(_ BuildContext, d *dag) (*core.SMTPConfig, error) {
	if d.SMTP.IsZero() {
		return nil, nil
	}

	return &core.SMTPConfig{
		Host:     d.SMTP.Host,
		Port:     d.SMTP.Port.String(),
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

func buildWaitMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.WaitMail)
}

func buildPreconditions(ctx BuildContext, d *dag) ([]*core.Condition, error) {
	return parsePrecondition(ctx, d.Preconditions)
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

				if err := validateNoDependsForChainType(result, st); err != nil {
					return nil, err
				}

				if err := validateNoRouterForChainType(result, st); err != nil {
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

						if err := validateNoDependsForChainType(result, st); err != nil {
							return nil, err
						}

						if err := validateNoRouterForChainType(result, st); err != nil {
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
		// Transform router steps: inject preconditions into targets
		if err := transformRouterSteps(steps); err != nil {
			return nil, err
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

			if err := validateNoDependsForChainType(result, builtStep); err != nil {
				return nil, err
			}

			if err := validateNoRouterForChainType(result, builtStep); err != nil {
				return nil, err
			}

			steps = append(steps, *builtStep)
		}
		// Transform router steps: inject preconditions into targets
		if err := transformRouterSteps(steps); err != nil {
			return nil, err
		}
		return steps, nil

	default:
		return nil, core.NewValidationError("steps", v, ErrStepsMustBeArrayOrMap)
	}
}

// buildMailConfigInternal builds a core.MailConfig from the mail configuration.
func buildMailConfigInternal(def mailConfig) (*core.MailConfig, error) {
	if def.IsZero() {
		return nil, nil
	}

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

	return &core.MailConfig{
		From:       strings.TrimSpace(def.From),
		To:         toAddresses,
		Prefix:     strings.TrimSpace(def.Prefix),
		AttachLogs: def.AttachLogs,
	}, nil
}

// validateNoDependsForChainType ensures that steps in chain type DAGs do not have explicit depends.
// Chain type DAGs should have fully automatic sequential execution with no manual dependency control.
func validateNoDependsForChainType(dag *core.DAG, step *core.Step) error {
	if dag.Type != core.TypeChain {
		return nil
	}
	if len(step.Depends) > 0 || step.ExplicitlyNoDeps {
		return core.NewValidationError("depends", step.Depends,
			fmt.Errorf("step '%s': %w", step.Name, core.ErrDependsNotAllowedInChainType))
	}
	return nil
}

// validateNoRouterForChainType returns an error if a router step is used in chain mode
func validateNoRouterForChainType(dag *core.DAG, step *core.Step) error {
	if dag.Type != core.TypeChain {
		return nil
	}
	if step.Router != nil {
		return core.NewValidationError("type", step.Name,
			fmt.Errorf("step '%s': router steps require type 'graph'; change DAG type from 'chain' to 'graph' to use router steps", step.Name))
	}
	return nil
}

// transformRouterSteps processes router-type steps and injects preconditions
// into their target steps. It modifies the steps slice in place.
func transformRouterSteps(steps []core.Step) error {
	// Build step index for lookup (using pointers to modify in place)
	stepIndex := make(map[string]*core.Step)
	for i := range steps {
		stepIndex[steps[i].Name] = &steps[i]
	}

	for i := range steps {
		if steps[i].Router == nil {
			continue
		}

		router := steps[i].Router
		routerName := steps[i].Name

		// Track targets to detect duplicates across routes
		seenTargets := make(map[string]string) // target -> first pattern that used it

		// For each route, inject precondition into target steps
		for _, route := range router.Routes {
			for _, targetName := range route.Targets {
				// Check for duplicate target
				if firstPattern, exists := seenTargets[targetName]; exists {
					return core.NewValidationError("routes", targetName,
						fmt.Errorf("router %q: step %q is targeted by multiple routes (%q and %q); each step can only be a target of one route",
							routerName, targetName, firstPattern, route.Pattern))
				}
				seenTargets[targetName] = route.Pattern

				target, ok := stepIndex[targetName]
				if !ok {
					return core.NewValidationError("routes", targetName,
						fmt.Errorf("router %q references non-existent step %q", routerName, targetName))
				}

				// Inject precondition: check if value matches pattern
				condition := &core.Condition{
					Condition: router.Value,
					Expected:  route.Pattern,
				}
				target.Preconditions = append(target.Preconditions, condition)

				// Add router as dependency if not already present
				if !slices.Contains(target.Depends, routerName) {
					target.Depends = append(target.Depends, routerName)
				}

				// Enable continueOn.skipped for proper flow
				target.ContinueOn.Skipped = true
			}
		}

		// Router itself allows downstream to continue
		steps[i].ContinueOn.Skipped = true
	}

	return nil
}
