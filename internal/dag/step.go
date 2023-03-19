package dag

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/utils"
)

// Step represents a step in a DAG.
type Step struct {
	Name            string
	Description     string
	Variables       []string
	OutputVariables *utils.SyncMap
	Dir             string
	ExecutorConfig  ExecutorConfig
	CmdWithArgs     string
	Command         string
	Script          string
	Stdout          string
	Stderr          string
	Output          string
	Args            []string
	Depends         []string
	ContinueOn      ContinueOn
	RetryPolicy     *RetryPolicy
	RepeatPolicy    RepeatPolicy
	MailOnError     bool
	Preconditions   []*Condition
	SignalOnStop    string
}

// ExecutorConfig represents the configuration for the executor of a step.
type ExecutorConfig struct {
	Type   string
	Config map[string]interface{}
}

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
