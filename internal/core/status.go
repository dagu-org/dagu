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
	Waiting
	Rejected
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
	case Waiting:
		return "waiting"
	case Rejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// IsActive checks if the status is active (not yet completed).
// This includes Running, Queued, and Wait (waiting for human approval).
func (s Status) IsActive() bool {
	return s == Running || s == Queued || s == Waiting
}

// IsSuccess checks if the status indicates a successful execution.
func (s Status) IsSuccess() bool {
	return s == Succeeded || s == PartiallySucceeded
}

// IsWaiting checks if the status is waiting for human approval.
func (s Status) IsWaiting() bool {
	return s == Waiting
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
	NodeWaiting
	NodeRejected
)

// IsSuccess checks if the node status indicates a successful execution.
func (s NodeStatus) IsSuccess() bool {
	return s == NodeSucceeded || s == NodePartiallySucceeded
}

// IsWaiting checks if the node status is waiting for human approval.
func (s NodeStatus) IsWaiting() bool {
	return s == NodeWaiting
}

// IsDone checks if the node has completed (success, failure, skipped, aborted, rejected, or partially succeeded).
func (s NodeStatus) IsDone() bool {
	return s == NodeSucceeded ||
		s == NodeFailed ||
		s == NodeSkipped ||
		s == NodeAborted ||
		s == NodePartiallySucceeded ||
		s == NodeRejected
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
	case NodeWaiting:
		return "waiting"
	case NodeRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// TriggerType represents how a DAG run was initiated.
type TriggerType int

const (
	TriggerTypeUnknown TriggerType = iota
	TriggerTypeScheduler
	TriggerTypeManual
	TriggerTypeWebhook
	TriggerTypeSubDAG
)

// String returns the canonical lowercase token for the trigger type.
func (t TriggerType) String() string {
	switch t {
	case TriggerTypeScheduler:
		return "scheduler"
	case TriggerTypeManual:
		return "manual"
	case TriggerTypeWebhook:
		return "webhook"
	case TriggerTypeSubDAG:
		return "subdag"
	default:
		return "unknown"
	}
}

// ParseTriggerType parses a string into a TriggerType.
func ParseTriggerType(s string) TriggerType {
	switch s {
	case "scheduler":
		return TriggerTypeScheduler
	case "manual":
		return TriggerTypeManual
	case "webhook":
		return TriggerTypeWebhook
	case "subdag":
		return TriggerTypeSubDAG
	default:
		return TriggerTypeUnknown
	}
}
