package model

import (
	"fmt"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
)

type Node struct {
	*dag.Step  `json:"Step"`
	Log        string               `json:"Log"`
	StartedAt  string               `json:"StartedAt"`
	FinishedAt string               `json:"FinishedAt"`
	Status     scheduler.NodeStatus `json:"Status"`
	RetryCount int                  `json:"RetryCount"`
	DoneCount  int                  `json:"DoneCount"`
	Error      string               `json:"Error"`
	StatusText string               `json:"StatusText"`
}

func (n *Node) ToNode() *scheduler.Node {
	startedAt, _ := utils.ParseTime(n.StartedAt)
	finishedAt, _ := utils.ParseTime(n.FinishedAt)
	var err error = nil
	if n.Error != "" {
		err = fmt.Errorf(n.Error)
	}
	ret := &scheduler.Node{
		Step: n.Step,
		NodeState: scheduler.NodeState{
			Status:     n.Status,
			Log:        n.Log,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			RetryCount: n.RetryCount,
			DoneCount:  n.DoneCount,
			Error:      err,
		},
	}
	return ret
}

func FromNode(n *scheduler.Node) *Node {
	node := &Node{
		Step:       n.Step,
		Log:        n.Log,
		StartedAt:  utils.FormatTime(n.StartedAt),
		FinishedAt: utils.FormatTime(n.FinishedAt),
		Status:     n.ReadStatus(),
		StatusText: n.ReadStatus().String(),
		RetryCount: n.ReadRetryCount(),
		DoneCount:  n.ReadDoneCount(),
	}
	if n.Error != nil {
		node.Error = n.Error.Error()
	}
	return node
}

func FromNodes(nodes []*scheduler.Node) []*Node {
	ret := []*Node{}
	for _, n := range nodes {
		ret = append(ret, FromNode(n))
	}
	return ret
}

func FromSteps(steps []*dag.Step) []*Node {
	ret := []*Node{}
	for _, s := range steps {
		ret = append(ret, fromStepWithDefValues(s))
	}
	return ret
}

func fromStepWithDefValues(s *dag.Step) *Node {
	if s == nil {
		return nil
	}
	step := &Node{
		Step:       s,
		Log:        "",
		StartedAt:  "-",
		FinishedAt: "-",
		Status:     scheduler.NodeStatus_None,
		StatusText: scheduler.NodeStatus_None.String(),
		RetryCount: 0,
	}
	return step
}
