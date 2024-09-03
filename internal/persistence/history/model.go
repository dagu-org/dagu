// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package history

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

func StatusFromJSON(s string) (*Status, error) {
	status := new(Status)
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, err
}

type History struct {
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
	Nodes      []*model.Node    `json:"Nodes"`
	OnExit     *model.Node      `json:"OnExit"`
	OnSuccess  *model.Node      `json:"OnSuccess"`
	OnFailure  *model.Node      `json:"OnFailure"`
	OnCancel   *model.Node      `json:"OnCancel"`
	StartedAt  string           `json:"StartedAt"`
	FinishedAt string           `json:"FinishedAt"`
	Log        string           `json:"Log"`
	Params     string           `json:"Params"`
	mu         sync.RWMutex
}

func NewStatusDefault(dAG *dag.DAG) *Status {
	return NewStatus(
		dAG, nil, scheduler.StatusNone, int(pidNotRunning), nil, nil,
	)
}

func NewStatus(
	dag *dag.DAG,
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
		Nodes:      model.FromNodesOrSteps(nodes, dag.Steps),
		OnExit:     nodeOrNil(dag.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(dag.HandlerOn.Success),
		OnFailure:  nodeOrNil(dag.HandlerOn.Failure),
		OnCancel:   nodeOrNil(dag.HandlerOn.Cancel),
		Params:     Params(dag.Params),
	}
	if startTime != nil {
		statusObj.StartedAt = util.FormatTime(*startTime)
	}
	if endTime != nil {
		statusObj.FinishedAt = util.FormatTime(*endTime)
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
	return util.FormatTime(val)
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

func nodeOrNil(s *dag.Step) *model.Node {
	if s == nil {
		return nil
	}
	return model.NewNode(*s)
}
