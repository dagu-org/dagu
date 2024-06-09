package dag

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// Step contains the runtime information for a step in a DAG.
// A step is created from parsing a DAG file written in YAML.
// It marshal/unmarshal to/from JSON when it is saved in the execution history.
type Step struct {
	Name            string         `json:"Name"`
	Description     string         `json:"Description,omitempty"`
	Variables       []string       `json:"Variables,omitempty"`
	OutputVariables *SyncMap       `json:"OutputVariables,omitempty"`
	Dir             string         `json:"Dir,omitempty"`
	ExecutorConfig  ExecutorConfig `json:"ExecutorConfig,omitempty"`
	CmdWithArgs     string         `json:"CmdWithArgs,omitempty"`
	Command         string         `json:"Command,omitempty"`
	Script          string         `json:"Script,omitempty"`
	Stdout          string         `json:"Stdout,omitempty"`
	Stderr          string         `json:"Stderr,omitempty"`
	Output          string         `json:"Output,omitempty"`
	Args            []string       `json:"Args,omitempty"`
	Depends         []string       `json:"Depends,omitempty"`
	ContinueOn      ContinueOn     `json:"ContinueOn,omitempty"`
	RetryPolicy     *RetryPolicy   `json:"RetryPolicy,omitempty"`
	RepeatPolicy    RepeatPolicy   `json:"RepeatPolicy,omitempty"`
	MailOnError     bool           `json:"MailOnError,omitempty"`
	Preconditions   []*Condition   `json:"Preconditions,omitempty"`
	SignalOnStop    string         `json:"SignalOnStop,omitempty"`
	SubWorkflow     *SubWorkflow   `json:"SubWorkflow,omitempty"`
}

// SubWorkflow contains information about a sub DAG to be executed.
type SubWorkflow struct {
	Name   string
	Params string
}

// ExecutorConfig contains the configuration for the executor.
type ExecutorConfig struct {
	// Type represents one of the registered executor.
	// See `executor.Register` in `internal/executor/executor.go`.
	Type string
	// Config contains the executor specific configuration.
	Config map[string]interface{}
}

const (
	// ExecutorTypeSubWorkflow is defined here in order to parse
	// the `run` field in the DAG file.
	ExecutorTypeSubWorkflow = "subworkflow"
)

// RetryPolicy contains the retry policy for a step.
type RetryPolicy struct {
	Limit    int
	Interval time.Duration
}

// RepeatPolicy contains the repeat policy for a step.
type RepeatPolicy struct {
	Repeat   bool
	Interval time.Duration
}

// ContinueOn contains the conditions to continue on failure or skipped.
type ContinueOn struct {
	Failure bool
	Skipped bool
}

// String implements the Stringer interface, converting the step to a
// human-readable string.
func (s *Step) String() string {
	vals := []string{
		fmt.Sprintf("Name: %s", s.Name),
		fmt.Sprintf("Dir: %s", s.Dir),
		fmt.Sprintf("Command: %s", s.Command),
		fmt.Sprintf("Args: %s", s.Args),
		fmt.Sprintf("Depends: [%s]", strings.Join(s.Depends, ", ")),
	}
	return strings.Join(vals, "\t")
}

// setup initializes the step's properties.
func (s *Step) setup(defaultLocation string) {
	if s.Dir == "" {
		s.Dir = path.Dir(defaultLocation)
	}
}
