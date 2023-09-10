package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
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
	RequestId  string                    `json:"RequestId"`
	Name       string                    `json:"Name"`
	Status     scheduler.SchedulerStatus `json:"Status"`
	StatusText string                    `json:"StatusText"`
	Pid        Pid                       `json:"Pid"`
	Nodes      []*Node                   `json:"Nodes"`
	OnExit     *Node                     `json:"OnExit"`
	OnSuccess  *Node                     `json:"OnSuccess"`
	OnFailure  *Node                     `json:"OnFailure"`
	OnCancel   *Node                     `json:"OnCancel"`
	StartedAt  string                    `json:"StartedAt"`
	FinishedAt string                    `json:"FinishedAt"`
	Log        string                    `json:"Log"`
	Params     string                    `json:"Params"`
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
	return NewStatus(d, nil, scheduler.SchedulerStatus_None, int(PidNotRunning), nil, nil)
}

func NewStatus(
	d *dag.DAG,
	nodes []*scheduler.Node,
	status scheduler.SchedulerStatus,
	pid int,
	startTime, endTIme *time.Time,
) *Status {
	finish, start := time.Time{}, time.Time{}
	if startTime != nil {
		start = *startTime
	}
	if endTIme != nil {
		finish = *endTIme
	}
	var models []*Node
	if len(nodes) != 0 {
		models = FromNodes(nodes)
	} else {
		models = FromSteps(d.Steps)
	}
	var onExit, onSuccess, onFailure, onCancel *Node
	onExit = fromStepWithDefValues(d.HandlerOn.Exit)
	onSuccess = fromStepWithDefValues(d.HandlerOn.Success)
	onFailure = fromStepWithDefValues(d.HandlerOn.Failure)
	onCancel = fromStepWithDefValues(d.HandlerOn.Cancel)
	return &Status{
		RequestId:  "",
		Name:       d.Name,
		Status:     status,
		StatusText: status.String(),
		Pid:        Pid(pid),
		Nodes:      models,
		OnExit:     onExit,
		OnSuccess:  onSuccess,
		OnFailure:  onFailure,
		OnCancel:   onCancel,
		StartedAt:  utils.FormatTime(start),
		FinishedAt: utils.FormatTime(finish),
		Params:     strings.Join(d.Params, " "),
	}
}

func (sts *Status) CorrectRunningStatus() {
	if sts.Status == scheduler.SchedulerStatus_Running {
		sts.Status = scheduler.SchedulerStatus_Error
		sts.StatusText = sts.Status.String()
	}
}

func (sts *Status) ToJson() ([]byte, error) {
	js, err := json.Marshal(sts)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
