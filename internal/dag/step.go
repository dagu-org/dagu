package dag

import (
	"fmt"
	"strings"
	"time"
)

// Step contains the runtime information for a step in a DAG.
// A step is created from parsing a DAG file written in YAML.
// It marshal/unmarshal to/from JSON when it is saved in the execution history.
type Step struct {
	// Name is the name of the step.
	Name string `json:"Name"`
	// Description is the description of the step.
	Description string `json:"Description,omitempty"`
	// Variables contains the list of variables to be set.
	Variables []string `json:"Variables,omitempty"`
	// OutputVariables is a structure to store the output variables for the
	// following steps.
	OutputVariables *SyncMap `json:"OutputVariables,omitempty"`
	// Dir is the working directory for the step.
	Dir string `json:"Dir,omitempty"`
	// ExecutorConfig contains the configuration for the executor.
	ExecutorConfig ExecutorConfig `json:"ExecutorConfig,omitempty"`
	// CmdWithArgs is the command with arguments.
	CmdWithArgs string `json:"CmdWithArgs,omitempty"`
	// Command specifies only the command without arguments.
	Command string `json:"Command,omitempty"`
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
	RetryPolicy *RetryPolicy `json:"RetryPolicy,omitempty"`
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
	// if the working directory is not set, use the directory of the DAG file.
	if s.Dir == "" {
		s.Dir = workDir
	}
}

// String implements the Stringer interface.
// TODO: Remove if not needed.
func (s *Step) String() string {
	values := []string{
		fmt.Sprintf("Name: %s", s.Name),
		fmt.Sprintf("Dir: %s", s.Dir),
		fmt.Sprintf("Command: %s", s.Command),
		fmt.Sprintf("Args: %s", s.Args),
		fmt.Sprintf("Depends: [%s]", strings.Join(s.Depends, ", ")),
	}

	return strings.Join(values, "\t")
}

// SubWorkflow contains information about a sub DAG to be executed.
type SubWorkflow struct {
	Name   string
	Params string
}

// ExecutorTypeSubWorkflow is defined here in order to parse
// the `run` field in the DAG file.
const ExecutorTypeSubWorkflow = "subworkflow"

// ExecutorConfig contains the configuration for the executor.
type ExecutorConfig struct {
	// Type represents one of the registered executor.
	// See `executor.Register` in `internal/executor/executor.go`.
	Type   string
	Config map[string]any // Config contains executor specific configuration.
}

// RetryPolicy contains the retry policy for a step.
type RetryPolicy struct {
	Limit    int           // Limit is the number of retries allowed.
	Interval time.Duration // Interval is the time to wait between retries.
}

// RepeatPolicy contains the repeat policy for a step.
type RepeatPolicy struct {
	Repeat   bool          // Repeat determines if the step should be repeated.
	Interval time.Duration // Interval is the time to wait between repeats.
}

// ContinueOn contains the conditions to continue on failure or skipped.
// Failure is the flag to continue to the next step on failure.
// Skipped is the flag to continue to the next step on skipped.
// A step can be skipped when the preconditions are not met.
// Then if the ContinueOn.Skip is set, the step will continue to the next step.
type ContinueOn struct {
	Failure bool // Failure is the flag to continue to the next step on failure.
	Skipped bool // Skipped is the flag to continue to the next step on skipped.
}
