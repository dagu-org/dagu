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

const pidNotRunning PID = -1

type (
	History struct {
		File   string
		Status *Status
	}

	StatusResponse struct {
		Status *Status `json:"status"`
	}

	Status struct {
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

	PID int
)

func StatusFromJSON(s string) (*Status, error) {
	status := new(Status)
	err := json.Unmarshal([]byte(s), status)
	return status, err
}

type NewStatusArgs struct {
	RequestID             string
	DAG                   *dag.DAG
	Nodes                 []scheduler.NodeData
	Status                scheduler.Status
	PID                   int
	StartedAt, FinishedAt time.Time
	Log                   string
}

func NewStatus(args NewStatusArgs) *Status {
	if args.PID == 0 {
		args.PID = int(pidNotRunning)
	}
	if args.DAG == nil {
		args.DAG = &dag.DAG{}
	}

	return &Status{
		RequestID:  args.RequestID,
		Name:       args.DAG.Name,
		Status:     args.Status,
		StatusText: args.Status.String(),
		PID:        PID(args.PID),
		Nodes:      model.FromNodesOrSteps(args.Nodes, args.DAG.Steps),
		OnExit:     nodeOrNil(args.DAG.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(args.DAG.HandlerOn.Success),
		OnFailure:  nodeOrNil(args.DAG.HandlerOn.Failure),
		OnCancel:   nodeOrNil(args.DAG.HandlerOn.Cancel),
		Params:     formatParams(args.DAG.Params),
		Log:        args.Log,
		StartedAt:  util.FormatTime(args.StartedAt),
		FinishedAt: util.FormatTime(args.FinishedAt),
	}
}

func (s *Status) CorrectRunningStatus() {
	if s.Status == scheduler.StatusRunning {
		s.Status = scheduler.StatusError
		s.StatusText = s.Status.String()
	}
}

func (s *Status) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

func (p PID) String() string {
	if p == pidNotRunning {
		return ""
	}
	return fmt.Sprintf("%d", p)
}

func (p PID) IsRunning() bool {
	return p != pidNotRunning
}

func formatParams(params []string) string {
	return strings.Join(params, " ")
}

func nodeOrNil(s *dag.Step) *model.Node {
	if s == nil {
		return nil
	}
	return model.NewNode(*s)
}
