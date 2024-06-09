package dag

import (
	"fmt"
	"path"
	"strings"
	"time"
)

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

// Step contains the runtime information for a step in a DAG.
// A step is created from parsing a DAG file written in YAML.
// It marshal/unmarshal to/from JSON when it is saved in the execution history.
type Step struct {
	Name            string         `json:"Name"`                      // Name is the name of the step.
	Description     string         `json:"Description,omitempty"`     // Description is the description of the step.
	Variables       []string       `json:"Variables,omitempty"`       // Variables contains the list of variables to be set.
	OutputVariables *SyncMap       `json:"OutputVariables,omitempty"` // OutputVariables is a structure to store the output variables for the following steps.
	Dir             string         `json:"Dir,omitempty"`             // Dir is the working directory for the step.
	ExecutorConfig  ExecutorConfig `json:"ExecutorConfig,omitempty"`  // ExecutorConfig contains the configuration for the executor.
	CmdWithArgs     string         `json:"CmdWithArgs,omitempty"`     // CmdWithArgs is the command with arguments.
	Command         string         `json:"Command,omitempty"`         // Command specifies only the command without arguments.
	Script          string         `json:"Script,omitempty"`          // Script is the script to be executed.
	Args            []string       `json:"Args,omitempty"`            // Args contains the arguments for the command.
	Stdout          string         `json:"Stdout,omitempty"`          // Stdout is the file to store the standard output.
	Stderr          string         `json:"Stderr,omitempty"`          // Stderr is the file to store the standard error.
	Output          string         `json:"Output,omitempty"`          // Output is the variable name to store the output.
	Depends         []string       `json:"Depends,omitempty"`         // Depends contains the list of step names to depend on.
	ContinueOn      ContinueOn     `json:"ContinueOn,omitempty"`      // ContinueOn contains the conditions to continue on failure or skipped.
	RetryPolicy     *RetryPolicy   `json:"RetryPolicy,omitempty"`     // RetryPolicy contains the retry policy for the step.
	RepeatPolicy    RepeatPolicy   `json:"RepeatPolicy,omitempty"`    // RepeatPolicy contains the repeat policy for the step.
	MailOnError     bool           `json:"MailOnError,omitempty"`     // MailOnError is the flag to send mail on error.
	Preconditions   []*Condition   `json:"Preconditions,omitempty"`   // Preconditions contains the conditions to be met before running the step.
	SignalOnStop    string         `json:"SignalOnStop,omitempty"`    // SignalOnStop is the signal to send on stop.
	SubWorkflow     *SubWorkflow   `json:"SubWorkflow,omitempty"`     // SubWorkflow contains the information about a sub DAG to be executed.
}

// setup sets the default values for the step.
func (s *Step) setup(workDir string) {
	// if the working directory is not set, use the directory of the DAG file.
	if s.Dir == "" {
		s.Dir = path.Dir(workDir)
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
