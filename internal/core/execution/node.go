package execution

import (
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
)

// Node represents a DAG step with its execution state for persistence
type Node struct {
	Step            core.Step            `json:"step,omitzero"`
	Stdout          string               `json:"stdout"` // standard output log file path
	Stderr          string               `json:"stderr"` // standard error log file path
	StartedAt       string               `json:"startedAt"`
	FinishedAt      string               `json:"finishedAt"`
	Status          core.NodeStatus      `json:"status"`
	RetriedAt       string               `json:"retriedAt,omitempty"`
	RetryCount      int                  `json:"retryCount,omitempty"`
	DoneCount       int                  `json:"doneCount,omitempty"`
	Repeated        bool                 `json:"repeated,omitempty"` // indicates if the node has been repeated
	Error           string               `json:"error,omitempty"`
	SubRuns         []SubDAGRun          `json:"children,omitempty"`
	SubRunsRepeated []SubDAGRun          `json:"childrenRepeated,omitempty"` // repeated sub DAG runs
	OutputVariables *collections.SyncMap `json:"outputVariables,omitempty"`
}

// SubDAGRun represents a sub DAG run associated with a node
type SubDAGRun struct {
	DAGRunID string `json:"dagRunId,omitempty"`
	Params   string `json:"params,omitempty"`
}

// NewNodesFromSteps converts a list of DAG steps to persistence Node objects.
func NewNodesFromSteps(steps []core.Step) []*Node {
	var ret []*Node
	for _, s := range steps {
		ret = append(ret, NewNodeFromStep(s))
	}
	return ret
}

// NewNodeFromStep creates a new Node with default status values for the given step.
func NewNodeFromStep(step core.Step) *Node {
	return &Node{
		Step:       step,
		StartedAt:  "-",
		FinishedAt: "-",
		Status:     core.NodeNotStarted,
	}
}

// NewNodeOrNil creates a Node from a Step or returns nil if the step is nil.
func NewNodeOrNil(s *core.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNodeFromStep(*s)
}
