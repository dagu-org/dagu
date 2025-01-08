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
	// Deprecated: Don't use this field
	Functions []*funcDef // deprecated
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
	// HistRetentionDays is the retention days of the history.
	HistRetentionDays *int
	// Precondition is the condition to run the DAG.
	Precondition any
	// Preconditions is the condition to run the DAG.
	Preconditions any
	// MaxActiveRuns is the maximum number of concurrent steps.
	MaxActiveRuns int
	// Params is the default parameters for the steps.
	Params any
	// MaxCleanUpTimeSec is the maximum time in seconds to clean up the DAG.
	// It is a wait time to kill the processes when it is requested to stop.
	// If the time is exceeded, the process is killed.
	MaxCleanUpTimeSec *int
	// Tags is the tags for the DAG.
	Tags any
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
	Name string
	// Description is the description of the step.
	Description string
	// Dir is the working directory of the step.
	Dir string
	// Executor is the executor configuration.
	Executor any
	// Command is the command to run (on shell).
	Command any
	// Shell is the shell to run the command. Default is `$SHELL` or `sh`.
	Shell string
	// Script is the script to run.
	Script string
	// Stdout is the file to write the stdout.
	Stdout string
	// Stderr is the file to write the stderr.
	Stderr string
	// Output is the variable name to store the output.
	Output string
	// Depends is the list of steps to depend on.
	Depends any // string or []string
	// ContinueOn is the condition to continue on.
	ContinueOn *continueOnDef
	// RetryPolicy is the retry policy.
	RetryPolicy *retryPolicyDef
	// RepeatPolicy is the repeat policy.
	RepeatPolicy *repeatPolicyDef
	// MailOnError is the flag to send mail on error.
	MailOnError bool
	// Precondition is the condition to run the step.
	Precondition any
	// Preconditions is the condition to run the step.
	Preconditions any
	// SignalOnStop is the signal when the step is requested to stop.
	// When it is empty, the same signal as the parent process is sent.
	// It can be KILL when the process does not stop over the timeout.
	SignalOnStop *string
	// Deprecated: Don't use this field
	Call *callFuncDef // deprecated
	// Run is a sub workflow to run
	Run string
	// Params is the parameters for the sub workflow
	Params string
}

// funcDef defines a function in the DAG.
type funcDef struct {
	Name    string // Name of the function
	Params  string // Parameters for the function
	Command string // Command to execute the function
}

// callFuncDef defines a function call in the DAG.
type callFuncDef struct {
	Function string         // Name of the function to call
	Args     map[string]any // Arguments for the function call
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
	Repeat      bool // Flag to indicate if the step should be repeated
	IntervalSec int  // Interval in seconds between repeats
}

// retryPolicyDef defines the retry policy for a step.
type retryPolicyDef struct {
	Limit       any // Limit on the number of retries
	IntervalSec any // Interval in seconds between retries
}

// smtpConfigDef defines the SMTP configuration.
type smtpConfigDef struct {
	Host     string // SMTP host
	Port     string // SMTP port
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
