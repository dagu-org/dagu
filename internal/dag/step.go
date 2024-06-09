package dag

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/util"
)

// Step represents a step in a DAG.
type Step struct {
	Name            string         `json:"Name"`
	Description     string         `json:"Description,omitempty"`
	Variables       []string       `json:"Variables,omitempty"`
	OutputVariables *util.SyncMap  `json:"OutputVariables,omitempty"`
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

type SubWorkflow struct {
	Name   string
	Params string
}

// ExecutorConfig represents the configuration for the executor of a step.
type ExecutorConfig struct {
	Type   string
	Config map[string]interface{}
}

const (
	ExecutorTypeSubWorkflow = "subworkflow"
)

// RetryPolicy represents the retry policy for a step.
type RetryPolicy struct {
	Limit    int
	Interval time.Duration
}

// RepeatPolicy represents the repeat policy for a step.
type RepeatPolicy struct {
	Repeat   bool
	Interval time.Duration
}

// ContinueOn represents the conditions under which the step continues.
type ContinueOn struct {
	Failure bool
	Skipped bool
}

// String returns a string representation of the step's properties.
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
