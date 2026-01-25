package sse

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetSSEHeaders(t *testing.T) {
	w := httptest.NewRecorder()

	SetSSEHeaders(w)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

func TestEventTypeConstants(t *testing.T) {
	assert.Equal(t, "data", EventTypeData)
	assert.Equal(t, "heartbeat", EventTypeHeartbeat)
	assert.Equal(t, "connected", EventTypeConnected)
	assert.Equal(t, "error", EventTypeError)
}

func TestTopicTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		topic    TopicType
		expected string
	}{
		{"DAGRun", TopicTypeDAGRun, "dagrun"},
		{"DAG", TopicTypeDAG, "dag"},
		{"DAGRunLogs", TopicTypeDAGRunLogs, "dagrunlogs"},
		{"StepLog", TopicTypeStepLog, "steplog"},
		{"DAGRuns", TopicTypeDAGRuns, "dagruns"},
		{"QueueItems", TopicTypeQueueItems, "queueitems"},
		{"Queues", TopicTypeQueues, "queues"},
		{"DAGsList", TopicTypeDAGsList, "dagslist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, TopicType(tt.expected), tt.topic)
		})
	}
}

func TestEventStruct(t *testing.T) {
	event := Event{
		Type: EventTypeData,
		Data: `{"key": "value"}`,
	}

	assert.Equal(t, EventTypeData, event.Type)
	assert.Equal(t, `{"key": "value"}`, event.Data)
}
