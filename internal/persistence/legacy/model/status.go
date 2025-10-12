package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/digraph/status"
)

func StatusFromJSON(s string) (*Status, error) {
	status := new(Status)
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, err
}

type StatusFile struct {
	File   string
	Status Status
}

type StatusResponse struct {
	Status *Status `json:"status"`
}

type Status struct {
	RequestID  string        `json:"RequestId"`
	Name       string        `json:"Name"`
	Status     status.Status `json:"Status"`
	StatusText string        `json:"StatusText"`
	PID        PID           `json:"Pid"`
	Nodes      []*Node       `json:"Nodes"`
	OnExit     *Node         `json:"OnExit"`
	OnSuccess  *Node         `json:"OnSuccess"`
	OnFailure  *Node         `json:"OnFailure"`
	OnCancel   *Node         `json:"OnCancel"`
	StartedAt  string        `json:"StartedAt"`
	FinishedAt string        `json:"FinishedAt"`
	Log        string        `json:"Log"`
	Params     string        `json:"Params,omitempty"`
	ParamsList []string      `json:"ParamsList,omitempty"`
}

func (st *Status) CorrectRunningStatus() {
	if st.Status == status.Running {
		st.Status = status.Error
		st.StatusText = st.Status.String()
	}
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
