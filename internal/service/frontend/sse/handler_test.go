package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	remoteNodes := map[string]config.RemoteNode{
		"node1": {Name: "node1"},
	}

	handler := NewHandler(hub, remoteNodes, nil)

	require.NotNil(t, handler)
	assert.Same(t, hub, handler.hub)
	assert.NotNil(t, handler.remoteNodes)
	assert.Nil(t, handler.authService)
}

func TestBuildTopic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		topicType TopicType
		parts     []string
		expected  string
	}{
		{
			name:      "single part",
			topicType: TopicTypeDAG,
			parts:     []string{"mydag.yaml"},
			expected:  "dag:mydag.yaml",
		},
		{
			name:      "multiple parts",
			topicType: TopicTypeDAGRun,
			parts:     []string{"mydag", "run123"},
			expected:  "dagrun:mydag/run123",
		},
		{
			name:      "three parts",
			topicType: TopicTypeStepLog,
			parts:     []string{"mydag", "run123", "step1"},
			expected:  "steplog:mydag/run123/step1",
		},
		{
			name:      "empty parts",
			topicType: TopicTypeDAGRuns,
			parts:     []string{""},
			expected:  "dagruns:",
		},
		{
			name:      "query string",
			topicType: TopicTypeDAGRuns,
			parts:     []string{"limit=50&offset=0"},
			expected:  "dagruns:limit=50&offset=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildTopic(tt.topicType, tt.parts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// setupHandler creates a hub with registered fetchers and returns a handler
func setupHandler(t *testing.T) (*Handler, *Hub) {
	t.Helper()
	hub := NewHub()
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)

	// Register all topic types
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.RegisterFetcher(TopicTypeDAG, fetcher)
	hub.RegisterFetcher(TopicTypeDAGRunLogs, fetcher)
	hub.RegisterFetcher(TopicTypeStepLog, fetcher)
	hub.RegisterFetcher(TopicTypeDAGRuns, fetcher)
	hub.RegisterFetcher(TopicTypeQueueItems, fetcher)
	hub.RegisterFetcher(TopicTypeQueues, fetcher)
	hub.RegisterFetcher(TopicTypeDAGsList, fetcher)

	hub.Start()
	t.Cleanup(hub.Shutdown)

	handler := NewHandler(hub, map[string]config.RemoteNode{}, nil)
	return handler, hub
}

// createChiRequest creates a request with chi URL params and timeout.
// Returns the request and a cancel function that should be called when the test is done.
func createChiRequest(method, path string, params map[string]string, timeout time.Duration) (*http.Request, context.CancelFunc) {
	r := httptest.NewRequest(method, path, nil)

	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	return r, cancel
}

func TestHandleEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		path          string
		params        map[string]string
		query         string
		handler       func(*Handler) http.HandlerFunc
		expectedTopic string
	}{
		{
			name:          "DAGRunEvents",
			path:          "/events/dag-runs/mydag/run123",
			params:        map[string]string{"name": "mydag", "dagRunId": "run123"},
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGRunEvents },
			expectedTopic: "dagrun:mydag/run123",
		},
		{
			name:          "DAGEvents",
			path:          "/events/dags/mydag.yaml",
			params:        map[string]string{"fileName": "mydag.yaml"},
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGEvents },
			expectedTopic: "dag:mydag.yaml",
		},
		{
			name:          "DAGRunLogsEvents",
			path:          "/events/dag-runs/mydag/run123/logs",
			params:        map[string]string{"name": "mydag", "dagRunId": "run123"},
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGRunLogsEvents },
			expectedTopic: "dagrunlogs:mydag/run123",
		},
		{
			name:          "DAGRunLogsEventsWithQuery",
			path:          "/events/dag-runs/mydag/run123/logs?tail=100",
			params:        map[string]string{"name": "mydag", "dagRunId": "run123"},
			query:         "tail=100",
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGRunLogsEvents },
			expectedTopic: "dagrunlogs:mydag/run123?tail=100",
		},
		{
			name:          "StepLogEvents",
			path:          "/events/dag-runs/mydag/run123/logs/steps/step1",
			params:        map[string]string{"name": "mydag", "dagRunId": "run123", "stepName": "step1"},
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleStepLogEvents },
			expectedTopic: "steplog:mydag/run123/step1",
		},
		{
			name:          "DAGRunsListEvents",
			path:          "/events/dag-runs?limit=50",
			query:         "limit=50",
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGRunsListEvents },
			expectedTopic: "dagruns:limit=50",
		},
		{
			name:          "QueueItemsEvents",
			path:          "/events/queues/myqueue/items",
			params:        map[string]string{"name": "myqueue"},
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleQueueItemsEvents },
			expectedTopic: "queueitems:myqueue",
		},
		{
			name:          "QueuesListEvents",
			path:          "/events/queues",
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleQueuesListEvents },
			expectedTopic: "queues:",
		},
		{
			name:          "DAGsListEvents",
			path:          "/events/dags?page=1&perPage=100",
			query:         "page=1&perPage=100",
			handler:       func(h *Handler) http.HandlerFunc { return h.HandleDAGsListEvents },
			expectedTopic: `dagslist:page=1\u0026perPage=100`, // & is escaped as \u0026 in JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler, _ := setupHandler(t)
			w := newMockFlusher()
			r, cancel := createChiRequest(http.MethodGet, tt.path, tt.params, 100*time.Millisecond)
			defer cancel()
			if tt.query != "" {
				r.URL.RawQuery = tt.query
			}

			tt.handler(handler)(w, r)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedTopic)
		})
	}
}

func TestHandleDAGRunEventsClientCleanup(t *testing.T) {
	t.Parallel()
	handler, hub := setupHandler(t)

	w := newMockFlusher()
	params := map[string]string{"name": "mydag", "dagRunId": "run123"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dag-runs/mydag/run123", params, 100*time.Millisecond)
	defer cancel()

	handler.HandleDAGRunEvents(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "event: connected")
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHandleSSEHeaders(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := newMockFlusher()
	params := map[string]string{"fileName": "mydag.yaml"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dags/mydag.yaml", params, 100*time.Millisecond)
	defer cancel()

	handler.HandleDAGEvents(w, r)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

func TestHandleSSENonFlusher(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	// Use nonFlusher which doesn't implement http.Flusher (defined in helper_test.go)
	w := &nonFlusher{}
	params := map[string]string{"fileName": "mydag.yaml"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dags/mydag.yaml", params, 100*time.Millisecond)
	defer cancel()

	handler.HandleDAGEvents(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.String(), "streaming not supported")
}

func TestHandleSSERemoteProxy(t *testing.T) {
	t.Parallel()
	// Create a mock remote server
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: connected\ndata: remote\n\n"))
	}))
	defer remoteServer.Close()

	hub := NewHub()
	hub.Start()
	defer hub.Shutdown()

	remoteNodes := map[string]config.RemoteNode{
		"remote1": {
			Name:       "remote1",
			APIBaseURL: remoteServer.URL,
		},
	}
	handler := NewHandler(hub, remoteNodes, nil)

	w := newMockFlusher()
	params := map[string]string{"fileName": "mydag.yaml"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dags/mydag.yaml?remoteNode=remote1", params, 5*time.Second)
	defer cancel()
	r.URL.RawQuery = "remoteNode=remote1"

	handler.HandleDAGEvents(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "remote")
}

func TestHandleSSELocalNode(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := newMockFlusher()
	params := map[string]string{"fileName": "mydag.yaml"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dags/mydag.yaml?remoteNode=local", params, 100*time.Millisecond)
	defer cancel()
	r.URL.RawQuery = "remoteNode=local"

	handler.HandleDAGEvents(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should handle locally, not proxy
	assert.Contains(t, w.Body.String(), "dag:mydag.yaml")
}

func TestValidateAuthNoService(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	handler := NewHandler(hub, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	result := handler.validateAuth(w, r)

	assert.True(t, result, "should pass when no auth service is configured")
}

func TestHandleSSEMaxClients(t *testing.T) {
	t.Parallel()
	hub := NewHub(WithMaxClients(1))
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAG, fetcher)
	hub.Start()
	defer hub.Shutdown()

	handler := NewHandler(hub, map[string]config.RemoteNode{}, nil)

	// First client - use longer timeout
	w1 := newMockFlusher()
	params := map[string]string{"fileName": "dag1.yaml"}
	r1, cancel1 := createChiRequest(http.MethodGet, "/events/dags/dag1.yaml", params, 200*time.Millisecond)
	defer cancel1()

	// Start first client in goroutine
	done1 := make(chan struct{})
	go func() {
		handler.HandleDAGEvents(w1, r1)
		close(done1)
	}()

	// Wait for first client to connect using require.Eventually for deterministic synchronization
	require.Eventually(t, func() bool {
		return hub.ClientCount() == 1
	}, 500*time.Millisecond, 10*time.Millisecond, "first client should connect")

	// Second client should fail (max clients = 1)
	w2 := newMockFlusher()
	r2, cancel2 := createChiRequest(http.MethodGet, "/events/dags/dag2.yaml", map[string]string{"fileName": "dag2.yaml"}, 50*time.Millisecond)
	defer cancel2()

	handler.HandleDAGEvents(w2, r2)

	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Contains(t, w2.Body.String(), "max clients reached")

	<-done1
}

func TestHandleSSEConnectedEvent(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := newMockFlusher()
	params := map[string]string{"fileName": "mydag.yaml"}
	r, cancel := createChiRequest(http.MethodGet, "/events/dags/mydag.yaml", params, 100*time.Millisecond)
	defer cancel()

	handler.HandleDAGEvents(w, r)

	body := w.Body.String()
	assert.Contains(t, body, "event: connected")
	assert.Contains(t, body, `"topic":"dag:mydag.yaml"`)
}
