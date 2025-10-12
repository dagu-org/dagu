// Package status defines the status constants for DAG and node execution states.
// These constants are separated to avoid import cycles between scheduler and executor packages.
package status

// Status represents the status of a DAG execution.
type Status int

const (
	None Status = iota
	Running
	Error
	Cancel
	Success
	Queued
	PartialSuccess
)

func (s Status) String() string {
	switch s {
	case Running:
		return "running"
	case Error:
		return "failed"
	case Cancel:
		return "cancelled"
	case Success:
		return "finished"
	case Queued:
		return "queued"
	case PartialSuccess:
		return "partial success"
	case None:
		fallthrough
	default:
		return "not started"
	}
}

// IsActive checks if the status is active.
func (s Status) IsActive() bool {
	return s == Running || s == Queued
}

// IsSuccess checks if the status indicates a successful execution.
func (s Status) IsSuccess() bool {
	return s == Success || s == PartialSuccess
}

// NodeStatus represents the status of a node in a DAG.
type NodeStatus int

const (
	NodeNone NodeStatus = iota
	NodeRunning
	NodeError
	NodeCancel
	NodeSuccess
	NodeSkipped
	NodePartialSuccess
)

// IsSuccess checks if the node status indicates a successful execution.
func (s NodeStatus) IsSuccess() bool {
	return s == NodeSuccess || s == NodePartialSuccess
}

func (s NodeStatus) String() string {
	switch s {
	case NodeRunning:
		return "running"
	case NodeError:
		return "failed"
	case NodeCancel:
		return "cancelled"
	case NodeSuccess:
		return "finished"
	case NodeSkipped:
		return "skipped"
	case NodePartialSuccess:
		return "partial success"
	case NodeNone:
		fallthrough
	default:
		return "not started"
	}
}
