package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
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

func NewStatus(cfg *config.Config, nodes []*scheduler.Node, status scheduler.SchedulerStatus,
	pid int, s, e *time.Time) *Status {
	finish, start := time.Time{}, time.Time{}
	if s != nil {
		start = *s
	}
	if e != nil {
		finish = *e
	}
	models := []*Node{}
	if nodes != nil && len(nodes) != 0 {
		models = FromNodes(nodes)
	} else {
		models = FromSteps(cfg.Steps)
	}
	var onExit, onSuccess, onFailure, onCancel *Node = nil, nil, nil, nil
	if cfg.HandlerOn.Exit != nil {
		onExit = fromStepWithDefValues(cfg.HandlerOn.Exit)
	}
	if cfg.HandlerOn.Success != nil {
		onSuccess = fromStepWithDefValues(cfg.HandlerOn.Success)
	}
	if cfg.HandlerOn.Failure != nil {
		onFailure = fromStepWithDefValues(cfg.HandlerOn.Failure)
	}
	if cfg.HandlerOn.Cancel != nil {
		onCancel = fromStepWithDefValues(cfg.HandlerOn.Cancel)
	}
	return &Status{
		RequestId:  "",
		Name:       cfg.Name,
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
		Params:     strings.Join(cfg.Params, " "),
	}
}

func (sts *Status) ToJson() ([]byte, error) {
	js, err := json.Marshal(sts)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
