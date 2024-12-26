// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

// definition is a temporary struct to hold the DAG definition.
// This struct is used to unmarshal the YAML data.
// The data is then converted to the DAG struct.
type definition struct {
	Name              string
	Group             string
	Description       string
	Schedule          any
	SkipIfSuccessful  bool
	LogDir            string
	Env               any
	HandlerOn         handlerOnDef
	Functions         []*funcDef // deprecated
	Steps             []stepDef
	SMTP              smtpConfigDef
	MailOn            *mailOnDef
	ErrorMail         mailConfigDef
	InfoMail          mailConfigDef
	TimeoutSec        int
	DelaySec          int
	RestartWaitSec    int
	HistRetentionDays *int
	Preconditions     []*conditionDef
	MaxActiveRuns     int
	Params            string
	MaxCleanUpTimeSec *int
	Tags              any
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
