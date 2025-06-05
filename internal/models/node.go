package models

import (
	"errors"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// Node represents a DAG step with its execution state for persistence
type Node struct {
	Step            digraph.Step         `json:"step"`
	Stdout          string               `json:"stdout"` // standard output log file path
	Stderr          string               `json:"stderr"` // standard error log file path
	StartedAt       string               `json:"startedAt"`
	FinishedAt      string               `json:"finishedAt"`
	Status          scheduler.NodeStatus `json:"status"`
	RetriedAt       string               `json:"retriedAt,omitempty"`
	RetryCount      int                  `json:"retryCount,omitempty"`
	DoneCount       int                  `json:"doneCount,omitempty"`
	Error           string               `json:"error,omitempty"`
	Children        []ChildDAGRun        `json:"children,omitempty"`
	OutputVariables *executor.SyncMap    `json:"outputVariables,omitempty"`
}

// ChildDAGRun represents a child DAG run associated with a node
type ChildDAGRun struct {
	DAGRunID string `json:"dagRunId,omitempty"`
}

// ToNode converts a persistence Node back to a scheduler Node
func (n *Node) ToNode() *scheduler.Node {
	startedAt, _ := stringutil.ParseTime(n.StartedAt)
	finishedAt, _ := stringutil.ParseTime(n.FinishedAt)
	retriedAt, _ := stringutil.ParseTime(n.RetriedAt)
	children := make([]scheduler.ChildDAGRun, len(n.Children))
	for i, r := range n.Children {
		children[i] = scheduler.ChildDAGRun(r)
	}
	return scheduler.NewNode(n.Step, scheduler.NodeState{
		Status:          n.Status,
		Stdout:          n.Stdout,
		Stderr:          n.Stderr,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		RetriedAt:       retriedAt,
		RetryCount:      n.RetryCount,
		DoneCount:       n.DoneCount,
		Error:           errors.New(n.Error),
		Children:        children,
		OutputVariables: n.OutputVariables,
	})
}

// NodesFromSteps converts a list of DAG steps to persistence Node objects
func NodesFromSteps(steps []digraph.Step) []*Node {
	var ret []*Node
	for _, s := range steps {
		ret = append(ret, newNodeFromStep(s))
	}
	return ret
}

// newNodeFromStep creates a new Node with default status values for the given step
func newNodeFromStep(step digraph.Step) *Node {
	return &Node{
		Step:       step,
		StartedAt:  "-",
		FinishedAt: "-",
		Status:     scheduler.NodeStatusNone,
	}
}

// newNode converts a single scheduler NodeData to a persistence Node
func newNode(node scheduler.NodeData) *Node {
	children := make([]ChildDAGRun, len(node.State.Children))
	for i, child := range node.State.Children {
		children[i] = ChildDAGRun(child)
	}
	var errText string
	if node.State.Error != nil {
		errText = node.State.Error.Error()
	}
	return &Node{
		Step:            node.Step,
		Stdout:          node.State.Stdout,
		Stderr:          node.State.Stderr,
		StartedAt:       stringutil.FormatTime(node.State.StartedAt),
		FinishedAt:      stringutil.FormatTime(node.State.FinishedAt),
		Status:          node.State.Status,
		RetriedAt:       stringutil.FormatTime(node.State.RetriedAt),
		RetryCount:      node.State.RetryCount,
		DoneCount:       node.State.DoneCount,
		Error:           errText,
		Children:        children,
		OutputVariables: node.State.OutputVariables,
	}
}
