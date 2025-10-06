package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Step contains the runtime information for a step in a DAG.
// A step is created from parsing a DAG file written in YAML.
// It marshals/unmarshals to/from JSON when it is saved in the execution history.
type Step struct {
	// ID is the optional unique identifier for the step.
	ID string `json:"id,omitempty"`
	// Name is the name of the step.
	Name string `json:"name"`
	// Description is the description of the step. This is optional.
	Description string `json:"description,omitempty"`
	// Shell is the shell program to execute the command. This is optional.
	Shell string `json:"shell,omitempty"`
	// ShellPackages is the list of packages to install. This is used only when the shell is `nix-shell`.
	ShellPackages []string `json:"shellPackages,omitempty"`
	// Dir is the working directory for the step.
	Dir string `json:"dir,omitempty"`
	// ExecutorConfig contains the configuration for the executor.
	ExecutorConfig ExecutorConfig `json:"executorConfig,omitzero"`
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
	// ExplicitlyNoDeps indicates the depends field was explicitly set to empty
	ExplicitlyNoDeps bool `json:"-"`
	// ContinueOn contains the conditions to continue on failure or skipped.
	ContinueOn ContinueOn `json:"continueOn,omitzero"`
	// RetryPolicy contains the retry policy for the step.
	RetryPolicy RetryPolicy `json:"retryPolicy,omitzero"`
	// RepeatPolicy contains the repeat policy for the step.
	RepeatPolicy RepeatPolicy `json:"repeatPolicy,omitzero"`
	// MailOnError is the flag to send mail on error.
	MailOnError bool `json:"mailOnError,omitempty"`
	// Preconditions contains the conditions to be met before running the step.
	Preconditions []*Condition `json:"preconditions,omitempty"`
	// SignalOnStop is the signal to send on stop.
	SignalOnStop string `json:"signalOnStop,omitempty"`
	// ChildDAG contains the information about a child DAG to be executed.
	ChildDAG *ChildDAG `json:"childDag,omitempty"`
	// WorkerSelector specifies required worker labels for execution.
	WorkerSelector map[string]string `json:"workerSelector,omitempty"`
	// Parallel contains the configuration for parallel execution.
	Parallel *ParallelConfig `json:"parallel,omitempty"`
	// Env contains environment variables for the step.
	Env []string `json:"env,omitempty"`
	// Uses specifies a GitHub Action to run (e.g., "actions/checkout@v4")
	Uses string `json:"uses,omitempty"`
	// With contains input parameters for the GitHub Action
	With map[string]string `json:"with,omitempty"`
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

// ChildDAG contains information about a child DAG to be executed.
type ChildDAG struct {
	Name   string `json:"name,omitempty"`
	Params string `json:"params,omitempty"`
}

// ExecutorConfig contains the configuration for the executor.
type ExecutorConfig struct {
	// Type represents one of the registered executors.
	// See `executor.Register` in `internal/executor/executor.go`.
	Type   string         `json:"type,omitempty"`
	Config map[string]any `json:"config,omitempty"` // Config contains executor-specific configuration.
	// Metadata contains additional metadata for the executor that is not passed to the executor itself.
	// This is used internally for optimization purposes.
	Metadata map[string]any `json:"metadata,omitempty"`
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
	// Backoff is the exponential backoff multiplier (e.g., 2.0 for doubling).
	Backoff float64 `json:"backoff,omitempty"`
	// MaxInterval is the maximum interval cap for exponential backoff.
	MaxInterval time.Duration `json:"maxInterval,omitempty"`
}

// RepeatMode is the type for the repeat mode.
type RepeatMode string

const (
	// RepeatModeWhile repeats the step while the condition is met.
	RepeatModeWhile RepeatMode = "while"
	// RepeatModeUntil repeats the step until the condition is met.
	RepeatModeUntil RepeatMode = "until"
)

// RepeatPolicy contains the repeat policy for a step.
type RepeatPolicy struct {
	// RepeatMode determines if and how the step should be repeated.
	// It can be 'while' or 'until'.
	RepeatMode RepeatMode `json:"repeatMode,omitempty"`
	// Interval is the time to wait between repeats.
	Interval time.Duration `json:"interval,omitempty"`
	// Limit is the maximum number of times to repeat the step.
	Limit int `json:"limit,omitempty"`
	// Backoff is the exponential backoff multiplier (e.g., 2.0 for doubling).
	Backoff float64 `json:"backoff,omitempty"`
	// MaxInterval is the maximum interval cap for exponential backoff.
	MaxInterval time.Duration `json:"maxInterval,omitempty"`
	// Condition is the condition object to be met for the repeat.
	Condition *Condition `json:"condition,omitempty"`
	// ExitCode is the list of exit codes that should trigger a repeat.
	ExitCode []int `json:"exitCode,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for RepeatPolicy.
// It handles the legacy boolean repeat field and the new string repeat modes.
func (r *RepeatPolicy) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid infinite recursion
	type Alias RepeatPolicy

	// First, unmarshal into the alias to get the new format fields
	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Copy the fields
	r.RepeatMode = aux.RepeatMode
	r.Interval = aux.Interval
	r.Limit = aux.Limit
	r.Condition = aux.Condition
	r.ExitCode = aux.ExitCode
	r.Backoff = aux.Backoff
	r.MaxInterval = aux.MaxInterval

	// If RepeatMode is already set, we're done (new format)
	if r.RepeatMode != "" {
		return nil
	}

	// Otherwise, check for legacy format
	var legacy struct {
		Repeat bool `json:"repeat"`
	}

	if err := json.Unmarshal(data, &legacy); err == nil && data != nil {
		// Successfully parsed legacy format
		if legacy.Repeat {
			// Legacy repeat: true -> while mode
			r.RepeatMode = RepeatModeWhile
		} else {
			// Legacy repeat: false -> infer based on conditions
			if r.Condition != nil && r.Condition.Expected != "" {
				// Condition with expected value -> "until" mode
				r.RepeatMode = RepeatModeUntil
			} else if r.Condition != nil || len(r.ExitCode) > 0 {
				// Just condition or exit code -> "while" mode
				r.RepeatMode = RepeatModeWhile
			}
			// Otherwise leave RepeatMode empty (no repeat)
		}
	}

	return nil
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

const (
	// ExecutorTypeDAG is the executor type for a child DAG.
	ExecutorTypeDAG = "dag"

	// ExecutorTypeParallel is the executor type for parallel steps.
	ExecutorTypeParallel = "parallel"
)
