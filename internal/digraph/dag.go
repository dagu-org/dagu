package digraph

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

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// Constants for configuration defaults
const (
	defaultDAGRunRetentionDays = 30
	defaultMaxCleanUpTime      = 60 * time.Second
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

// DAG contains all information about a DAG.
type DAG struct {
	// WorkingDir is the working directory to run the DAG.
	// Default value is the directory of DAG file.
	// If the source is not a DAG file, current directory when it's created.
	WorkingDir string `json:"workingDir,omitempty"`
	// Location is the absolute path to the DAG file.
	// It is used to generate unix socket name and can be blank
	Location string `json:"location,omitempty"`
	// Group is the group name of the DAG. This is optional.
	Group string `json:"group,omitempty"`
	// Name is the name of the DAG. The default is the filename without the extension.
	Name string `json:"name,omitempty"`
	// Type is the execution type (graph, chain, or agent). Default is graph.
	Type string `json:"type,omitempty"`
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
	Env []string `json:"env,omitempty"`
	// LogDir is the directory where the logs are stored.
	LogDir string `json:"logDir,omitempty"`
	// DefaultParams contains the default parameters to be passed to the DAG.
	DefaultParams string `json:"defaultParams,omitempty"`
	// Params contains the list of parameters to be passed to the DAG.
	Params []string `json:"params,omitempty"`
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
	// If specified, the DAG will only run on workers with matching labels.
	WorkerSelector map[string]string `json:"workerSelector,omitempty"`
	// MaxOutputSize is the maximum size of step output to capture in bytes.
	// Default is 1MB. Output exceeding this will return an error.
	MaxOutputSize int `json:"maxOutputSize,omitempty"`
	// OTel contains the OpenTelemetry configuration for the DAG.
	OTel *OTelConfig `json:"otel,omitempty"`
	// BuildErrors contains any errors encountered while building the DAG.
	BuildErrors []error
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
	RegistryAuths map[string]*AuthConfig `json:"registryAuths,omitempty"`
	// SSH contains the default SSH configuration for the DAG.
	SSH *SSHConfig `json:"ssh,omitempty"`
}

// CreateTask creates a coordinator task from this DAG for distributed execution.
// It constructs a task with the given operation and run ID, setting the DAG's name
// as both the root DAG and target, and includes the DAG's YAML definition.
//
// Parameters:
//   - op: The operation type (START or RETRY)
//   - runID: The unique identifier for this DAG run
//   - opts: Optional task modifiers (e.g., WithTaskParams, WithWorkerSelector)
//
// Example:
//
//	task := dag.CreateTask(
//	    coordinatorv1.Operation_OPERATION_START,
//	    "run-123",
//	    digraph.WithTaskParams("env=prod"),
//	    digraph.WithWorkerSelector(map[string]string{"gpu": "true"}),
//	)
func (d *DAG) CreateTask(
	op coordinatorv1.Operation,
	runID string,
	opts ...TaskOption,
) *coordinatorv1.Task {
	task := &coordinatorv1.Task{
		RootDagRunName: d.Name,
		RootDagRunId:   runID,
		Operation:      op,
		DagRunId:       runID,
		Target:         d.Name,
		Definition:     string(d.YamlData),
	}

	for _, opt := range opts {
		opt(task)
	}

	return task
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

// SockAddr returns the unix socket address for the DAG.
// The address is used to communicate with the agent process.
func (d *DAG) SockAddr(dagRunID string) string {
	if d.Location != "" {
		return SockAddr(d.Location, "")
	}
	return SockAddr(d.Name, dagRunID)
}

// SockAddrForChildDAGRun returns the unix socket address for a specific dag-run ID.
// This is used to control child dag-runs.
func (d *DAG) SockAddrForChildDAGRun(dagRunID string) string {
	return SockAddr(d.GetName(), dagRunID)
}

// GetName returns the name of the DAG.
// If the name is not set, it returns the default name (filename without extension).
func (d *DAG) GetName() string {
	name := d.Name
	if name != "" {
		return name
	}
	return defaultName(d.Location)
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
					wrapError("depends", dep, fmt.Errorf("step %s depends on non-existent step", step.Name)),
				}
				return errList
			}
		}
	}

	return nil
}

// LoadEnv loads required environment variable
func (d *DAG) LoadEnv(ctx context.Context) {
	// Load dotenv
	d.loadDotEnv(ctx, []string{d.WorkingDir})

	// Load env
	for _, env := range d.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			_ = os.Setenv(parts[0], parts[1])
		}
	}
}

// loadDotEnv loads dotenv file
func (d *DAG) loadDotEnv(ctx context.Context, relativeTos []string) {
	resolver := fileutil.NewFileResolver(relativeTos)
	candidates := append([]string{".env"}, d.Dotenv...)

	for _, filePath := range candidates {
		if strings.TrimSpace(filePath) == "" {
			continue
		}
		filePath, err := cmdutil.EvalString(ctx, filePath)
		if err != nil {
			logger.Warn(ctx, "Failed to eval filepath", "filePath", filePath, "err", err)
			continue
		}
		resolvedPath, err := resolver.ResolveFilePath(filePath)
		if err != nil {
			continue
		}
		if !fileutil.FileExists(resolvedPath) {
			continue
		}
		if err := godotenv.Load(resolvedPath); err != nil {
			logger.Warn(ctx, "Failed to load .env", "file", resolvedPath, "err", err)
			continue
		}

		// Load the first found one
		return
	}
}

// initializeDefaults sets the default values for the DAG.
func (d *DAG) initializeDefaults() {
	// Set the name if not set.
	if d.Name == "" {
		d.Name = defaultName(d.Location)
	}

	// Set default type to chain if not specified.
	if d.Type == "" {
		d.Type = TypeChain
	}

	// Set default history retention days to 30 if not specified.
	if d.HistRetentionDays == 0 {
		d.HistRetentionDays = defaultDAGRunRetentionDays
	}

	// Set default max cleanup time to 60 seconds if not specified.
	if d.MaxCleanUpTime == 0 {
		d.MaxCleanUpTime = defaultMaxCleanUpTime
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

// ParamsMap returns the parameters as a map.
func (d *DAG) ParamsMap() map[string]string {
	params := make(map[string]string)
	for _, p := range d.Params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}
	return params
}

// TaskOption is a function that modifies a coordinatorv1.Task.
type TaskOption func(*coordinatorv1.Task)

// WithRootDagRun sets the root DAG run name and ID in the task.
func WithRootDagRun(ref DAGRunRef) TaskOption {
	return func(task *coordinatorv1.Task) {
		if ref.Name == "" || ref.ID == "" {
			return // No root DAG run reference provided
		}
		task.RootDagRunName = ref.Name
		task.RootDagRunId = ref.ID
	}
}

// WithParentDagRun sets the parent DAG run name and ID in the task.
func WithParentDagRun(ref DAGRunRef) TaskOption {
	return func(task *coordinatorv1.Task) {
		if ref.Name == "" || ref.ID == "" {
			return // No parent DAG run reference provided
		}
		task.ParentDagRunName = ref.Name
		task.ParentDagRunId = ref.ID
	}
}

// WithTaskParams sets the parameters for the task.
func WithTaskParams(params string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.Params = params
	}
}

// WithWorkerSelector sets the worker selector labels for the task.
func WithWorkerSelector(selector map[string]string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.WorkerSelector = selector
	}
}

// WithStep sets the step name for retry operations.
func WithStep(step string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.Step = step
	}
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
	Failure *Step `json:"failure,omitempty"`
	Success *Step `json:"success,omitempty"`
	Cancel  *Step `json:"cancel,omitempty"`
	Exit    *Step `json:"exit,omitempty"`
}

// MailOn contains the conditions to send mail.
type MailOn struct {
	Failure bool `json:"failure,omitempty"`
	Success bool `json:"success,omitempty"`
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
	HandlerOnSuccess HandlerType = "onSuccess"
	HandlerOnFailure HandlerType = "onFailure"
	HandlerOnCancel  HandlerType = "onCancel"
	HandlerOnExit    HandlerType = "onExit"
)

func (h HandlerType) String() string {
	return string(h)
}

// ParseHandlerType converts a string to a HandlerType.
func ParseHandlerType(s string) HandlerType {
	return handlerMapping[s]
}

var handlerMapping = map[string]HandlerType{
	"onSuccess": HandlerOnSuccess,
	"onFailure": HandlerOnFailure,
	"onCancel":  HandlerOnCancel,
	"onExit":    HandlerOnExit,
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
