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
	Name string `json:"Name"`
	// Description is the description of the step. This is optional.
	Description string `json:"Description,omitempty"`
	// Shell is the shell program to execute the command. This is optional.
	Shell string `json:"Shell,omitempty"`
	// OutputVariables stores the output variables for the following steps.
	// It only contains the local output variables.
	OutputVariables *SyncMap `json:"OutputVariables,omitempty"`
	// Dir is the working directory for the step.
	Dir string `json:"Dir,omitempty"`
	// ExecutorConfig contains the configuration for the executor.
	ExecutorConfig ExecutorConfig `json:"ExecutorConfig,omitempty"`
	// CmdWithArgs is the command with arguments (only display purpose).
	CmdWithArgs string `json:"CmdWithArgs,omitempty"`
	// CmdArgsSys is the command with arguments for the system.
	CmdArgsSys string `json:"CmdArgsSys,omitempty"`
	// Command specifies only the command without arguments.
	Command string `json:"Command,omitempty"`
	// ShellCmdArgs is the shell command with arguments.
	ShellCmdArgs string `json:"ShellCmdArgs,omitempty"`
	// Script is the script to be executed.
	Script string `json:"Script,omitempty"`
	// Args contains the arguments for the command.
	Args []string `json:"Args,omitempty"`
	// Stdout is the file to store the standard output.
	Stdout string `json:"Stdout,omitempty"`
	// Stderr is the file to store the standard error.
	Stderr string `json:"Stderr,omitempty"`
	// Output is the variable name to store the output.
	Output string `json:"Output,omitempty"`
	// Depends contains the list of step names to depend on.
	Depends []string `json:"Depends,omitempty"`
	// ContinueOn contains the conditions to continue on failure or skipped.
	ContinueOn ContinueOn `json:"ContinueOn,omitempty"`
	// RetryPolicy contains the retry policy for the step.
	RetryPolicy RetryPolicy `json:"RetryPolicy,omitempty"`
	// RepeatPolicy contains the repeat policy for the step.
	RepeatPolicy RepeatPolicy `json:"RepeatPolicy,omitempty"`
	// MailOnError is the flag to send mail on error.
	MailOnError bool `json:"MailOnError,omitempty"`
	// Preconditions contains the conditions to be met before running the step.
	Preconditions []Condition `json:"Preconditions,omitempty"`
	// SignalOnStop is the signal to send on stop.
	SignalOnStop string `json:"SignalOnStop,omitempty"`
	// SubWorkflow contains the information about a sub DAG to be executed.
	SubWorkflow *SubWorkflow `json:"SubWorkflow,omitempty"`
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

// SubWorkflow contains information about a sub DAG to be executed.
type SubWorkflow struct {
	Name   string `json:"Name,omitempty"`
	Params string `json:"Params,omitempty"`
}

// ExecutorTypeSubWorkflow is defined here in order to parse
// the `run` field in the DAG file.
const ExecutorTypeSubWorkflow = "subworkflow"

// ExecutorConfig contains the configuration for the executor.
type ExecutorConfig struct {
	// Type represents one of the registered executors.
	// See `executor.Register` in `internal/executor/executor.go`.
	Type   string         `json:"Type,omitempty"`
	Config map[string]any `json:"Config,omitempty"` // Config contains executor-specific configuration.
}

// IsCommand returns true if the executor is a command
func (e ExecutorConfig) IsCommand() bool {
	return e.Type == "" || e.Type == "command"
}

// RetryPolicy contains the retry policy for a step.
type RetryPolicy struct {
	// Limit is the number of retries allowed.
	Limit int `json:"Limit,omitempty"`
	// Interval is the time to wait between retries.
	Interval time.Duration `json:"Interval,omitempty"`
	// LimitStr is the string representation of the limit.
	LimitStr string `json:"LimitStr,omitempty"`
	// IntervalSecStr is the string representation of the interval.
	IntervalSecStr string `json:"IntervalSecStr,omitempty"`
}

// RepeatPolicy contains the repeat policy for a step.
type RepeatPolicy struct {
	// Repeat determines if the step should be repeated.
	Repeat bool `json:"Repeat,omitempty"`
	// Interval is the time to wait between repeats.
	Interval time.Duration `json:"Interval,omitempty"`
}

// ContinueOn contains the conditions to continue on failure or skipped.
// Failure is the flag to continue to the next step on failure.
// Skipped is the flag to continue to the next step on skipped.
// A step can be skipped when the preconditions are not met.
// Then if the ContinueOn.Skip is set, the step will continue to the next step.
type ContinueOn struct {
	Failure     bool     `json:"Failure,omitempty"`     // Failure is the flag to continue to the next step on failure.
	Skipped     bool     `json:"Skipped,omitempty"`     // Skipped is the flag to continue to the next step on skipped.
	ExitCode    []int    `json:"ExitCode,omitempty"`    // ExitCode is the list of exit codes to continue to the next step.
	Output      []string `json:"Output,omitempty"`      // Output is the list of output (stdout/stderr) to continue to the next step.
	MarkSuccess bool     `json:"MarkSuccess,omitempty"` // MarkSuccess is the flag to mark the step as success when the condition is met.
}
