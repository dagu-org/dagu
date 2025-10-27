package transform

import (
	"errors"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
)

// ToNode converts a persistence Node back to a scheduler Node
func ToNode(n *execution.Node) *runtime.Node {
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
	})
}

// newNode converts a single scheduler NodeData to a persistence Node
func newNode(node runtime.NodeData) *execution.Node {
	children := make([]execution.SubDAGRun, len(node.State.SubRuns))
	for i, child := range node.State.SubRuns {
		children[i] = execution.SubDAGRun(child)
	}
	var errText string
	if node.State.Error != nil {
		errText = node.State.Error.Error()
	}
	childrenRepeated := make([]execution.SubDAGRun, len(node.State.SubRunsRepeated))
	for i, child := range node.State.SubRunsRepeated {
		childrenRepeated[i] = execution.SubDAGRun(child)
	}
	return &execution.Node{
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
	}
}
