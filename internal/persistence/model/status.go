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

type StatusFactory struct {
	dag *digraph.DAG
}

func NewStatusFactory(dag *digraph.DAG) *StatusFactory {
	return &StatusFactory{dag: dag}
}

func (f *StatusFactory) CreateDefault() *Status {
	return &Status{
		Name:       f.dag.Name,
		Status:     scheduler.StatusNone,
		StatusText: scheduler.StatusNone.String(),
		PID:        PID(pidNotRunning),
		Nodes:      FromSteps(f.dag.Steps),
		OnExit:     nodeOrNil(f.dag.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(f.dag.HandlerOn.Success),
		OnFailure:  nodeOrNil(f.dag.HandlerOn.Failure),
		OnCancel:   nodeOrNil(f.dag.HandlerOn.Cancel),
		Params:     Params(f.dag.Params),
		StartedAt:  stringutil.FormatTime(time.Time{}),
		FinishedAt: stringutil.FormatTime(time.Time{}),
	}
}

type StatusOption func(*Status)

func WithRequestID(reqID string) StatusOption {
	return func(s *Status) {
		s.RequestID = reqID
	}
}

func WithNodes(nodes []scheduler.NodeData) StatusOption {
	return func(s *Status) {
		s.Nodes = FromNodes(nodes)
	}
}

func WithFinishedAt(t time.Time) StatusOption {
	return func(s *Status) {
		s.FinishedAt = FormatTime(t)
	}
}

func (f *StatusFactory) Create(
	status scheduler.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) *Status {
	statusObj := f.CreateDefault()
	statusObj.Status = status
	statusObj.StatusText = status.String()
	statusObj.PID = PID(pid)
	statusObj.StartedAt = FormatTime(startedAt)

	for _, opt := range opts {
		opt(statusObj)
	}

	return statusObj
}

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
