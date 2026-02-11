package exec

import (
	"github.com/dagu-org/dagu/internal/cmn/collections"
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
	// ApprovedAt records when this wait step was approved (HITL)
	ApprovedAt string `json:"approvedAt,omitempty"`
	// ApprovalInputs stores key-value parameters provided during approval
	ApprovalInputs map[string]string `json:"approvalInputs,omitempty"`
	// ApprovedBy records who approved this wait step (username)
	ApprovedBy string `json:"approvedBy,omitempty"`
	// RejectedAt records when this wait step was rejected (HITL)
	RejectedAt string `json:"rejectedAt,omitempty"`
	// RejectedBy records who rejected this wait step (username)
	RejectedBy string `json:"rejectedBy,omitempty"`
	// RejectionReason stores the optional reason for rejection
	RejectionReason string `json:"rejectionReason,omitempty"`
	// ChatMessages stores the session messages for chat/LLM steps.
	// This field is populated during execution and synced via status updates
	// in shared-nothing mode where workers don't have filesystem access.
	ChatMessages []LLMMessage `json:"chatMessages,omitempty"`
	// ToolDefinitions stores the tool definitions that were available to the LLM.
	// This enables debugging visibility into what tools and schemas were sent.
	ToolDefinitions []ToolDefinition `json:"toolDefinitions,omitempty"`
}

// SubDAGRun represents a sub DAG run associated with a node
type SubDAGRun struct {
	DAGRunID string `json:"dagRunId,omitempty"`
	Params   string `json:"params,omitempty"`
	// DAGName is the name of the executed sub-DAG.
	// For chat tool calls, this is the tool DAG name.
	// This field enables UI drill-down when step.call is not set.
	DAGName string `json:"dagName,omitempty"`
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
