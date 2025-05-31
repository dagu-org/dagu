package model

import (
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

func FromSteps(steps []digraph.Step) []*Node {
	var ret []*Node
	for _, s := range steps {
		ret = append(ret, NewNode(s))
	}
	return ret
}

func FromNodes(nodes []scheduler.NodeData) []*Node {
	var ret []*Node
	for _, node := range nodes {
		ret = append(ret, FromNode(node))
	}
	return ret
}

func FromNode(node scheduler.NodeData) *Node {
	return &Node{
		Step:       node.Step,
		Log:        node.State.Log,
		StartedAt:  stringutil.FormatTime(node.State.StartedAt),
		FinishedAt: stringutil.FormatTime(node.State.FinishedAt),
		Status:     node.State.Status,
		StatusText: node.State.Status.String(),
		RetriedAt:  stringutil.FormatTime(node.State.RetriedAt),
		RetryCount: node.State.RetryCount,
		DoneCount:  node.State.DoneCount,
		Error:      errText(node.State.Error),
	}
}

type Node struct {
	Step       digraph.Step         `json:"Step"`
	Log        string               `json:"Log"`
	StartedAt  string               `json:"StartedAt"`
	FinishedAt string               `json:"FinishedAt"`
	Status     scheduler.NodeStatus `json:"Status"`
	RetriedAt  string               `json:"RetriedAt,omitempty"`
	RetryCount int                  `json:"RetryCount,omitempty"`
	DoneCount  int                  `json:"DoneCount,omitempty"`
	Error      string               `json:"Error,omitempty"`
	StatusText string               `json:"StatusText"`
}

func (n *Node) ToNode() *scheduler.Node {
	startedAt, _ := stringutil.ParseTime(n.StartedAt)
	finishedAt, _ := stringutil.ParseTime(n.FinishedAt)
	retriedAt, _ := stringutil.ParseTime(n.RetriedAt)
	return scheduler.NewNode(n.Step, scheduler.NodeState{
		Status:     n.Status,
		Log:        n.Log,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		RetriedAt:  retriedAt,
		RetryCount: n.RetryCount,
		DoneCount:  n.DoneCount,
		Error:      errFromText(n.Error),
	})
}

func NewNode(step digraph.Step) *Node {
	return &Node{
		Step:       step,
		StartedAt:  "-",
		FinishedAt: "-",
		Status:     scheduler.NodeStatusNone,
		StatusText: scheduler.NodeStatusNone.String(),
	}
}

var errNodeProcessing = errors.New("node processing error")

func errFromText(err string) error {
	if err == "" {
		return nil
	}
	return fmt.Errorf("%w: %s", errNodeProcessing, err)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
