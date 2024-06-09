package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/util"
)

type StatusResponse struct {
	Status *Status `json:"status"`
}

type Pid int

const PidNotRunning Pid = -1

func (p Pid) String() string {
	if p == PidNotRunning {
		return ""
	}
	return fmt.Sprintf("%d", p)
}

func (p Pid) IsRunning() bool {
	return p != PidNotRunning
}

type Status struct {
	RequestId  string           `json:"RequestId"`
	Name       string           `json:"Name"`
	Status     scheduler.Status `json:"Status"`
	StatusText string           `json:"StatusText"`
	Pid        Pid              `json:"Pid"`
	Nodes      []*Node          `json:"Nodes"`
	OnExit     *Node            `json:"OnExit"`
	OnSuccess  *Node            `json:"OnSuccess"`
	OnFailure  *Node            `json:"OnFailure"`
	OnCancel   *Node            `json:"OnCancel"`
	StartedAt  string           `json:"StartedAt"`
	FinishedAt string           `json:"FinishedAt"`
	Log        string           `json:"Log"`
	Params     string           `json:"Params"`
	mu         sync.RWMutex
}

type StatusFile struct {
	File   string
	Status *Status
}

func StatusFromJson(s string) (*Status, error) {
	status := &Status{}
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, err
}

func NewStatusDefault(d *dag.DAG) *Status {
	return NewStatus(d, nil, scheduler.StatusNone, int(PidNotRunning), nil, nil)
}

func Time(t time.Time) *time.Time {
	return &t
}

type NodeStepPair struct {
	Node scheduler.NodeState
	Step dag.Step
}

func NewStatus(
	d *dag.DAG,
	nodes []NodeStepPair,
	status scheduler.Status,
	pid int,
	startTime, endTime *time.Time,
) *Status {
	var onExit, onSuccess, onFailure, onCancel *Node
	onExit = nodeOrNil(d.HandlerOn.Exit)
	onSuccess = nodeOrNil(d.HandlerOn.Success)
	onFailure = nodeOrNil(d.HandlerOn.Failure)
	onCancel = nodeOrNil(d.HandlerOn.Cancel)
	return &Status{
		Name:       d.Name,
		Status:     status,
		StatusText: status.String(),
		Pid:        Pid(pid),
		Nodes:      nodesOrSteps(nodes, d.Steps),
		OnExit:     onExit,
		OnSuccess:  onSuccess,
		OnFailure:  onFailure,
		OnCancel:   onCancel,
		StartedAt:  formatTime(startTime),
		FinishedAt: formatTime(endTime),
		Params:     strings.Join(d.Params, " "),
	}
}

func nodeOrNil(s *dag.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNode(*s)
}

func nodesOrSteps(nodes []NodeStepPair, steps []dag.Step) []*Node {
	if len(nodes) != 0 {
		return FromNodes(nodes)
	}
	return FromSteps(steps)
}

func formatTime(val *time.Time) string {
	if val == nil || val.IsZero() {
		return ""
	}
	return util.FormatTime(*val)
}

func (st *Status) CorrectRunningStatus() {
	if st.Status == scheduler.StatusRunning {
		st.Status = scheduler.StatusError
		st.StatusText = st.Status.String()
	}
}

func (st *Status) ToJson() ([]byte, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	js, err := json.Marshal(st)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
