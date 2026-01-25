package sse

import (
	"context"
	"net/http"
)

// Event type constants for SSE messages
const (
	// EventTypeData is sent when data changes
	EventTypeData = "data"
	// EventTypeHeartbeat is sent every 30 seconds to keep connection alive
	EventTypeHeartbeat = "heartbeat"
	// EventTypeConnected is sent when client successfully connects
	EventTypeConnected = "connected"
	// EventTypeError is sent when an error occurs
	EventTypeError = "error"
)

// TopicType identifies the type of data being watched.
// Each topic type has its own registered fetcher function.
type TopicType string

const (
	// TopicTypeDAGRun watches a specific DAG run's status
	// Identifier format: "dagName/dagRunId"
	TopicTypeDAGRun TopicType = "dagrun"

	// TopicTypeDAG watches a DAG's details including latest run
	// Identifier format: "fileName"
	TopicTypeDAG TopicType = "dag"

	// TopicTypeDAGRunLogs watches a DAG run's execution logs
	// Identifier format: "dagName/dagRunId"
	TopicTypeDAGRunLogs TopicType = "dagrunlogs"

	// TopicTypeStepLog watches an individual step's log output
	// Identifier format: "dagName/dagRunId/stepName"
	TopicTypeStepLog TopicType = "steplog"

	// TopicTypeDAGRuns watches the dashboard DAG runs list
	// Identifier format: URL query string (e.g., "limit=50&offset=0")
	TopicTypeDAGRuns TopicType = "dagruns"

	// TopicTypeQueueItems watches queue items
	// Identifier format: "queueName"
	TopicTypeQueueItems TopicType = "queueitems"
)

// FetchFunc fetches data for a given identifier.
// The returned data will be JSON marshaled and sent to clients.
// It should return the same structure as the corresponding REST API endpoint.
type FetchFunc func(ctx context.Context, identifier string) (any, error)

// Event represents an SSE event to be sent to clients
type Event struct {
	Type string
	Data string
}

// SetSSEHeaders sets the standard headers required for SSE responses
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}
