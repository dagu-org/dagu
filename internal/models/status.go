package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// StatusBuilder creates Status objects for a specific DAG
type StatusBuilder struct {
	dag *digraph.DAG // The DAG for which to create status objects
}

// NewStatusBuilder creates a new StatusFactory for the specified DAG
func NewStatusBuilder(dag *digraph.DAG) *StatusBuilder {
	return &StatusBuilder{dag: dag}
}

// InitialStatus creates an initial Status object for the given DAG
func InitialStatus(dag *digraph.DAG) Status {
	return Status{
		Name:       dag.Name,
		Status:     scheduler.StatusNone,
		PID:        PID(0),
		Nodes:      FromSteps(dag.Steps),
		OnExit:     nodeOrNil(dag.HandlerOn.Exit),
		OnSuccess:  nodeOrNil(dag.HandlerOn.Success),
		OnFailure:  nodeOrNil(dag.HandlerOn.Failure),
		OnCancel:   nodeOrNil(dag.HandlerOn.Cancel),
		Params:     strings.Join(dag.Params, " "),
		ParamsList: dag.Params,
		StartedAt:  stringutil.FormatTime(time.Time{}),
		FinishedAt: stringutil.FormatTime(time.Time{}),
	}
}

// StatusOption is a functional option pattern for configuring Status objects
type StatusOption func(*Status)

// WithHierarchyRefs returns a StatusOption that sets the root DAG information
func WithHierarchyRefs(root digraph.WorkflowRef, parent digraph.WorkflowRef) StatusOption {
	return func(s *Status) {
		s.Root = root
		s.Parent = parent
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
		s.FinishedAt = formatTime(t)
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

// Create builds a Status object for a workflow with the specified parameters
func (f *StatusBuilder) Create(
	workflowID string,
	status scheduler.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) Status {
	statusObj := InitialStatus(f.dag)
	statusObj.WorkflowID = workflowID
	statusObj.Status = status
	statusObj.PID = PID(pid)
	statusObj.StartedAt = formatTime(startedAt)

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

// Status represents the complete execution state of a workflow
type Status struct {
	Root       digraph.WorkflowRef `json:"root,omitempty"`
	Parent     digraph.WorkflowRef `json:"parent,omitempty"`
	Name       string              `json:"name"`
	WorkflowID string              `json:"workflowId"`
	Status     scheduler.Status    `json:"status"`
	PID        PID                 `json:"pid,omitempty"`
	Nodes      []*Node             `json:"nodes,omitempty"`
	OnExit     *Node               `json:"onExit,omitempty"`
	OnSuccess  *Node               `json:"onSuccess,omitempty"`
	OnFailure  *Node               `json:"onFailure,omitempty"`
	OnCancel   *Node               `json:"onCancel,omitempty"`
	StartedAt  string              `json:"startedAt,omitempty"`
	FinishedAt string              `json:"finishedAt,omitempty"`
	Log        string              `json:"log,omitempty"`
	Params     string              `json:"params,omitempty"`
	ParamsList []string            `json:"paramsList,omitempty"`
}

// Workflow returns the execution reference for the current status
func (st *Status) Workflow() digraph.WorkflowRef {
	return digraph.NewWorkflowRef(st.Name, st.WorkflowID)
}

// Errors returns a slice of errors for the current status
func (st *Status) Errors() []error {
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

// PID represents a process ID for a running workflow
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
func nodeOrNil(s *digraph.Step) *Node {
	if s == nil {
		return nil
	}
	return NewNode(*s)
}
