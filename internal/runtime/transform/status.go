package transform

import (
	"time"

	"github.com/dagu-org/dagu/internal/core"
	core1 "github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
)

// StatusBuilder creates Status objects for a specific DAG
type StatusBuilder struct {
	dag *core.DAG // The DAG for which to create status objects
}

// NewStatusBuilder creates a new StatusFactory for the specified DAG
func NewStatusBuilder(dag *core.DAG) *StatusBuilder {
	return &StatusBuilder{dag: dag}
}

// StatusOption is a functional option pattern for configuring Status objects
type StatusOption func(*execution.DAGRunStatus)

// WithHierarchyRefs returns a StatusOption that sets the root DAG information
func WithHierarchyRefs(root execution.DAGRunRef, parent execution.DAGRunRef) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.Root = root
		s.Parent = parent
	}
}

// WithNodes returns a StatusOption that sets the node data for the status
func WithNodes(nodes []runtime.NodeData) StatusOption {
	return func(s *execution.DAGRunStatus) {
		convertedNode := make([]*execution.Node, len(nodes))
		for i, n := range nodes {
			convertedNode[i] = newNode(n)
		}
		s.Nodes = convertedNode
	}
}

func WithAttemptID(attemptID string) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.AttemptID = attemptID
	}
}

// WithQueuedAt returns a StatusOption that sets the finished time
func WithQueuedAt(formattedTime string) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.QueuedAt = formattedTime
	}
}

// WithCreatedAt returns a StatusOption that sets the created time
func WithCreatedAt(t int64) StatusOption {
	return func(s *execution.DAGRunStatus) {
		if t == 0 {
			t = time.Now().UnixMilli()
		}
		s.CreatedAt = t
	}
}

// WithFinishedAt returns a StatusOption that sets the finished time
func WithFinishedAt(t time.Time) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.FinishedAt = execution.FormatTime(t)
	}
}

// WithOnExitNode returns a StatusOption that sets the exit handler node
func WithOnExitNode(node *runtime.Node) StatusOption {
	return func(s *execution.DAGRunStatus) {
		if node != nil {
			s.OnExit = newNode(node.NodeData())
		}
	}
}

// WithOnSuccessNode returns a StatusOption that sets the success handler node
func WithOnSuccessNode(node *runtime.Node) StatusOption {
	return func(s *execution.DAGRunStatus) {
		if node != nil {
			s.OnSuccess = newNode(node.NodeData())
		}
	}
}

// WithOnFailureNode returns a StatusOption that sets the failure handler node
func WithOnFailureNode(node *runtime.Node) StatusOption {
	return func(s *execution.DAGRunStatus) {
		if node != nil {
			s.OnFailure = newNode(node.NodeData())
		}
	}
}

// WithOnCancelNode returns a StatusOption that sets the cancel handler node
func WithOnCancelNode(node *runtime.Node) StatusOption {
	return func(s *execution.DAGRunStatus) {
		if node != nil {
			s.OnCancel = newNode(node.NodeData())
		}
	}
}

// WithLogFilePath returns a StatusOption that sets the log file path
func WithLogFilePath(logFilePath string) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.Log = logFilePath
	}
}

// WithPreconditions returns a StatusOption that sets the preconditions
func WithPreconditions(conditions []*core.Condition) StatusOption {
	return func(s *execution.DAGRunStatus) {
		s.Preconditions = conditions
	}
}

// Create builds a Status object for a dag-run with the specified parameters
func (f *StatusBuilder) Create(
	dagRunID string,
	status core1.Status,
	pid int,
	startedAt time.Time,
	opts ...StatusOption,
) execution.DAGRunStatus {
	statusObj := execution.InitialStatus(f.dag)
	statusObj.DAGRunID = dagRunID
	statusObj.Status = status
	statusObj.PID = execution.PID(pid)
	statusObj.StartedAt = execution.FormatTime(startedAt)
	statusObj.CreatedAt = time.Now().UnixMilli()

	for _, opt := range opts {
		opt(&statusObj)
	}

	return statusObj
}
