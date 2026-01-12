package transform

import (
	"errors"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
)

// ToNode converts a persistence Node back to a runtime Node
func ToNode(n *exec.Node) *runtime.Node {
	startedAt, _ := stringutil.ParseTime(n.StartedAt)
	finishedAt, _ := stringutil.ParseTime(n.FinishedAt)
	retriedAt, _ := stringutil.ParseTime(n.RetriedAt)
	children := make([]runtime.SubDAGRun, len(n.SubRuns))
	for i, r := range n.SubRuns {
		children[i] = runtime.SubDAGRun(r)
	}
	childrenRepeated := make([]runtime.SubDAGRun, len(n.SubRunsRepeated))
	for i, r := range n.SubRunsRepeated {
		childrenRepeated[i] = runtime.SubDAGRun(r)
	}
	var err error
	if n.Error != "" {
		err = errors.New(n.Error)
	}
	return runtime.NewNode(n.Step, runtime.NodeState{
		Status:          n.Status,
		Stdout:          n.Stdout,
		Stderr:          n.Stderr,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		RetriedAt:       retriedAt,
		RetryCount:      n.RetryCount,
		DoneCount:       n.DoneCount,
		Repeated:        n.Repeated,
		Error:           err,
		SubRuns:         children,
		SubRunsRepeated: childrenRepeated,
		OutputVariables: n.OutputVariables,
		ChatMessages:    n.ChatMessages,
		ApprovalInputs:  n.ApprovalInputs,
		ApprovedAt:      n.ApprovedAt,
		ApprovedBy:      n.ApprovedBy,
		RejectedAt:      n.RejectedAt,
		RejectedBy:      n.RejectedBy,
		RejectionReason: n.RejectionReason,
	})
}

// newNode converts a single runtime NodeData to a persistence Node
func newNode(node runtime.NodeData) *exec.Node {
	children := make([]exec.SubDAGRun, len(node.State.SubRuns))
	for i, child := range node.State.SubRuns {
		children[i] = exec.SubDAGRun(child)
	}
	var errText string
	if node.State.Error != nil {
		errText = node.State.Error.Error()
	}
	childrenRepeated := make([]exec.SubDAGRun, len(node.State.SubRunsRepeated))
	for i, child := range node.State.SubRunsRepeated {
		childrenRepeated[i] = exec.SubDAGRun(child)
	}
	return &exec.Node{
		Step:            node.Step,
		Stdout:          node.State.Stdout,
		Stderr:          node.State.Stderr,
		StartedAt:       stringutil.FormatTime(node.State.StartedAt),
		FinishedAt:      stringutil.FormatTime(node.State.FinishedAt),
		Status:          node.State.Status,
		RetriedAt:       stringutil.FormatTime(node.State.RetriedAt),
		RetryCount:      node.State.RetryCount,
		DoneCount:       node.State.DoneCount,
		Repeated:        node.State.Repeated,
		Error:           errText,
		SubRuns:         children,
		SubRunsRepeated: childrenRepeated,
		OutputVariables: node.State.OutputVariables,
		ChatMessages:    node.State.ChatMessages,
		ApprovalInputs:  node.State.ApprovalInputs,
		ApprovedAt:      node.State.ApprovedAt,
		ApprovedBy:      node.State.ApprovedBy,
		RejectedAt:      node.State.RejectedAt,
		RejectedBy:      node.State.RejectedBy,
		RejectionReason: node.State.RejectionReason,
	}
}
