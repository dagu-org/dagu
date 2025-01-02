// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
	Steps []stepDef
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
	Preconditions []*conditionDef
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

type conditionDef struct {
	Condition string
	Expected  string
}

type handlerOnDef struct {
	Failure *stepDef
	Success *stepDef
	Cancel  *stepDef
	Exit    *stepDef
}

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
	Depends []string
	// ContinueOn is the condition to continue on.
	ContinueOn *continueOnDef
	// RetryPolicy is the retry policy.
	RetryPolicy *retryPolicyDef
	// RepeatPolicy is the repeat policy.
	RepeatPolicy *repeatPolicyDef
	// MailOnError is the flag to send mail on error.
	MailOnError bool
	// Precondition is the condition to run the step.
	Preconditions []*conditionDef
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

type funcDef struct {
	Name    string
	Params  string
	Command string
}

type callFuncDef struct {
	Function string
	Args     map[string]any
}

type continueOnDef struct {
	Failure bool
	Skipped bool
}

type repeatPolicyDef struct {
	Repeat      bool
	IntervalSec int
}

type retryPolicyDef struct {
	Limit       any
	IntervalSec any
}

type smtpConfigDef struct {
	Host     string
	Port     string
	Username string
	Password string
}

type mailConfigDef struct {
	From       string
	To         string
	Prefix     string
	AttachLogs bool
}

type mailOnDef struct {
	Failure bool
	Success bool
}
