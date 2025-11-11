package core

// Status represents the canonical lifecycle phases for a DAG execution.
type Status int

const (
	NotStarted Status = iota
	Running
	Failed
	Aborted
	Succeeded
	Queued
	PartiallySucceeded
)

// String returns the canonical lowercase token used across APIs, logs, and
// environment variables.
func (s Status) String() string {
	switch s {
	case Running:
		return "running"
	case Failed:
		return "failed"
	case Aborted:
		return "aborted"
	case Succeeded:
		return "succeeded"
	case Queued:
		return "queued"
	case PartiallySucceeded:
		return "partially_succeeded"
	case NotStarted:
		return "not_started"
	default:
		return "unknown"
	}
}

// IsActive checks if the status is active.
func (s Status) IsActive() bool {
	return s == Running || s == Queued
}

// IsSuccess checks if the status indicates a successful execution.
func (s Status) IsSuccess() bool {
	return s == Succeeded || s == PartiallySucceeded
}

// NodeStatus represents the canonical lifecycle phases for an individual node.
type NodeStatus int

const (
	NodeNotStarted NodeStatus = iota
	NodeRunning
	NodeFailed
	NodeAborted
	NodeSucceeded
	NodeSkipped
	NodePartiallySucceeded
)

// IsSuccess checks if the node status indicates a successful execution.
func (s NodeStatus) IsSuccess() bool {
	return s == NodeSucceeded || s == NodePartiallySucceeded
}

// String returns the canonical lowercase token for the node lifecycle phase.
func (s NodeStatus) String() string {
	switch s {
	case NodeRunning:
		return "running"
	case NodeFailed:
		return "failed"
	case NodeAborted:
		return "aborted"
	case NodeSucceeded:
		return "succeeded"
	case NodeSkipped:
		return "skipped"
	case NodePartiallySucceeded:
		return "partially_succeeded"
	case NodeNotStarted:
		return "not_started"
	default:
		return "unknown"
	}
}
