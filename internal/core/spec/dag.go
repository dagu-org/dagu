package spec

import "github.com/dagu-org/dagu/internal/core/spec/types"

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
