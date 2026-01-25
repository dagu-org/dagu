package sse

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRemoteURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		baseURL  string
		topic    string
		token    string
		expected string
	}{
		{
			name:     "dagrun topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagrun:mydag/run123",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs/mydag/run123",
		},
		{
			name:     "dag topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dag:mydag.yaml",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dags/mydag.yaml",
		},
		{
			name:     "dagrunlogs topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagrunlogs:mydag/run123",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs/mydag/run123/logs",
		},
		{
			name:     "steplog topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "steplog:mydag/run123/step1",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs/mydag/run123/logs/steps/step1",
		},
		{
			name:     "dagruns topic with query",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagruns:limit=50&offset=0",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs?limit=50&offset=0",
		},
		{
			name:     "dagruns topic without query",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagruns:",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs",
		},
		{
			name:     "queueitems topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "queueitems:myqueue",
			token:    "",
			expected: "http://remote:8080/api/v2/events/queues/myqueue/items",
		},
		{
			name:     "queues topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "queues:limit=10",
			token:    "",
			expected: "http://remote:8080/api/v2/events/queues?limit=10",
		},
		{
			name:     "dagslist topic",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagslist:page=1&perPage=100",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dags?page=1&perPage=100",
		},
		{
			name:     "with token",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagrun:mydag/run123",
			token:    "abc123",
			expected: "http://remote:8080/api/v2/events/dag-runs/mydag/run123?token=abc123",
		},
		{
			name:     "with token and existing query",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "dagruns:limit=50",
			token:    "abc123",
			expected: "http://remote:8080/api/v2/events/dag-runs?limit=50&token=abc123",
		},
		{
			name:     "trailing slash in baseURL",
			baseURL:  "http://remote:8080/api/v2/",
			topic:    "dagrun:mydag/run123",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs/mydag/run123",
		},
		{
			name:     "topic without colon (fallback)",
			baseURL:  "http://remote:8080/api/v2",
			topic:    "invalidtopic",
			token:    "",
			expected: "http://remote:8080/api/v2/events/dag-runs/invalidtopic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildRemoteURL(tt.baseURL, tt.topic, tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPathForTopic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		topicType  TopicType
		identifier string
		expected   string
	}{
		{TopicTypeDAGRun, "mydag/run123", "/events/dag-runs/mydag/run123"},
		{TopicTypeDAG, "mydag.yaml", "/events/dags/mydag.yaml"},
		{TopicTypeDAGRunLogs, "mydag/run123", "/events/dag-runs/mydag/run123/logs"},
		{TopicTypeStepLog, "mydag/run123/step1", "/events/dag-runs/mydag/run123/logs/steps/step1"},
		{TopicTypeDAGRuns, "limit=50", "/events/dag-runs?limit=50"},
		{TopicTypeDAGRuns, "", "/events/dag-runs"},
		{TopicTypeQueueItems, "myqueue", "/events/queues/myqueue/items"},
		{TopicTypeQueues, "limit=10", "/events/queues?limit=10"},
		{TopicTypeQueues, "", "/events/queues"},
		{TopicTypeDAGsList, "page=1", "/events/dags?page=1"},
		{TopicTypeDAGsList, "", "/events/dags"},
		{TopicType("unknown"), "identifier", "/events/unknown/identifier"},
	}

	for _, tt := range tests {
		t.Run(string(tt.topicType)+"_"+tt.identifier, func(t *testing.T) {
			t.Parallel()
			result := buildPathForTopic(tt.topicType, tt.identifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepLogPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{
			name:       "full path with 3 parts",
			identifier: "mydag/run123/step1",
			expected:   "/events/dag-runs/mydag/run123/logs/steps/step1",
		},
		{
			name:       "path with special characters",
			identifier: "my-dag/run-123/step-1",
			expected:   "/events/dag-runs/my-dag/run-123/logs/steps/step-1",
		},
		{
			name:       "path with fewer parts",
			identifier: "mydag/run123",
			expected:   "/events/dag-runs/mydag/run123/logs/steps",
		},
		{
			name:       "single part",
			identifier: "mydag",
			expected:   "/events/dag-runs/mydag/logs/steps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildStepLogPath(tt.identifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPathWithOptionalQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		basePath string
		query    string
		expected string
	}{
		{
			name:     "with query",
			basePath: "/events/dag-runs",
			query:    "limit=50&offset=0",
			expected: "/events/dag-runs?limit=50&offset=0",
		},
		{
			name:     "without query",
			basePath: "/events/dag-runs",
			query:    "",
			expected: "/events/dag-runs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := pathWithOptionalQuery(tt.basePath, tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyNodeAuth(t *testing.T) {
	t.Parallel()
	t.Run("basic auth", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		node := config.RemoteNode{
			IsBasicAuth:       true,
			BasicAuthUsername: "user",
			BasicAuthPassword: "pass",
		}

		applyNodeAuth(req, node)

		user, pass, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)
	})

	t.Run("bearer token", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		node := config.RemoteNode{
			IsAuthToken: true,
			AuthToken:   "my-token-123",
		}

		applyNodeAuth(req, node)

		assert.Equal(t, "Bearer my-token-123", req.Header.Get("Authorization"))
	})

	t.Run("no auth", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		node := config.RemoteNode{}

		applyNodeAuth(req, node)

		assert.Empty(t, req.Header.Get("Authorization"))
	})
}

func TestStreamResponse(t *testing.T) {
	t.Parallel()
	t.Run("streams data correctly", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		body := io.NopCloser(bytes.NewReader([]byte("event: data\ndata: test\n\n")))

		streamResponse(w, w, body)

		assert.Equal(t, "event: data\ndata: test\n\n", w.Body.String())
		assert.GreaterOrEqual(t, w.flushCount, 1)
	})

	t.Run("handles large data in chunks", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		// Create data larger than buffer (4096 bytes)
		largeData := make([]byte, 10000)
		for i := range largeData {
			largeData[i] = 'x'
		}
		body := io.NopCloser(bytes.NewReader(largeData))

		streamResponse(w, w, body)

		assert.Equal(t, string(largeData), w.Body.String())
	})

	t.Run("handles empty body", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		body := io.NopCloser(bytes.NewReader([]byte{}))

		streamResponse(w, w, body)

		assert.Empty(t, w.Body.String())
	})
}

func TestProxyToRemoteNode(t *testing.T) {
	t.Parallel()
	t.Run("unknown remote node", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		handler := NewHandler(hub, map[string]config.RemoteNode{}, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/test", nil)

		handler.proxyToRemoteNode(w, r, "unknown-node", "dagrun:test")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "unknown remote node")
	})

	t.Run("successful proxy", func(t *testing.T) {
		t.Parallel()
		// Create a mock remote server
		remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request path
			assert.Contains(t, r.URL.Path, "/events/dag-runs/mydag/run123")

			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: data\ndata: test\n\n"))
		}))
		defer remoteServer.Close()

		hub := NewHub()
		remoteNodes := map[string]config.RemoteNode{
			"remote1": {
				Name:       "remote1",
				APIBaseURL: remoteServer.URL,
			},
		}
		handler := NewHandler(hub, remoteNodes, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/mydag/run123?token=abc", nil)

		handler.proxyToRemoteNode(w, r, "remote1", "dagrun:mydag/run123")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "event: data")
	})

	t.Run("remote server error", func(t *testing.T) {
		t.Parallel()
		remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer remoteServer.Close()

		hub := NewHub()
		remoteNodes := map[string]config.RemoteNode{
			"remote1": {
				Name:       "remote1",
				APIBaseURL: remoteServer.URL,
			},
		}
		handler := NewHandler(hub, remoteNodes, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/test", nil)

		handler.proxyToRemoteNode(w, r, "remote1", "dagrun:test")

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("connection failure", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		remoteNodes := map[string]config.RemoteNode{
			"remote1": {
				Name:       "remote1",
				APIBaseURL: "http://invalid-host:99999",
			},
		}
		handler := NewHandler(hub, remoteNodes, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/test", nil)

		handler.proxyToRemoteNode(w, r, "remote1", "dagrun:test")

		assert.Equal(t, http.StatusBadGateway, w.Code)
	})

	t.Run("with basic auth", func(t *testing.T) {
		t.Parallel()
		var receivedAuth string
		remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: ok\n\n"))
		}))
		defer remoteServer.Close()

		hub := NewHub()
		remoteNodes := map[string]config.RemoteNode{
			"remote1": {
				Name:              "remote1",
				APIBaseURL:        remoteServer.URL,
				IsBasicAuth:       true,
				BasicAuthUsername: "testuser",
				BasicAuthPassword: "testpass",
			},
		}
		handler := NewHandler(hub, remoteNodes, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/test", nil)

		handler.proxyToRemoteNode(w, r, "remote1", "dagrun:test")

		assert.NotEmpty(t, receivedAuth)
		assert.Contains(t, receivedAuth, "Basic")
	})
}

func TestStreamResponseWriteError(t *testing.T) {
	t.Parallel()
	// Create a writer that fails after writing some data
	failingWriter := &failingResponseWriter{
		failAfter: 100,
		buf:       bytes.Buffer{},
	}

	// Large body that will trigger the write error
	body := io.NopCloser(bytes.NewReader(make([]byte, 1000)))

	// Should not panic
	assert.NotPanics(t, func() {
		streamResponse(failingWriter, failingWriter, body)
	})
}

// failingResponseWriter simulates a writer that fails after a certain number of bytes
type failingResponseWriter struct {
	failAfter int
	written   int
	buf       bytes.Buffer
}

func (f *failingResponseWriter) Header() http.Header { return http.Header{} }
func (f *failingResponseWriter) WriteHeader(_ int)   {}
func (f *failingResponseWriter) Flush()              {}
func (f *failingResponseWriter) Write(b []byte) (int, error) {
	f.written += len(b)
	if f.written > f.failAfter {
		return 0, io.ErrClosedPipe
	}
	return f.buf.Write(b)
}

func TestProxyToRemoteNodeNonFlusher(t *testing.T) {
	t.Parallel()
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: test\n\n"))
	}))
	defer remoteServer.Close()

	hub := NewHub()
	remoteNodes := map[string]config.RemoteNode{
		"remote1": {
			Name:       "remote1",
			APIBaseURL: remoteServer.URL,
		},
	}
	handler := NewHandler(hub, remoteNodes, nil)

	// Use nonFlusher which doesn't implement http.Flusher
	w := &nonFlusher{}
	r := httptest.NewRequest(http.MethodGet, "/events/dag-runs/test", nil)

	handler.proxyToRemoteNode(w, r, "remote1", "dagrun:test")

	// Should return error since writer doesn't support flushing
	// The error message should indicate streaming not supported
	require.Contains(t, w.String(), "streaming not supported")
}
