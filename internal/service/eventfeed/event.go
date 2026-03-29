// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import "time"

// EventType identifies the lifecycle action captured in the internal event feed.
type EventType string

const (
	EventTypeWaiting  EventType = "waiting"
	EventTypeFailed   EventType = "failed"
	EventTypeAborted  EventType = "aborted"
	EventTypeApproved EventType = "approved"
	EventTypeRejected EventType = "rejected"
	EventTypePushBack EventType = "push_back"
)

// Entry is a single event-feed record.
type Entry struct {
	ID                 string    `json:"id"`
	Timestamp          time.Time `json:"timestamp"`
	Type               EventType `json:"type"`
	DAGName            string    `json:"dagName"`
	DAGRunID           string    `json:"dagRunId"`
	SubDAGRunID        string    `json:"subDAGRunId,omitempty"`
	AttemptID          string    `json:"attemptId,omitempty"`
	StepName           string    `json:"stepName,omitempty"`
	Actor              string    `json:"actor,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	ApprovalIteration  *int      `json:"approvalIteration,omitempty"`
	Resumed            *bool     `json:"resumed,omitempty"`
	ResultingRunStatus string    `json:"resultingRunStatus,omitempty"`
}

// QueryFilter narrows an event-feed query.
type QueryFilter struct {
	Type      EventType
	DAGName   string
	DAGRunID  string
	Actor     string
	Search    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
	Offset    int
}

// QueryResult contains paginated query results.
type QueryResult struct {
	Entries []Entry
	Total   int
}
