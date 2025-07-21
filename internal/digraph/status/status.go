// Package status defines the status constants for DAG and node execution states.
// These constants are separated to avoid import cycles between scheduler and executor packages.
package status

// Status represents the status of a DAG execution.
type Status int

const (
	StatusNone Status = iota
	StatusRunning
	StatusError
	StatusCancel
	StatusSuccess
	StatusQueued
	StatusPartialSuccess
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusError:
		return "failed"
	case StatusCancel:
		return "canceled"
	case StatusSuccess:
		return "finished"
	case StatusQueued:
		return "queued"
	case StatusPartialSuccess:
		return "partial success"
	case StatusNone:
		fallthrough
	default:
		return "not started"
	}
}

// IsActive checks if the status is active.
func (s Status) IsActive() bool {
	return s == StatusRunning || s == StatusQueued
}

// NodeStatus represents the status of a node in a DAG.
type NodeStatus int

const (
	NodeStatusNone NodeStatus = iota
	NodeStatusRunning
	NodeStatusError
	NodeStatusCancel
	NodeStatusSuccess
	NodeStatusSkipped
	NodeStatusPartialSuccess
)

func (s NodeStatus) String() string {
	switch s {
	case NodeStatusRunning:
		return "running"
	case NodeStatusError:
		return "failed"
	case NodeStatusCancel:
		return "canceled"
	case NodeStatusSuccess:
		return "finished"
	case NodeStatusSkipped:
		return "skipped"
	case NodeStatusPartialSuccess:
		return "partial success"
	case NodeStatusNone:
		fallthrough
	default:
		return "not started"
	}
}