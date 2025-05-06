package runstore

import (
	"errors"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// FromSteps converts a list of DAG steps to persistence Node objects
func FromSteps(steps []digraph.Step) []*Node {
	var ret []*Node
	for _, s := range steps {
		ret = append(ret, NewNode(s))
	}
	return ret
}

// FromNodes converts scheduler NodeData objects to persistence Node objects
func FromNodes(nodes []scheduler.NodeData) []*Node {
	var ret []*Node
	for _, node := range nodes {
		ret = append(ret, FromNode(node))
	}
	return ret
}

// FromNode converts a single scheduler NodeData to a persistence Node
func FromNode(node scheduler.NodeData) *Node {
	children := make([]ChildRun, len(node.State.ChildRuns))
	for i, childRun := range node.State.ChildRuns {
		children[i] = ChildRun(childRun)
	}
	var errText string
	if node.State.Error != nil {
		errText = node.State.Error.Error()
	}
	return &Node{
		Step:       node.Step,
		Log:        node.State.Log,
		StartedAt:  stringutil.FormatTime(node.State.StartedAt),
		FinishedAt: stringutil.FormatTime(node.State.FinishedAt),
		Status:     node.State.Status,
		RetriedAt:  stringutil.FormatTime(node.State.RetriedAt),
		RetryCount: node.State.RetryCount,
		DoneCount:  node.State.DoneCount,
		Error:      errText,
		Children:   children,
	}
}

// Node represents a DAG step with its execution state for persistence
type Node struct {
	Step       digraph.Step         `json:"step"`
	Log        string               `json:"log"`
	StartedAt  string               `json:"startedAt"`
	FinishedAt string               `json:"finishedAt"`
	Status     scheduler.NodeStatus `json:"status"`
	RetriedAt  string               `json:"retriedAt,omitempty"`
	RetryCount int                  `json:"retryCount,omitempty"`
	DoneCount  int                  `json:"doneCount,omitempty"`
	Error      string               `json:"error,omitempty"`
	Children   []ChildRun           `json:"children,omitempty"`
}

type ChildRun struct {
	RequestID string `json:"requestId,omitempty"`
}

// ToNode converts a persistence Node back to a scheduler Node
func (n *Node) ToNode() *scheduler.Node {
	startedAt, _ := stringutil.ParseTime(n.StartedAt)
	finishedAt, _ := stringutil.ParseTime(n.FinishedAt)
	retriedAt, _ := stringutil.ParseTime(n.RetriedAt)
	children := make([]scheduler.ChildRun, len(n.Children))
	for i, child := range n.Children {
		children[i] = scheduler.ChildRun(child)
	}
	return scheduler.NewNode(n.Step, scheduler.NodeState{
		Status:     n.Status,
		Log:        n.Log,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		RetriedAt:  retriedAt,
		RetryCount: n.RetryCount,
		DoneCount:  n.DoneCount,
		Error:      errors.New(n.Error),
		ChildRuns:  children,
	})
}

// NewNode creates a new Node with default status values for the given step
func NewNode(step digraph.Step) *Node {
	return &Node{
		Step:       step,
		StartedAt:  "-",
		FinishedAt: "-",
		Status:     scheduler.NodeStatusNone,
	}
}
