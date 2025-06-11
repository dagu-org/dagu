package digraph

// definition is a temporary struct to hold the DAG definition.
// This struct is used to unmarshal the YAML data.
// The data is then converted to the DAG struct.
type definition struct {
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
	// Dotenv is the path to the dotenv file (string or []string).
	Dotenv any
	// Schedule is the cron schedule to run the DAG.
	Schedule any
	// SkipIfSuccessful is the flag to skip the DAG on schedule when it is
	// executed manually before the schedule.
	SkipIfSuccessful bool
	// LogFile is the file to write the log.
	LogDir string
	// Env is the environment variables setting.
	Env any
	// HandlerOn is the handler configuration.
	HandlerOn handlerOnDef
	// Steps is the list of steps to run.
	Steps any // []stepDef or map[string]stepDef
	// SMTP is the SMTP configuration.
	SMTP smtpConfigDef
	// MailOn is the mail configuration.
	MailOn *mailOnDef
	// ErrorMail is the mail configuration for error.
	ErrorMail mailConfigDef
	// InfoMail is the mail configuration for information.
	InfoMail mailConfigDef
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
	Tags any
	// Queue is the name of the queue to assign this DAG to.
	Queue string
	// Resources specifies resource requirements for the DAG.
	Resources *ResourcesConfig
}

// handlerOnDef defines the steps to be executed on different events.
type handlerOnDef struct {
	Failure *stepDef // Step to execute on failure
	Success *stepDef // Step to execute on success
	Cancel  *stepDef // Step to execute on cancel
	Exit    *stepDef // Step to execute on exit
}

// stepDef defines a step in the DAG.
type stepDef struct {
	// Name is the name of the step.
	Name string `yaml:"name,omitempty"`
	// Description is the description of the step.
	Description string `yaml:"description,omitempty"`
	// Dir is the working directory of the step.
	Dir string `yaml:"dir,omitempty"`
	// Executor is the executor configuration.
	Executor any `yaml:"executor,omitempty"`
	// Command is the command to run (on shell).
	Command any `yaml:"command,omitempty"`
	// Shell is the shell to run the command. Default is `$SHELL` or `sh`.
	Shell string `yaml:"shell,omitempty"`
	// Packages is the list of packages to install.
	// This is used only when the shell is `nix-shell`.
	Packages []string `yaml:"packages,omitempty"`
	// Script is the script to run.
	Script string `yaml:"script,omitempty"`
	// Stdout is the file to write the stdout.
	Stdout string `yaml:"stdout,omitempty"`
	// Stderr is the file to write the stderr.
	Stderr string `yaml:"stderr,omitempty"`
	// Output is the variable name to store the output.
	Output string `yaml:"output,omitempty"`
	// Depends is the list of steps to depend on.
	Depends any `yaml:"depends,omitempty"` // string or []string
	// ContinueOn is the condition to continue on.
	ContinueOn *continueOnDef `yaml:"continueOn,omitempty"`
	// RetryPolicy is the retry policy.
	RetryPolicy *retryPolicyDef `yaml:"retryPolicy,omitempty"`
	// RepeatPolicy is the repeat policy.
	RepeatPolicy *repeatPolicyDef `yaml:"repeatPolicy,omitempty"`
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
	// Run is the name of a DAG to run as a child dag-run.
	Run string `yaml:"run,omitempty"`
	// Params specifies the parameters for the child dag-run.
	Params any `yaml:"params,omitempty"`
	// Parallel specifies parallel execution configuration.
	// Can be:
	// - Direct array reference: parallel: ${ITEMS}
	// - Static array: parallel: [item1, item2]
	// - Object configuration: parallel: {items: ${ITEMS}, maxConcurrent: 5}
	Parallel any `yaml:"parallel,omitempty"`
	// Resources specifies resource requirements for the step.
	Resources *ResourcesConfig `yaml:"resources,omitempty"`
}

// continueOnDef defines the conditions to continue on failure or skipped.
type continueOnDef struct {
	Failure     bool // Continue on failure
	Skipped     bool // Continue on skipped
	ExitCode    any  // Continue on specific exit codes
	Output      any  // Continue on specific output (string or []string)
	MarkSuccess bool // Mark the step as success when the condition is met
}

// repeatPolicyDef defines the repeat policy for a step.
type repeatPolicyDef struct {
	Repeat      bool   `yaml:"repeat,omitempty"`      // Flag to indicate if the step should be repeated
	IntervalSec int    `yaml:"intervalSec,omitempty"` // Interval in seconds to wait before repeating the step
	Condition   string `yaml:"condition,omitempty"`   // Condition to check before repeating
	Expected    string `yaml:"expected,omitempty"`    // Expected output to match before repeating
	ExitCode    []int  `yaml:"exitCode,omitempty"`    // List of exit codes to consider for repeating the step
}

// retryPolicyDef defines the retry policy for a step.
type retryPolicyDef struct {
	Limit       any   `yaml:"limit,omitempty"`
	IntervalSec any   `yaml:"intervalSec,omitempty"`
	ExitCode    []int `yaml:"exitCode,omitempty"`
}

// smtpConfigDef defines the SMTP configuration.
type smtpConfigDef struct {
	Host     string // SMTP host
	Port     any    // SMTP port (can be string or number)
	Username string // SMTP username
	Password string // SMTP password
}

// mailConfigDef defines the mail configuration.
type mailConfigDef struct {
	From       string // Sender email address
	To         string // Recipient email address
	Prefix     string // Prefix for the email subject
	AttachLogs bool   // Flag to attach logs to the email
}

// mailOnDef defines the conditions to send mail.
type mailOnDef struct {
	Failure bool // Send mail on failure
	Success bool // Send mail on success
}
