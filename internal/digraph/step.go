package digraph

import (
	"fmt"
	"strings"
	"time"
)

// Step contains the runtime information for a step in a DAG.
// A step is created from parsing a DAG file written in YAML.
// It marshals/unmarshals to/from JSON when it is saved in the execution history.
type Step struct {
	// Name is the name of the step.
	Name string `json:"name"`
	// Description is the description of the step. This is optional.
	Description string `json:"description,omitempty"`
	// Shell is the shell program to execute the command. This is optional.
	Shell string `json:"shell,omitempty"`
	// Dir is the working directory for the step.
	Dir string `json:"dir,omitempty"`
	// ExecutorConfig contains the configuration for the executor.
	ExecutorConfig ExecutorConfig `json:"executorConfig,omitempty"`
	// CmdWithArgs is the command with arguments (only display purpose).
	CmdWithArgs string `json:"cmdWithArgs,omitempty"`
	// CmdArgsSys is the command with arguments for the system.
	CmdArgsSys string `json:"cmdArgsSys,omitempty"`
	// Command specifies only the command without arguments.
	Command string `json:"command,omitempty"`
	// ShellCmdArgs is the shell command with arguments.
	ShellCmdArgs string `json:"shellCmdArgs,omitempty"`
	// Script is the script to be executed.
	Script string `json:"script,omitempty"`
	// Args contains the arguments for the command.
	Args []string `json:"args,omitempty"`
	// Stdout is the file to store the standard output.
	Stdout string `json:"stdout,omitempty"`
	// Stderr is the file to store the standard error.
	Stderr string `json:"stderr,omitempty"`
	// Output is the variable name to store the output.
	Output string `json:"output,omitempty"`
	// Depends contains the list of step names to depend on.
	Depends []string `json:"depends,omitempty"`
	// ContinueOn contains the conditions to continue on failure or skipped.
	ContinueOn ContinueOn `json:"continueOn,omitempty"`
	// RetryPolicy contains the retry policy for the step.
	RetryPolicy RetryPolicy `json:"retryPolicy,omitempty"`
	// RepeatPolicy contains the repeat policy for the step.
	RepeatPolicy RepeatPolicy `json:"repeatPolicy,omitempty"`
	// MailOnError is the flag to send mail on error.
	MailOnError bool `json:"mailOnError,omitempty"`
	// Preconditions contains the conditions to be met before running the step.
	Preconditions []*Condition `json:"preconditions,omitempty"`
	// SignalOnStop is the signal to send on stop.
	SignalOnStop string `json:"signalOnStop,omitempty"`
	// ChildWorkflow contains the information about a child workflow to be executed.
	ChildWorkflow *ChildWorkflow `json:"childWorkflow,omitempty"`
}

// setup sets the default values for the step.
func (s *Step) setup(workDir string) {
	// If the working directory is not set, use the directory of the DAG file.
	if s.Dir == "" {
		s.Dir = workDir
	}
}

// String returns a formatted string representation of the step
func (s *Step) String() string {
	fields := []struct {
		name  string
		value string
	}{
		{"Name", s.Name},
		{"Dir", s.Dir},
		{"Command", s.Command},
		{"Args", fmt.Sprintf("%v", s.Args)},
		{"Depends", fmt.Sprintf("[%s]", strings.Join(s.Depends, ", "))},
	}

	var parts []string
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field.name, field.value))
	}

	return strings.Join(parts, "\t")
}

// ChildWorkflow contains information about a child workflow to be executed.
type ChildWorkflow struct {
	Name   string `json:"name,omitempty"`
	Params string `json:"params,omitempty"`
}

// ExecutorTypeSubLegacy is defined here in order to parse
// the `run` field in the DAG file.
const ExecutorTypeSubLegacy = "subworkflow"
const ExecutorTypeSub = "sub"

// ExecutorConfig contains the configuration for the executor.
type ExecutorConfig struct {
	// Type represents one of the registered executors.
	// See `executor.Register` in `internal/executor/executor.go`.
	Type   string         `json:"type,omitempty"`
	Config map[string]any `json:"config,omitempty"` // Config contains executor-specific configuration.
}

// IsCommand returns true if the executor is a command
func (e ExecutorConfig) IsCommand() bool {
	return e.Type == "" || e.Type == "command"
}

// RetryPolicy contains the retry policy for a step.
type RetryPolicy struct {
	// Limit is the number of retries allowed.
	Limit int `json:"limit,omitempty"`
	// Interval is the time to wait between retries.
	Interval time.Duration `json:"interval,omitempty"`
	// LimitStr is the string representation of the limit.
	LimitStr string `json:"limitStr,omitempty"`
	// IntervalSecStr is the string representation of the interval.
	IntervalSecStr string `json:"intervalSecStr,omitempty"`
	// ExitCodes is the list of exit codes that should trigger a retry.
	ExitCodes []int `json:"exitCode,omitempty"`
}

// RepeatPolicy contains the repeat policy for a step.
type RepeatPolicy struct {
	// Repeat determines if the step should be repeated.
	Repeat bool `json:"repeat,omitempty"`
	// Interval is the time to wait between repeats.
	Interval time.Duration `json:"interval,omitempty"`
}

// ContinueOn contains the conditions to continue on failure or skipped.
// Failure is the flag to continue to the next step on failure.
// Skipped is the flag to continue to the next step on skipped.
// A step can be skipped when the preconditions are not met.
// Then if the ContinueOn.Skip is set, the step will continue to the next step.
type ContinueOn struct {
	Failure     bool     `json:"failure,omitempty"`     // Failure is the flag to continue to the next step on failure.
	Skipped     bool     `json:"skipped,omitempty"`     // Skipped is the flag to continue to the next step on skipped.
	ExitCode    []int    `json:"exitCode,omitempty"`    // ExitCode is the list of exit codes to continue to the next step.
	Output      []string `json:"output,omitempty"`      // Output is the list of output (stdout/stderr) to continue to the next step.
	MarkSuccess bool     `json:"markSuccess,omitempty"` // MarkSuccess is the flag to mark the step as success when the condition is met.
}
