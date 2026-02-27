package sse

import (
	"context"
	"net/http"
)

// Event type constants for SSE messages.
const (
	EventTypeData      = "data"
	EventTypeHeartbeat = "heartbeat"
	EventTypeConnected = "connected"
	EventTypeError     = "error"
)

// TopicType identifies the type of data being watched.
type TopicType string

// Topic type constants. Each has a registered fetcher function.
// Identifier formats:
//   - TopicTypeDAGRun: "dagName/dagRunId"
//   - TopicTypeDAG: "fileName"
//   - TopicTypeDAGHistory: "fileName"
//   - TopicTypeDAGRunLogs: "dagName/dagRunId"
//   - TopicTypeStepLog: "dagName/dagRunId/stepName"
//   - TopicTypeDAGRuns: URL query string (e.g., "limit=50&offset=0")
//   - TopicTypeQueueItems: "queueName"
//   - TopicTypeQueues: URL query string
//   - TopicTypeDAGsList: URL query string (e.g., "page=1&perPage=100&name=mydag")
//   - TopicTypeDoc: document path (e.g., "runbooks/deployment")
//   - TopicTypeDocTree: URL query string (e.g., "page=1&perPage=200")
const (
	TopicTypeDAGRun     TopicType = "dagrun"
	TopicTypeDAG        TopicType = "dag"
	TopicTypeDAGHistory TopicType = "daghistory"
	TopicTypeDAGRunLogs TopicType = "dagrunlogs"
	TopicTypeStepLog    TopicType = "steplog"
	TopicTypeDAGRuns    TopicType = "dagruns"
	TopicTypeQueueItems TopicType = "queueitems"
	TopicTypeQueues     TopicType = "queues"
	TopicTypeDAGsList   TopicType = "dagslist"
	TopicTypeDoc        TopicType = "doc"
	TopicTypeDocTree    TopicType = "doctree"
)

// FetchFunc fetches data for a given identifier. The returned data is JSON
// marshaled and sent to clients with the same structure as the REST API.
type FetchFunc func(ctx context.Context, identifier string) (any, error)

// Event represents an SSE event to be sent to clients.
type Event struct {
	Type string
	Data string
}

// SetSSEHeaders sets the standard headers required for SSE responses.
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}
