package persistence

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// StatusFactory creates Status objects for a specific DAG
type StatusFactory struct {
	dag *digraph.DAG // The DAG for which to create status objects
}

// NewStatusFactory creates a new StatusFactory for the specified DAG
func NewStatusFactory(dag *digraph.DAG) *StatusFactory {
	return &StatusFactory{dag: dag}
}

// Default creates a default Status object for the DAG with initial values
func (f *StatusFactory) Default() Status {
	return Status{
		Name:       f.dag.GetName(),
		Status:     scheduler.StatusNone,
		StatusText: scheduler.StatusNone.String(),
		PID:        PID(0),
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

// StatusOption is a functional option pattern for configuring Status objects
type StatusOption func(*Status)

// WithRootDAG returns a StatusOption that sets the root DAG information
func WithRootDAG(rootDAG digraph.RootDAG) StatusOption {
	return func(s *Status) {
		s.RootRequestID = rootDAG.RequestID
		s.RootDAGName = rootDAG.Name
	}
}

// WithNodes returns a StatusOption that sets the node data for the status
func WithNodes(nodes []scheduler.NodeData) StatusOption {
	return func(s *Status) {
		s.Nodes = FromNodes(nodes)
	}
}

// WithFinishedAt returns a StatusOption that sets the finished time
func WithFinishedAt(t time.Time) StatusOption {
	return func(s *Status) {
		s.FinishedAt = FormatTime(t)
	}
}

// WithOnExitNode returns a StatusOption that sets the exit handler node
func WithOnExitNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnExit = FromNode(node.NodeData())
		}
	}
}

// WithOnSuccessNode returns a StatusOption that sets the success handler node
func WithOnSuccessNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnSuccess = FromNode(node.NodeData())
		}
	}
}

// WithOnFailureNode returns a StatusOption that sets the failure handler node
func WithOnFailureNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnFailure = FromNode(node.NodeData())
		}
	}
}

// WithOnCancelNode returns a StatusOption that sets the cancel handler node
func WithOnCancelNode(node *scheduler.Node) StatusOption {
	return func(s *Status) {
		if node != nil {
			s.OnCancel = FromNode(node.NodeData())
		}
	}
}

// WithLogFilePath returns a StatusOption that sets the log file path
func WithLogFilePath(logFilePath string) StatusOption {
	return func(s *Status) {
		s.Log = logFilePath
	}
}

// Create builds a Status object for a DAG run with the specified parameters
func (f *StatusFactory) Create(
	requestID string,
	status scheduler.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) Status {
	statusObj := f.Default()
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

// StatusFromJSON deserializes a JSON string into a Status object
func StatusFromJSON(s string) (*Status, error) {
	status := new(Status)
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, err
}

// Status represents the complete execution state of a DAG run
type Status struct {
	RootDAGName   string           `json:"rootDAGName,omitempty"`
	RootRequestID string           `json:"rootRequestId,omitempty"`
	RequestID     string           `json:"requestId,omitempty"`
	Name          string           `json:"name,omitempty"`
	Status        scheduler.Status `json:"status"`
	StatusText    string           `json:"statusText"`
	PID           PID              `json:"pid,omitempty"`
	Nodes         []*Node          `json:"nodes,omitempty"`
	OnExit        *Node            `json:"onExit,omitempty"`
	OnSuccess     *Node            `json:"onSuccess,omitempty"`
	OnFailure     *Node            `json:"onFailure,omitempty"`
	OnCancel      *Node            `json:"onCancel,omitempty"`
	StartedAt     string           `json:"startedAt,omitempty"`
	FinishedAt    string           `json:"finishedAt,omitempty"`
	Log           string           `json:"log,omitempty"`
	Params        string           `json:"params,omitempty"`
	ParamsList    []string         `json:"paramsList,omitempty"`
}

// SetStatusToErrorIfRunning changes the status to Error if it is currently Running
func (st *Status) SetStatusToErrorIfRunning() {
	if st.Status == scheduler.StatusRunning {
		st.Status = scheduler.StatusError
		st.StatusText = st.Status.String()
	}
}

// NodesByName returns a slice of nodes with the specified name
func (st *Status) NodeByName(name string) (*Node, error) {
	for _, node := range st.Nodes {
		if node.Step.Name == name {
			return node, nil
		}
	}
	if st.OnExit != nil && st.OnExit.Step.Name == name {
		return st.OnExit, nil
	}
	if st.OnSuccess != nil && st.OnSuccess.Step.Name == name {
		return st.OnSuccess, nil
	}
	if st.OnFailure != nil && st.OnFailure.Step.Name == name {
		return st.OnFailure, nil
	}
	if st.OnCancel != nil && st.OnCancel.Step.Name == name {
		return st.OnCancel, nil
	}
	return nil, fmt.Errorf("node %s not found", name)
}

// PID represents a process ID for a running DAG run
type PID int

// String returns the string representation of the PID, or an empty string if 0
func (p PID) String() string {
	if p <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", p)
}

// FormatTime formats a time.Time or returns empty string if it's the zero value
func FormatTime(val time.Time) string {
	if val.IsZero() {
		return ""
	}
	return stringutil.FormatTime(val)
}

// nodeOrNil creates a Node from a Step or returns nil if the step is nil
func nodeOrNil(s *digraph.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNode(*s)
}
