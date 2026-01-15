package core

import (
	"context"
	"crypto/md5" // nolint:gosec
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// Execution type constants
const (
	// TypeGraph is the default execution type using dependency-based execution
	TypeGraph = "graph"
	// TypeChain executes steps sequentially in the order they are defined
	TypeChain = "chain"
	// TypeAgent is reserved for future agent-based execution
	TypeAgent = "agent"
)

// LogOutputMode represents the mode for log output handling.
// It determines how stdout and stderr are written to log files.
type LogOutputMode string

const (
	// LogOutputSeparate keeps stdout and stderr in separate files (.out and .err).
	// This is the default behavior for backward compatibility.
	LogOutputSeparate LogOutputMode = "separate"

	// LogOutputMerged combines stdout and stderr into a single log file (.log).
	// Both streams are interleaved in the order they are written.
	LogOutputMerged LogOutputMode = "merged"
)

// EffectiveLogOutput returns the effective log output mode for a step.
// It resolves the inheritance chain: step-level overrides DAG-level,
// and if neither is set, returns the default (LogOutputSeparate).
func EffectiveLogOutput(dag *DAG, step *Step) LogOutputMode {
	// Step-level override takes precedence
	if step != nil && step.LogOutput != "" {
		return step.LogOutput
	}
	// Fall back to DAG-level setting
	if dag != nil && dag.LogOutput != "" {
		return dag.LogOutput
	}
	// Default to separate
	return LogOutputSeparate
}

// DAG contains all information about a DAG.
type DAG struct {
	// WorkingDir is the working directory to run the DAG.
	// Default value is the directory of DAG file.
	// If the source is not a DAG file, current directory when it's created.
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	WorkingDir string `json:"-"`
	// Location is the absolute path to the DAG file.
	// It is used to generate unix socket name and can be blank
	Location string `json:"location,omitempty"`
	// Group is the group name of the DAG. This is optional.
	Group string `json:"group,omitempty"`
	// Name is the name of the DAG. The default is the filename without the extension.
	Name string `json:"name,omitempty"`
	// Type is the execution type (graph, chain, or agent). Default is graph.
	Type string `json:"type,omitempty"`
	// Shell is the default shell to use for all steps in this DAG.
	// If not specified, the system default shell is used.
	// Can be overridden at the step level.
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	Shell string `json:"-"`
	// ShellArgs contains additional arguments to pass to the shell.
	// These are populated when Shell is specified as a string with arguments
	// (e.g., "bash -e") or as an array (e.g., ["bash", "-e"]).
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	ShellArgs []string `json:"-"`
	// Dotenv is the path to the dotenv file. This is optional.
	Dotenv []string `json:"dotenv,omitempty"`
	// Tags contains the list of tags for the DAG. This is optional.
	Tags []string `json:"tags,omitempty"`
	// Description is the description of the DAG. This is optional.
	Description string `json:"description,omitempty"`
	// Schedule configuration for starting, stopping, and restarting the DAG.
	Schedule []Schedule `json:"schedule,omitempty"`
	// StopSchedule contains the cron expressions for stopping the DAG.
	StopSchedule []Schedule `json:"stopSchedule,omitempty"`
	// RestartSchedule contains the cron expressions for restarting the DAG.
	RestartSchedule []Schedule `json:"restartSchedule,omitempty"`
	// SkipIfSuccessful indicates whether to skip the DAG if it was successful previously.
	// E.g., when the DAG has already been executed manually before the scheduled time.
	SkipIfSuccessful bool `json:"skipIfSuccessful,omitempty"`
	// Env contains a list of environment variables to be set before running the DAG.
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	Env []string `json:"-"`
	// LogDir is the directory where the logs are stored.
	LogDir string `json:"logDir,omitempty"`
	// LogOutput specifies how stdout and stderr are handled in log files.
	// Can be "separate" (default) for separate .out and .err files,
	// or "merged" for a single combined .log file.
	LogOutput LogOutputMode `json:"logOutput,omitempty"`
	// DefaultParams contains the default parameters to be passed to the DAG.
	DefaultParams string `json:"defaultParams,omitempty"`
	// Params contains the list of parameters to be passed to the DAG.
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	Params []string `json:"-"`
	// ParamsJSON contains the JSON representation of the resolved parameters.
	// When params were supplied as JSON, the original payload is preserved.
	// Steps can consume this via the DAGU_PARAMS_JSON environment variable.
	// Note: This field is evaluated at build time and may contain secrets.
	// It is excluded from JSON serialization to prevent secret leakage.
	ParamsJSON string `json:"-"`
	// Steps contains the list of steps in the DAG.
	Steps []Step `json:"steps,omitempty"`
	// HandlerOn contains the steps to be executed on different events.
	HandlerOn HandlerOn `json:"handlerOn,omitzero"`
	// Preconditions contains the conditions to be met before running the DAG.
	Preconditions []*Condition `json:"preconditions,omitempty"`
	// SMTP contains the SMTP configuration.
	SMTP *SMTPConfig `json:"smtp,omitempty"`
	// ErrorMail contains the mail configuration for errors.
	ErrorMail *MailConfig `json:"errorMail,omitempty"`
	// InfoMail contains the mail configuration for informational messages.
	InfoMail *MailConfig `json:"infoMail,omitempty"`
	// WaitMail contains the mail configuration for wait status.
	WaitMail *MailConfig `json:"waitMail,omitempty"`
	// MailOn contains the conditions to send mail.
	MailOn *MailOn `json:"mailOn,omitempty"`
	// Timeout specifies the maximum execution time of the DAG task.
	Timeout time.Duration `json:"timeout,omitempty"`
	// Delay is the delay before starting the DAG.
	Delay time.Duration `json:"delay,omitempty"`
	// RestartWait is the time to wait before restarting the DAG.
	RestartWait time.Duration `json:"restartWait,omitempty"`
	// MaxActiveSteps specifies the maximum concurrent steps to run in an execution.
	MaxActiveSteps int `json:"maxActiveSteps,omitempty"`
	// MaxActiveRuns specifies the maximum number of concurrent dag-runs.
	MaxActiveRuns int `json:"maxActiveRuns,omitempty"`
	// MaxCleanUpTime is the maximum time to wait for cleanup when the DAG is stopped.
	MaxCleanUpTime time.Duration `json:"maxCleanUpTime,omitempty"`
	// HistRetentionDays is the number of days to keep the history of dag-runs.
	HistRetentionDays int `json:"histRetentionDays,omitempty"`
	// Queue is the name of the queue to assign this DAG to.
	Queue string `json:"queue,omitempty"`
	// WorkerSelector defines labels required for worker selection in distributed execution.
	// If specified, the DAG will only run on workers with matching tag.
	WorkerSelector map[string]string `json:"workerSelector,omitempty"`
	// MaxOutputSize is the maximum size of step output to capture in bytes.
	// Default is 1MB. Output exceeding this will return an error.
	MaxOutputSize int `json:"maxOutputSize,omitempty"`
	// OTel contains the OpenTelemetry configuration for the DAG.
	OTel *OTelConfig `json:"otel,omitempty"`
	// BuildErrors contains any errors encountered while building the DAG.
	BuildErrors []error `json:"-"`
	// BuildWarnings contains non-fatal warnings detected while building the DAG.
	BuildWarnings []string `json:"-"`
	// LocalDAGs contains DAGs defined in the same file, keyed by DAG name
	LocalDAGs map[string]*DAG `json:"localDAGs,omitempty"`
	// YamlData contains the raw YAML data of the DAG.
	YamlData []byte `json:"yamlData,omitempty"`
	// Container contains the container definition for the DAG.
	Container *Container `json:"container,omitempty"`
	// RunConfig contains configuration for controlling user interactions during DAG runs.
	RunConfig *RunConfig `json:"runConfig,omitempty"`
	// RegistryAuths maps registry hostnames to authentication configs.
	// Optional: If not specified, falls back to DOCKER_AUTH_CONFIG or docker config.
	// Note: This field contains sensitive credentials and is evaluated at build time.
	// It is excluded from JSON serialization to prevent secret leakage.
	RegistryAuths map[string]*AuthConfig `json:"-"`
	// SSH contains the default SSH configuration for the DAG.
	SSH *SSHConfig `json:"ssh,omitempty"`
	// LLM contains the default LLM configuration for the DAG.
	// Steps with type: chat inherit this configuration if they don't specify their own llm field.
	LLM *LLMConfig `json:"llm,omitempty"`
	// Secrets contains references to external secrets to be resolved at runtime.
	Secrets []SecretRef `json:"secrets,omitempty"`
}

// SecretRef represents a reference to an external secret.
// Secrets are resolved at DAG execution time and never persisted to disk.
type SecretRef struct {
	// Name is the environment variable name to set (required).
	Name string `json:"name"`
	// Provider specifies the secret backend (e.g., "env", "file", "gcp-secrets") (required).
	Provider string `json:"provider"`
	// Key is the provider-specific identifier for the secret (required).
	Key string `json:"key"`
	// Options contains provider-specific configuration (optional).
	Options map[string]string `json:"options,omitempty"`
}

// HasTag checks if the DAG has the given tag.
func (d *DAG) HasTag(tag string) bool {
	for _, t := range d.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasHITLSteps returns true if the DAG contains any HITL executor steps.
// DAGs with HITL steps cannot be dispatched to workers because
// HITL steps require local storage access for approval.
func (d *DAG) HasHITLSteps() bool {
	for _, step := range d.Steps {
		if step.ExecutorConfig.Type == ExecutorTypeHITL {
			return true
		}
	}
	return false
}

// SockAddr returns the unix socket address for the DAG.
// The address is used to communicate with the agent process.
func (d *DAG) SockAddr(dagRunID string) string {
	if d.Location != "" {
		return SockAddr(d.Location, "")
	}
	return SockAddr(d.Name, dagRunID)
}

// SockAddrForSubDAGRun returns the unix socket address for a specific dag-run ID.
// This is used to control sub dag-runs.
func (d *DAG) SockAddrForSubDAGRun(dagRunID string) string {
	return SockAddr(d.GetName(), dagRunID)
}

// GetName returns the name of the DAG.
// If the name is not set, it returns the default name (filename without extension).
func (d *DAG) GetName() string {
	name := d.Name
	if name != "" {
		return name
	}
	filename := filepath.Base(d.Location)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// String implements the Stringer interface.
// String returns a formatted string representation of the DAG
func (d *DAG) String() string {
	var sb strings.Builder

	sb.WriteString("{\n")
	fmt.Fprintf(&sb, "\tName: %s\n", d.Name)
	fmt.Fprintf(&sb, "\tDescription: %s\n", strings.TrimSpace(d.Description))
	fmt.Fprintf(&sb, "\tParams: %v\n", strings.Join(d.Params, ", "))
	fmt.Fprintf(&sb, "\tLogDir: %v\n", d.LogDir)

	for i, step := range d.Steps {
		fmt.Fprintf(&sb, "\tStep%d: %s\n", i, step.String())
	}

	sb.WriteString("}\n")
	return sb.String()
}

// Validate performs basic validation of the DAG structure
func (d *DAG) Validate() error {
	// If Name is not set, return an error
	if d.Name == "" {
		return fmt.Errorf("DAG name is required")
	}

	// Ensure all referenced steps exist
	stepMap := make(map[string]bool)
	for _, step := range d.Steps {
		stepMap[step.Name] = true
	}

	// Check dependencies
	for _, step := range d.Steps {
		for _, dep := range step.Depends {
			if !stepMap[dep] {
				var errList error = ErrorList{
					NewValidationError("depends", dep, fmt.Errorf("step %s depends on non-existent step", step.Name)),
				}
				return errList
			}
		}
	}

	return nil
}

// NextRun returns the next scheduled run time based on the DAG's schedules.
func (d *DAG) NextRun(now time.Time) time.Time {
	if len(d.Schedule) == 0 {
		return time.Time{}
	}
	var next time.Time
	for _, sched := range d.Schedule {
		if sched.Parsed != nil {
			t := sched.Parsed.Next(now)
			if next.IsZero() || t.Before(next) {
				next = t
			}
		}
	}
	return next
}

// deduplicateStrings removes duplicate strings while preserving order.
func deduplicateStrings(input []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(input))
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// LoadDotEnv loads all dotenv files in order, with later files overriding earlier ones.
func (d *DAG) LoadDotEnv(ctx context.Context) {
	if len(d.Dotenv) == 0 {
		return
	}

	relativeTos := []string{d.WorkingDir}
	fileDir := filepath.Dir(d.Location)
	if d.Location != "" && fileDir != d.WorkingDir {
		relativeTos = append(relativeTos, fileDir)
	}

	resolver := fileutil.NewFileResolver(relativeTos)
	candidates := deduplicateStrings(append([]string{".env"}, d.Dotenv...))

	for _, filePath := range candidates {
		if strings.TrimSpace(filePath) == "" {
			continue
		}
		filePath, err := cmdutil.EvalString(ctx, filePath)
		if err != nil {
			logger.Warn(ctx, "Failed to evaluate filepath",
				tag.File(filePath),
				tag.Error(err),
			)
			continue
		}
		resolvedPath, err := resolver.ResolveFilePath(filePath)
		if err != nil {
			continue
		}
		if !fileutil.FileExists(resolvedPath) {
			continue
		}
		// Use godotenv.Read instead of godotenv.Load to avoid os.Setenv
		vars, err := godotenv.Read(resolvedPath)
		if err != nil {
			logger.Warn(ctx, "Failed to load .env file",
				tag.File(resolvedPath),
				tag.Error(err),
			)
			continue
		}
		// Add dotenv vars to DAG.Env so they're included in AllEnvs()
		// Note: We no longer call os.Setenv to avoid race conditions when
		// multiple DAGs are loaded concurrently. The vars are stored in
		// d.Env and will be passed to step execution via cmd.Env.
		for k, v := range vars {
			d.Env = append(d.Env, fmt.Sprintf("%s=%s", k, v))
		}
		logger.Info(ctx, "Loaded dotenv file", tag.File(resolvedPath))
	}
}

// initializeDefaults sets the default values for the DAG.
func (d *DAG) initializeDefaults() {
	// Set default type to chain if not specified.
	if d.Type == "" {
		d.Type = TypeChain
	}

	// Set default log output mode to separate if not specified.
	if d.LogOutput == "" {
		d.LogOutput = LogOutputSeparate
	}

	// Set default history retention days to 30 if not specified.
	if d.HistRetentionDays == 0 {
		d.HistRetentionDays = 30
	}

	// Set default max cleanup time to 60 seconds if not specified.
	if d.MaxCleanUpTime == 0 {
		d.MaxCleanUpTime = 5 * time.Second
	}

	// Set default max active runs to 1 only when not specified (0).
	// MaxActiveRuns < 0 means queueing is disabled for this DAG.
	if d.MaxActiveRuns == 0 {
		d.MaxActiveRuns = 1
	}

	// Set default max output size to 1MB if not specified.
	if d.MaxOutputSize == 0 {
		d.MaxOutputSize = 1024 * 1024 // 1MB
	}
}

// InitializeDefaults exposes initializeDefaults for packages that prepare DAGs before execution.
func InitializeDefaults(d *DAG) {
	d.initializeDefaults()
}

// ParamsMap returns the parameters as a map.
func (d *DAG) ParamsMap() map[string]string {
	params := make(map[string]string)
	for _, p := range d.Params {
		key, value, found := strings.Cut(p, "=")
		if found {
			params[key] = value
		}
	}
	return params
}

// ProcGroup returns the name of the process group for this DAG.
// The process group name is used to identify and manage related DAG executions.
//
// Returns:
//   - If Queue is set: returns the Queue value
//   - If Queue is empty: returns the DAG name as the default
//
// The process group name is used by the process store to:
//  1. Manage heartbeat files for active DAG runs
//  2. Enforce concurrency limits (max concurrent runs) across DAGs in the same group
//
// This allows the scheduler to control how many DAGs can run simultaneously
// within the same process group.
func (d *DAG) ProcGroup() string {
	// If the queue is not set, return the default queue name.
	if d.Queue != "" {
		return d.Queue
	}
	return d.Name
}

// FileName returns the filename of the DAG without the extension.
func (d *DAG) FileName() string {
	if d.Location == "" {
		return ""
	}
	return fileutil.TrimYAMLFileExtension(filepath.Base(d.Location))
}

// AuthConfig represents Docker registry authentication configuration.
// This is a simplified structure for user convenience that will be
// converted to Docker's registry.AuthConfig format when needed.
type AuthConfig struct {
	// Username for registry authentication
	Username string `json:"username,omitempty"`
	// Password for registry authentication
	Password string `json:"password,omitempty"`
	// Auth can be used instead of username/password for pre-encoded credentials
	// This should be base64(username:password)
	Auth string `json:"auth,omitempty"`
}

// RunConfig contains configuration for controlling user interactions during DAG runs.
type RunConfig struct {
	// DisableParamEdit when set to true, prevents users from editing parameters when starting a DAG.
	DisableParamEdit bool `json:"disableParamEdit,omitempty"`
	// DisableRunIdEdit when set to true, prevents users from specifying custom run IDs.
	DisableRunIdEdit bool `json:"disableRunIdEdit,omitempty"`
}

// SSHConfig contains the SSH configuration for the DAG.
type SSHConfig struct {
	// User is the SSH user.
	User string `json:"user,omitempty"`
	// Host is the SSH host.
	Host string `json:"host,omitempty"`
	// Port is the SSH port. Default is "22".
	Port string `json:"port,omitempty"`
	// Key is the path to the SSH private key.
	Key string `json:"key,omitempty"`
	// Password is the SSH password.
	Password string `json:"password,omitempty"`
	// StrictHostKey enables strict host key checking. Defaults to true.
	StrictHostKey bool `json:"strictHostKey,omitempty"`
	// KnownHostFile is the path to the known_hosts file. Defaults to ~/.ssh/known_hosts.
	KnownHostFile string `json:"knownHostFile,omitempty"`
	// Shell is the shell to use for remote command execution.
	// If not specified, commands are executed directly without shell wrapping.
	Shell string `json:"shell,omitempty"`
	// ShellArgs contains additional arguments that should be passed to the shell executable.
	ShellArgs []string `json:"shellArgs,omitempty"`
}

// Schedule contains the cron expression and the parsed cron schedule.
type Schedule struct {
	// Expression is the cron expression.
	Expression string `json:"expression"`
	// Parsed is the parsed cron schedule.
	Parsed cron.Schedule `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (s Schedule) MarshalJSON() ([]byte, error) {
	// Create a temporary struct for marshaling
	type ScheduleAlias struct {
		Expression string `json:"expression"`
	}

	return json.Marshal(ScheduleAlias{
		Expression: s.Expression,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// and also parses the cron expression to populate the Parsed field.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	// Create a temporary struct for unmarshaling
	type ScheduleAlias struct {
		Expression string `json:"expression"`
	}

	var alias ScheduleAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	s.Expression = alias.Expression

	// Parse the cron expression to populate the Parsed field
	if s.Expression != "" {
		parsed, err := cron.ParseStandard(s.Expression)
		if err != nil {
			return fmt.Errorf("invalid cron expression %q: %w", s.Expression, err)
		}
		s.Parsed = parsed
	}

	return nil
}

// HandlerOn contains the steps to be executed on different events in the DAG.
type HandlerOn struct {
	Init    *Step `json:"init,omitempty"`
	Failure *Step `json:"failure,omitempty"`
	Success *Step `json:"success,omitempty"`
	Cancel  *Step `json:"cancel,omitempty"`
	Exit    *Step `json:"exit,omitempty"`
	Wait    *Step `json:"wait,omitempty"`
}

// MailOn contains the conditions to send mail.
type MailOn struct {
	Failure bool `json:"failure,omitempty"`
	Success bool `json:"success,omitempty"`
	Wait    bool `json:"wait,omitempty"`
}

// SMTPConfig contains the SMTP configuration.
type SMTPConfig struct {
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// MailConfig contains the mail configuration.
type MailConfig struct {
	From       string   `json:"from,omitempty"`
	To         []string `json:"to,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
	AttachLogs bool     `json:"attachLogs,omitempty"`
}

// OTelConfig contains the OpenTelemetry configuration.
type OTelConfig struct {
	Enabled  bool              `json:"enabled,omitempty"`
	Endpoint string            `json:"endpoint,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Insecure bool              `json:"insecure,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty"`
	Resource map[string]any    `json:"resource,omitempty"`
}

// HandlerType is the type of the handler.
type HandlerType string

const (
	HandlerOnInit    HandlerType = "onInit"
	HandlerOnSuccess HandlerType = "onSuccess"
	HandlerOnFailure HandlerType = "onFailure"
	HandlerOnCancel  HandlerType = "onCancel"
	HandlerOnExit    HandlerType = "onExit"
	HandlerOnWait    HandlerType = "onWait"
)

func (h HandlerType) String() string {
	return string(h)
}

// SockAddr returns the unix socket address for the DAG.
// The address is used to communicate with the agent process.
func SockAddr(name, dagRunID string) string {
	// Create MD5 hash of the combined name and dag-run ID and take first 6 chars
	hashLength := 6
	hash := fmt.Sprintf("%x", md5.Sum([]byte(name+dagRunID)))[:hashLength] // nolint:gosec

	maxSocketNameLength := 50 // Maximum length for socket name
	name = fileutil.SafeName(name)

	// Calculate the total length with the full name
	prefix := "@dagu_"
	connector := "_"
	suffix := ".sock"
	totalLen := len(prefix) + len(name) + len(connector) + len(hash) + len(suffix)

	// Truncate name only if the total length exceeds maxSocketNameLength (50)
	if totalLen > maxSocketNameLength {
		// Calculate how much to truncate
		excessLen := totalLen - maxSocketNameLength
		nameLen := len(name) - excessLen
		name = name[:nameLen]
	}

	// Build the socket name
	socketName := prefix + name + connector + hash + suffix

	return getSocketPath(socketName)
}

// getSocketPath returns the appropriate socket path for the current platform.
// On Unix systems, it uses /tmp directory.
// On Windows, it uses the system temp directory.
func getSocketPath(socketName string) string {
	if runtime.GOOS == "windows" {
		// On Windows, use the system temp directory instead of /tmp
		return filepath.Join(os.TempDir(), socketName)
	}
	// On Unix systems, use /tmp
	return filepath.Join("/tmp", socketName)
}

// EvaluateFromYamlIfNeeded re-evaluates sensitive fields from YamlData.
// This is called AFTER LoadDotEnv() to ensure correct precedence.
//
// For NORMAL execution: Fields are already populated from build → returns immediately
// For RETRY: Fields are empty (json:"-") → re-parses YamlData and evaluates
func (d *DAG) EvaluateFromYamlIfNeeded(ctx context.Context) error {
	// Check if fields are already populated (normal execution)
	if d.hasEvaluatedFields() {
		return nil // Nothing to do - fields already populated from build
	}

	// Fields are empty (retry case) - re-evaluate from YamlData
	if len(d.YamlData) == 0 {
		return nil // No YAML data to evaluate
	}

	return d.evaluateFromYaml(ctx)
}

// hasEvaluatedFields checks if sensitive fields are populated.
// WorkingDir is particularly important as it's required for execution.
// We check WorkingDir explicitly because it may need to be computed
// even when other fields like Params are already populated (e.g., rescheduling).
func (d *DAG) hasEvaluatedFields() bool {
	// WorkingDir must be populated for execution - this is the key indicator
	// that evaluation has happened. Other fields might be populated through
	// other means (e.g., Params via reschedule from status).
	if d.WorkingDir == "" {
		return false
	}
	return true
}

// rawDAGFields is used to unmarshal sensitive fields from YamlData.
type rawDAGFields struct {
	Env           any    `yaml:"env"`
	Params        any    `yaml:"params"`
	Shell         any    `yaml:"shell"`
	WorkingDir    string `yaml:"workingDir"`
	RegistryAuths []struct {
		Host     string `yaml:"host"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Auth     string `yaml:"auth"`
	} `yaml:"registryAuths"`
	Container *struct {
		Env any `yaml:"env"`
	} `yaml:"container"`
}

// evaluateFromYaml re-parses YamlData and evaluates all sensitive fields.
// This is called when loading a DAG from dag.json during retry, where
// sensitive fields are empty (excluded via json:"-").
func (d *DAG) evaluateFromYaml(ctx context.Context) error {
	var raw rawDAGFields
	if err := yaml.Unmarshal(d.YamlData, &raw); err != nil {
		return fmt.Errorf("failed to parse yaml: %w", err)
	}

	// Build env map starting with any dotenv values already in d.Env
	// (LoadDotEnv was called before EvaluateFromYamlIfNeeded)
	envVars := make(map[string]string)
	for _, env := range d.Env {
		if key, value, found := strings.Cut(env, "="); found {
			envVars[key] = value
		}
	}

	// === EVALUATE ENV ===
	if raw.Env != nil {
		evaluated, err := d.evaluateEnvFromRaw(ctx, raw.Env, envVars)
		if err != nil {
			return fmt.Errorf("failed to evaluate env: %w", err)
		}
		for key, value := range evaluated {
			if _, exists := envVars[key]; !exists {
				// Don't override dotenv values
				d.Env = append(d.Env, key+"="+value)
				envVars[key] = value
			}
		}
	}

	// === EVALUATE PARAMS (can reference env) ===
	// Only evaluate from YAML if params weren't already provided (e.g., via reschedule)
	if raw.Params != nil && len(d.Params) == 0 {
		evaluated, err := d.evaluateParamsFromRaw(ctx, raw.Params, envVars)
		if err != nil {
			return fmt.Errorf("failed to evaluate params: %w", err)
		}
		d.Params = evaluated
		d.ParamsJSON = buildParamsJSON(evaluated)
	}

	// === EVALUATE SHELL ===
	if raw.Shell != nil {
		shell, shellArgs := d.evaluateShellFromRaw(ctx, raw.Shell, envVars)
		d.Shell = shell
		d.ShellArgs = shellArgs
	}

	// === EVALUATE WORKINGDIR ===
	if raw.WorkingDir != "" {
		d.WorkingDir = expandWithEnv(ctx, raw.WorkingDir, envVars)
	} else if d.WorkingDir == "" && d.Location != "" {
		// Default to the directory containing the DAG file
		// This mirrors the behavior in spec/dag.go transformer
		d.WorkingDir = filepath.Dir(d.Location)
	}

	// === EVALUATE REGISTRY AUTHS (CRITICAL - contains credentials) ===
	for _, auth := range raw.RegistryAuths {
		if d.RegistryAuths == nil {
			d.RegistryAuths = make(map[string]*AuthConfig)
		}
		d.RegistryAuths[auth.Host] = &AuthConfig{
			Username: expandWithEnv(ctx, auth.Username, envVars),
			Password: expandWithEnv(ctx, auth.Password, envVars),
			Auth:     expandWithEnv(ctx, auth.Auth, envVars),
		}
	}

	// === EVALUATE CONTAINER ENV ===
	if d.Container != nil && raw.Container != nil && raw.Container.Env != nil {
		containerEnv, err := d.evaluateEnvFromRaw(ctx, raw.Container.Env, envVars)
		if err != nil {
			return fmt.Errorf("failed to evaluate container env: %w", err)
		}
		for key, value := range containerEnv {
			d.Container.Env = append(d.Container.Env, key+"="+value)
		}
	}

	return nil
}

// expandWithEnv expands ${VAR} and $VAR references using the env map.
func expandWithEnv(ctx context.Context, s string, envVars map[string]string) string {
	if s == "" {
		return s
	}
	scope := cmdutil.NewEnvScope(nil, true)
	for k, v := range envVars {
		scope.Set(k, v, cmdutil.EnvSourceDAGEnv)
	}
	return cmdutil.ExpandEnvContext(cmdutil.WithEnvScope(ctx, scope), s)
}

// evaluateEnvFromRaw parses raw env (map or array) and evaluates values.
// Returns a map of key→value with all variables expanded and commands substituted.
func (d *DAG) evaluateEnvFromRaw(ctx context.Context, raw any, baseVars map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	// Collect all entries from the raw input
	var entries []struct{ Key, Value string }
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				for k, val := range m {
					entries = append(entries, struct{ Key, Value string }{k, fmt.Sprintf("%v", val)})
				}
			} else if s, ok := item.(string); ok {
				if key, value, found := strings.Cut(s, "="); found {
					entries = append(entries, struct{ Key, Value string }{key, value})
				}
			}
		}
	case map[string]any:
		for k, val := range v {
			entries = append(entries, struct{ Key, Value string }{k, fmt.Sprintf("%v", val)})
		}
	}

	// Build an env scope with baseVars + accumulated results
	for _, entry := range entries {
		if _, exists := baseVars[entry.Key]; exists {
			// Skip if already set by dotenv
			continue
		}

		// Build scope with baseVars + results so far
		scope := cmdutil.NewEnvScope(nil, true)
		for k, v := range baseVars {
			scope.Set(k, v, cmdutil.EnvSourceDotEnv)
		}
		for k, v := range result {
			scope.Set(k, v, cmdutil.EnvSourceDAGEnv)
		}
		evalCtx := cmdutil.WithEnvScope(ctx, scope)

		// Merge baseVars and result for variable substitution
		allVars := make(map[string]string)
		for k, v := range baseVars {
			allVars[k] = v
		}
		for k, v := range result {
			allVars[k] = v
		}

		// Evaluate the value (handles command substitution and variable expansion)
		evaluated, err := cmdutil.EvalString(evalCtx, entry.Value, cmdutil.WithVariables(allVars))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate %q: %w", entry.Key, err)
		}
		result[entry.Key] = evaluated
	}

	return result, nil
}

// evaluateParamsFromRaw parses raw params and evaluates with env available.
// Returns a slice of "NAME=value" strings.
func (d *DAG) evaluateParamsFromRaw(ctx context.Context, raw any, envVars map[string]string) ([]string, error) {
	var params []string
	accumulatedVars := make(map[string]string)

	// Copy envVars as the base
	for k, v := range envVars {
		accumulatedVars[k] = v
	}

	// Build scope with env vars
	scope := cmdutil.NewEnvScope(nil, true)
	for k, v := range envVars {
		scope.Set(k, v, cmdutil.EnvSourceDAGEnv)
	}
	evalCtx := cmdutil.WithEnvScope(ctx, scope)

	// Parse params based on type
	var entries []struct{ Name, Value string }
	switch v := raw.(type) {
	case string:
		// Space-separated or key=value format
		entries = parseParamString(v)
	case []any:
		for _, item := range v {
			switch val := item.(type) {
			case string:
				entries = append(entries, parseParamString(val)...)
			case map[string]any:
				for k, vv := range val {
					entries = append(entries, struct{ Name, Value string }{k, fmt.Sprintf("%v", vv)})
				}
			}
		}
	case map[string]any:
		for k, vv := range v {
			entries = append(entries, struct{ Name, Value string }{k, fmt.Sprintf("%v", vv)})
		}
	}

	// Evaluate each param with accumulated vars
	for i, entry := range entries {
		evaluated, err := cmdutil.EvalString(evalCtx, entry.Value, cmdutil.WithVariables(accumulatedVars))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate param %q: %w", entry.Name, err)
		}

		paramName := entry.Name
		if paramName == "" {
			paramName = fmt.Sprintf("%d", i+1)
		}

		params = append(params, paramName+"="+evaluated)
		accumulatedVars[paramName] = evaluated
	}

	return params, nil
}

// parseParamString parses a parameter string into name-value pairs.
func parseParamString(s string) []struct{ Name, Value string } {
	var result []struct{ Name, Value string }
	s = strings.TrimSpace(s)
	if s == "" {
		return result
	}

	// Check for key=value format
	if key, value, found := strings.Cut(s, "="); found {
		result = append(result, struct{ Name, Value string }{key, value})
	} else {
		// Positional parameter
		result = append(result, struct{ Name, Value string }{"", s})
	}
	return result
}

// evaluateShellFromRaw evaluates the shell field which can be a string or array.
func (d *DAG) evaluateShellFromRaw(ctx context.Context, raw any, envVars map[string]string) (string, []string) {
	switch v := raw.(type) {
	case string:
		expanded := expandWithEnv(ctx, v, envVars)
		// Check if shell has arguments (e.g., "bash -e")
		parts := strings.Fields(expanded)
		if len(parts) > 1 {
			return parts[0], parts[1:]
		}
		return expanded, nil
	case []any:
		var shell string
		var args []string
		for i, item := range v {
			if s, ok := item.(string); ok {
				expanded := expandWithEnv(ctx, s, envVars)
				if i == 0 {
					shell = expanded
				} else {
					args = append(args, expanded)
				}
			}
		}
		return shell, args
	}
	return "", nil
}

// buildParamsJSON creates the JSON representation of params.
func buildParamsJSON(params []string) string {
	if len(params) == 0 {
		return ""
	}
	paramMap := make(map[string]string)
	for _, p := range params {
		if key, value, found := strings.Cut(p, "="); found {
			paramMap[key] = value
		}
	}
	data, err := json.Marshal(paramMap)
	if err != nil {
		return ""
	}
	return string(data)
}
