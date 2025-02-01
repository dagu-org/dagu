package model

import (
	"encoding/json"
	"fmt"
	"strings"
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

func (f *StatusFactory) CreateDefault() Status {
	return Status{
		Name:       f.dag.Name,
		Status:     scheduler.StatusNone,
		StatusText: scheduler.StatusNone.String(),
		PID:        PID(pidNotRunning),
		Nodes:      FromSteps(f.dag.Steps),
		OnExit:     nodeOrNil(f.dag.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(f.dag.HandlerOn.Success),
		OnFailure:  nodeOrNil(f.dag.HandlerOn.Failure),
		OnCancel:   nodeOrNil(f.dag.HandlerOn.Cancel),
		Params:     strings.Join(f.dag.Params, " "),
		ParamsList: f.dag.Params,
		StartedAt:  stringutil.FormatTime(time.Time{}),
		FinishedAt: stringutil.FormatTime(time.Time{}),
	}
}

type StatusOption func(*Status)

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

func WithOnExitNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnExit = FromNode(node.Data())
		}
	}
}

func WithOnSuccessNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnSuccess = FromNode(node.Data())
		}
	}
}

func WithOnFailureNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnFailure = FromNode(node.Data())
		}
	}
}

func WithOnCancelNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnCancel = FromNode(node.Data())
		}
	}
}

func WithLogFilePath(logFilePath string) StatusOption {
	return func(s *Status) {
		s.Log = logFilePath
	}
}

func (f *StatusFactory) Create(
	requestID string,
	status scheduler.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) Status {
	statusObj := f.CreateDefault()
	statusObj.RequestID = requestID
	statusObj.Status = status
	statusObj.StatusText = status.String()
	statusObj.PID = PID(pid)
	statusObj.StartedAt = FormatTime(startedAt)

	for _, opt := range opts {
		opt(&statusObj)
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

type StatusFile struct {
	File   string
	Status Status
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
	Params     string           `json:"Params,omitempty"`
	ParamsList []string         `json:"ParamsList,omitempty"`
}

func (st *Status) CorrectRunningStatus() {
	if st.Status == scheduler.StatusRunning {
		st.Status = scheduler.StatusError
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

func nodeOrNil(s *digraph.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNode(*s)
}
