// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

func StatusFromJSON(s string) (*Status, error) {
	status := new(Status)
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, err
}

func FromNodesOrSteps(nodes []scheduler.NodeData, steps []digraph.Step) []*Node {
	if len(nodes) != 0 {
		return FromNodes(nodes)
	}
	return FromSteps(steps)
}

type StatusFile struct {
	File   string
	Status *Status
}

type StatusResponse struct {
	Status *Status `json:"status"`
}

type Status struct {
	RequestID  string           `json:"RequestId"`
	Name       string           `json:"Name"`
	Status     scheduler.Status `json:"Status"`
	StatusText string           `json:"StatusText"`
	PID        PID              `json:"Pid"`
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

func NewStatusDefault(dag *digraph.DAG) *Status {
	return NewStatus(
		dag, nil, scheduler.StatusNone, int(pidNotRunning), nil, nil,
	)
}

func NewStatus(
	dag *digraph.DAG,
	nodes []scheduler.NodeData,
	status scheduler.Status,
	pid int,
	startTime, endTime *time.Time,
) *Status {
	statusObj := &Status{
		Name:       dag.Name,
		Status:     status,
		StatusText: status.String(),
		PID:        PID(pid),
		Nodes:      FromNodesOrSteps(nodes, dag.Steps),
		OnExit:     nodeOrNil(dag.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(dag.HandlerOn.Success),
		OnFailure:  nodeOrNil(dag.HandlerOn.Failure),
		OnCancel:   nodeOrNil(dag.HandlerOn.Cancel),
		Params:     Params(dag.Params),
	}
	if startTime != nil {
		statusObj.StartedAt = stringutil.FormatTime(*startTime)
	}
	if endTime != nil {
		statusObj.FinishedAt = stringutil.FormatTime(*endTime)
	}
	return statusObj
}

func (st *Status) CorrectRunningStatus() {
	if st.Status == scheduler.StatusRunning {
		st.Status = scheduler.StatusError
		st.StatusText = st.Status.String()
	}
}

func (st *Status) ToJSON() ([]byte, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	js, err := json.Marshal(st)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}

func FormatTime(val time.Time) string {
	if val.IsZero() {
		return ""
	}
	return stringutil.FormatTime(val)
}

func Time(t time.Time) *time.Time {
	return &t
}

func Params(params []string) string {
	return strings.Join(params, " ")
}

type PID int

const pidNotRunning PID = -1

func (p PID) String() string {
	if p == pidNotRunning {
		return ""
	}
	return fmt.Sprintf("%d", p)
}

func (p PID) IsRunning() bool {
	return p != pidNotRunning
}

func nodeOrNil(s *digraph.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNode(*s)
}
