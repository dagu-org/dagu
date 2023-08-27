package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

type StatusResponse struct {
	Status *Status `jsondb:"status"`
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
	RequestId  string                    `jsondb:"RequestId"`
	Name       string                    `jsondb:"Name"`
	Status     scheduler.SchedulerStatus `jsondb:"Status"`
	StatusText string                    `jsondb:"StatusText"`
	Pid        Pid                       `jsondb:"Pid"`
	Nodes      []*Node                   `jsondb:"Nodes"`
	OnExit     *Node                     `jsondb:"OnExit"`
	OnSuccess  *Node                     `jsondb:"OnSuccess"`
	OnFailure  *Node                     `jsondb:"OnFailure"`
	OnCancel   *Node                     `jsondb:"OnCancel"`
	StartedAt  string                    `jsondb:"StartedAt"`
	FinishedAt string                    `jsondb:"FinishedAt"`
	Log        string                    `jsondb:"Log"`
	Params     string                    `jsondb:"Params"`
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

func NewStatus(d *dag.DAG, nodes []*scheduler.Node, status scheduler.SchedulerStatus,
	pid int, s, e *time.Time) *Status {
	finish, start := time.Time{}, time.Time{}
	if s != nil {
		start = *s
	}
	if e != nil {
		finish = *e
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
