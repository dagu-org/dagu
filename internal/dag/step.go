package dag

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Step represents a step in a DAG.
type Step struct {
	Name            string
	Description     string
	Variables       []string
	OutputVariables *sync.Map
	Dir             string
	Executor        string
	ExecutorConfig  map[string]interface{}
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

type RetryPolicy struct {
	Limit    int
	Interval time.Duration
}

type RepeatPolicy struct {
	Repeat   bool
	Interval time.Duration
}

type ContinueOn struct {
	Failure bool
	Skipped bool
}

func (s *Step) String() string {
	vals := []string{}
	vals = append(vals, fmt.Sprintf("Name: %s", s.Name))
	vals = append(vals, fmt.Sprintf("Dir: %s", s.Dir))
	vals = append(vals, fmt.Sprintf("Command: %s", s.Command))
	vals = append(vals, fmt.Sprintf("Args: %s", s.Args))
	vals = append(vals, fmt.Sprintf("Depends: [%s]", strings.Join(s.Depends, ", ")))
	return strings.Join(vals, "\t")
}
