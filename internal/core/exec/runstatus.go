package exec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

// InitialStatus creates an initial Status object for the given DAG
func InitialStatus(dag *core.DAG) DAGRunStatus {
	return DAGRunStatus{
		Name:          dag.Name,
		Status:        core.NotStarted,
		PID:           PID(0),
		Nodes:         NewNodesFromSteps(dag.Steps),
		OnInit:        NewNodeOrNil(dag.HandlerOn.Init),
		OnExit:        NewNodeOrNil(dag.HandlerOn.Exit),
		OnSuccess:     NewNodeOrNil(dag.HandlerOn.Success),
		OnFailure:     NewNodeOrNil(dag.HandlerOn.Failure),
		OnCancel:      NewNodeOrNil(dag.HandlerOn.Cancel),
		OnWait:        NewNodeOrNil(dag.HandlerOn.Wait),
		Params:        strings.Join(dag.Params, " "),
		ParamsList:    dag.Params,
		CreatedAt:     time.Now().UnixMilli(),
		StartedAt:     stringutil.FormatTime(time.Time{}),
		FinishedAt:    stringutil.FormatTime(time.Time{}),
		Preconditions: dag.Preconditions,
		Tags:          dag.Tags.Strings(),
	}
}

// DAGRunStatus represents the complete execution state of a dag-run.
type DAGRunStatus struct {
	Root          DAGRunRef         `json:"root,omitzero"`
	Parent        DAGRunRef         `json:"parent,omitzero"`
	Name          string            `json:"name"`
	DAGRunID      string            `json:"dagRunId"`
	AttemptID     string            `json:"attemptId"`
	AttemptKey    string            `json:"attemptKey,omitempty"` // Globally unique attempt identifier
	Status        core.Status       `json:"status"`
	TriggerType   core.TriggerType  `json:"triggerType,omitempty"`
	ScheduledTime string            `json:"scheduledTime,omitempty"`
	WorkerID      string            `json:"workerId,omitempty"`
	PID           PID               `json:"pid,omitempty"`
	Nodes         []*Node           `json:"nodes,omitempty"`
	OnInit        *Node             `json:"onInit,omitempty"`
	OnExit        *Node             `json:"onExit,omitempty"`
	OnSuccess     *Node             `json:"onSuccess,omitempty"`
	OnFailure     *Node             `json:"onFailure,omitempty"`
	OnCancel      *Node             `json:"onCancel,omitempty"`
	OnWait        *Node             `json:"onWait,omitempty"`
	CreatedAt     int64             `json:"createdAt,omitempty"`
	QueuedAt      string            `json:"queuedAt,omitempty"`
	StartedAt     string            `json:"startedAt,omitempty"`
	FinishedAt    string            `json:"finishedAt,omitempty"`
	Log           string            `json:"log,omitempty"`
	Error         string            `json:"error,omitempty"`
	Params        string            `json:"params,omitempty"`
	ParamsList    []string          `json:"paramsList,omitempty"`
	Preconditions []*core.Condition `json:"preconditions,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
}

// DAGRun returns a reference to the dag-run associated with this status
func (st *DAGRunStatus) DAGRun() DAGRunRef {
	return NewDAGRunRef(st.Name, st.DAGRunID)
}

// Errors returns a slice of errors for the current status
func (st *DAGRunStatus) Errors() []error {
	var errs []error
	if st.Error != "" {
		errs = append(errs, errors.New(st.Error))
	}
	for _, node := range st.Nodes {
		if node.Error != "" {
			errs = append(errs, fmt.Errorf("node %s: %s", node.Step.Name, node.Error))
		}
	}
	for _, handler := range st.handlerNodes() {
		if handler.node != nil && handler.node.Error != "" {
			errs = append(errs, fmt.Errorf("%s: %s", handler.name, handler.node.Error))
		}
	}
	return errs
}

// NodeByName returns the node with the specified name.
// For handlers, it matches on both the handler label (e.g., "onSuccess")
// and the step name within the handler.
func (st *DAGRunStatus) NodeByName(name string) (*Node, error) {
	for _, node := range st.Nodes {
		if node.Step.Name == name {
			return node, nil
		}
	}
	for _, handler := range st.handlerNodes() {
		if handler.node != nil {
			// Match on handler label (e.g., "onSuccess") or step name
			if handler.name == name || handler.node.Step.Name == name {
				return handler.node, nil
			}
		}
	}
	return nil, fmt.Errorf("node %s not found", name)
}

// StatusFromJSON deserializes a JSON string into a Status object
func StatusFromJSON(s string) (*DAGRunStatus, error) {
	var status DAGRunStatus
	if err := json.Unmarshal([]byte(s), &status); err != nil {
		return nil, err
	}
	return &status, nil
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

// FormatTime formats a time.Time or returns empty string if it's the zero value.
// This is a convenience wrapper around stringutil.FormatTime.
func FormatTime(val time.Time) string {
	return stringutil.FormatTime(val)
}

// handlerNode pairs a handler node with its name for iteration
type handlerNode struct {
	name string
	node *Node
}

// handlerNodes returns all handler nodes for iteration
func (st *DAGRunStatus) handlerNodes() []handlerNode {
	return []handlerNode{
		{"onInit", st.OnInit},
		{"onExit", st.OnExit},
		{"onSuccess", st.OnSuccess},
		{"onFailure", st.OnFailure},
		{"onCancel", st.OnCancel},
		{"onWait", st.OnWait},
	}
}
