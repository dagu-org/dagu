package sse

import "net/http"

// Event type constants for SSE messages
const (
	// EventTypeData is sent when DAG run status changes
	EventTypeData = "data"
	// EventTypeHeartbeat is sent every 30 seconds to keep connection alive
	EventTypeHeartbeat = "heartbeat"
	// EventTypeConnected is sent when client successfully connects
	EventTypeConnected = "connected"
	// EventTypeError is sent when an error occurs
	EventTypeError = "error"
)

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
