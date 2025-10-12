package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/scheduler"
	"github.com/dagu-org/dagu/internal/core/status"
)

// StatusBuilder creates Status objects for a specific DAG
type StatusBuilder struct {
	dag *core.DAG // The DAG for which to create status objects
}

// NewStatusBuilder creates a new StatusFactory for the specified DAG
func NewStatusBuilder(dag *core.DAG) *StatusBuilder {
	return &StatusBuilder{dag: dag}
}

// InitialStatus creates an initial Status object for the given DAG
func InitialStatus(dag *core.DAG) DAGRunStatus {
	return DAGRunStatus{
		Name:          dag.Name,
		Status:        status.None,
		PID:           PID(0),
		Nodes:         NodesFromSteps(dag.Steps),
		OnExit:        nodeOrNil(dag.HandlerOn.Exit),
		OnSuccess:     nodeOrNil(dag.HandlerOn.Success),
		OnFailure:     nodeOrNil(dag.HandlerOn.Failure),
		OnCancel:      nodeOrNil(dag.HandlerOn.Cancel),
		Params:        strings.Join(dag.Params, " "),
		ParamsList:    dag.Params,
		CreatedAt:     time.Now().UnixMilli(),
		StartedAt:     stringutil.FormatTime(time.Time{}),
		FinishedAt:    stringutil.FormatTime(time.Time{}),
		Preconditions: dag.Preconditions,
	}
}

// StatusOption is a functional option pattern for configuring Status objects
type StatusOption func(*DAGRunStatus)

// WithHierarchyRefs returns a StatusOption that sets the root DAG information
func WithHierarchyRefs(root core.DAGRunRef, parent core.DAGRunRef) StatusOption {
	return func(s *DAGRunStatus) {
		s.Root = root
		s.Parent = parent
	}
}

// WithNodes returns a StatusOption that sets the node data for the status
func WithNodes(nodes []scheduler.NodeData) StatusOption {
	return func(s *DAGRunStatus) {
		s.Nodes = s.setNodes(nodes)
	}
}

func WithAttemptID(attemptID string) StatusOption {
	return func(s *DAGRunStatus) {
		s.AttemptID = attemptID
	}
}

// WithQueuedAt returns a StatusOption that sets the finished time
func WithQueuedAt(formattedTime string) StatusOption {
	return func(s *DAGRunStatus) {
		s.QueuedAt = formattedTime
	}
}

// WithCreatedAt returns a StatusOption that sets the created time
func WithCreatedAt(t int64) StatusOption {
	return func(s *DAGRunStatus) {
		if t == 0 {
			t = time.Now().UnixMilli()
		}
		s.CreatedAt = t
	}
}

// WithFinishedAt returns a StatusOption that sets the finished time
func WithFinishedAt(t time.Time) StatusOption {
	return func(s *DAGRunStatus) {
		s.FinishedAt = formatTime(t)
	}
}

// WithOnExitNode returns a StatusOption that sets the exit handler node
func WithOnExitNode(node *scheduler.Node) StatusOption {
	return func(s *DAGRunStatus) {
		if node != nil {
			s.OnExit = newNode(node.NodeData())
		}
	}
}

// WithOnSuccessNode returns a StatusOption that sets the success handler node
func WithOnSuccessNode(node *scheduler.Node) StatusOption {
	return func(s *DAGRunStatus) {
		if node != nil {
			s.OnSuccess = newNode(node.NodeData())
		}
	}
}

// WithOnFailureNode returns a StatusOption that sets the failure handler node
func WithOnFailureNode(node *scheduler.Node) StatusOption {
	return func(s *DAGRunStatus) {
		if node != nil {
			s.OnFailure = newNode(node.NodeData())
		}
	}
}

// WithOnCancelNode returns a StatusOption that sets the cancel handler node
func WithOnCancelNode(node *scheduler.Node) StatusOption {
	return func(s *DAGRunStatus) {
		if node != nil {
			s.OnCancel = newNode(node.NodeData())
		}
	}
}

// WithLogFilePath returns a StatusOption that sets the log file path
func WithLogFilePath(logFilePath string) StatusOption {
	return func(s *DAGRunStatus) {
		s.Log = logFilePath
	}
}

// WithPreconditions returns a StatusOption that sets the preconditions
func WithPreconditions(conditions []*core.Condition) StatusOption {
	return func(s *DAGRunStatus) {
		s.Preconditions = conditions
	}
}

// Create builds a Status object for a dag-run with the specified parameters
func (f *StatusBuilder) Create(
	dagRunID string,
	status status.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) DAGRunStatus {
	statusObj := InitialStatus(f.dag)
	statusObj.DAGRunID = dagRunID
	statusObj.Status = status
	statusObj.PID = PID(pid)
	statusObj.StartedAt = formatTime(startedAt)
	statusObj.CreatedAt = time.Now().UnixMilli()

	for _, opt := range opts {
		opt(&statusObj)
	}

	return statusObj
}

// StatusFromJSON deserializes a JSON string into a Status object
func StatusFromJSON(s string) (*DAGRunStatus, error) {
	status := new(DAGRunStatus)
	err := json.Unmarshal([]byte(s), status)
	if err != nil {
		return nil, err
	}
	return status, nil
}

// DAGRunStatus represents the complete execution state of a dag-run.
type DAGRunStatus struct {
	Root          core.DAGRunRef    `json:"root,omitzero"`
	Parent        core.DAGRunRef    `json:"parent,omitzero"`
	Name          string            `json:"name"`
	DAGRunID      string            `json:"dagRunId"`
	AttemptID     string            `json:"attemptId"`
	Status        status.Status     `json:"status"`
	PID           PID               `json:"pid,omitempty"`
	Nodes         []*Node           `json:"nodes,omitempty"`
	OnExit        *Node             `json:"onExit,omitempty"`
	OnSuccess     *Node             `json:"onSuccess,omitempty"`
	OnFailure     *Node             `json:"onFailure,omitempty"`
	OnCancel      *Node             `json:"onCancel,omitempty"`
	CreatedAt     int64             `json:"createdAt,omitempty"`
	QueuedAt      string            `json:"queuedAt,omitempty"`
	StartedAt     string            `json:"startedAt,omitempty"`
	FinishedAt    string            `json:"finishedAt,omitempty"`
	Log           string            `json:"log,omitempty"`
	Params        string            `json:"params,omitempty"`
	ParamsList    []string          `json:"paramsList,omitempty"`
	Preconditions []*core.Condition `json:"preconditions,omitempty"`
}

// DAGRun returns a reference to the dag-run associated with this status
func (st *DAGRunStatus) DAGRun() core.DAGRunRef {
	return core.NewDAGRunRef(st.Name, st.DAGRunID)
}

// Errors returns a slice of errors for the current status
func (st *DAGRunStatus) Errors() []error {
	var errs []error
	for _, node := range st.Nodes {
		if node.Error != "" {
			errs = append(errs, fmt.Errorf("node %s: %s", node.Step.Name, node.Error))
		}
	}
	if st.OnExit != nil && st.OnExit.Error != "" {
		errs = append(errs, fmt.Errorf("onExit: %s", st.OnExit.Error))
	}
	if st.OnSuccess != nil && st.OnSuccess.Error != "" {
		errs = append(errs, fmt.Errorf("onSuccess: %s", st.OnSuccess.Error))
	}
	if st.OnFailure != nil && st.OnFailure.Error != "" {
		errs = append(errs, fmt.Errorf("onFailure: %s", st.OnFailure.Error))
	}
	if st.OnCancel != nil && st.OnCancel.Error != "" {
		errs = append(errs, fmt.Errorf("onCancel: %s", st.OnCancel.Error))
	}
	return errs
}

// NodesByName returns a slice of nodes with the specified name
func (st *DAGRunStatus) NodeByName(name string) (*Node, error) {
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

// setNodes converts scheduler NodeData objects to persistence Node objects
func (s *DAGRunStatus) setNodes(nodes []scheduler.NodeData) []*Node {
	var ret []*Node
	for _, node := range nodes {
		ret = append(ret, newNode(node))
	}
	return ret
}

// PID represents a process ID for a running dag-run
type PID int

// String returns the string representation of the PID, or an empty string if 0
func (p PID) String() string {
	if p <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", p)
}

// formatTime formats a time.Time or returns empty string if it's the zero value
func formatTime(val time.Time) string {
	if val.IsZero() {
		return ""
	}
	return stringutil.FormatTime(val)
}

// nodeOrNil creates a Node from a Step or returns nil if the step is nil
func nodeOrNil(s *core.Step) *Node {
	if s == nil {
		return nil
	}
	return newNodeFromStep(*s)
}
