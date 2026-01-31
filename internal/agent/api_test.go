package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiTestSetup contains the common test infrastructure for API tests.
type apiTestSetup struct {
	api         *API
	router      chi.Router
	configStore *mockConfigStore
}

// newAPITestSetup creates a new API test setup with the given options.
func newAPITestSetup(t *testing.T, enabled bool, withProvider bool, workingDir string) *apiTestSetup {
	t.Helper()

	configStore := newMockConfigStore(enabled)
	if withProvider {
		configStore.provider = &mockLLMProvider{}
	}

	if workingDir == "" {
		workingDir = t.TempDir()
	}

	api := NewAPI(APIConfig{
		ConfigStore: configStore,
		WorkingDir:  workingDir,
	})

	r := chi.NewRouter()
	api.RegisterRoutes(r, nil)

	return &apiTestSetup{
		api:         api,
		router:      r,
		configStore: configStore,
	}
}

// postJSON sends a POST request with JSON body and returns the recorder.
func (s *apiTestSetup) postJSON(path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

// get sends a GET request and returns the recorder.
func (s *apiTestSetup) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

// createConversation creates a new conversation and returns its ID.
func (s *apiTestSetup) createConversation(t *testing.T, message string) string {
	t.Helper()
	rec := s.postJSON("/api/v2/agent/conversations/new", ChatRequest{Message: message})
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp NewConversationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.ConversationID
}

func TestNewAPI(t *testing.T) {
	t.Parallel()

	t.Run("creates API with config", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{
			ConfigStore: newMockConfigStore(true),
			WorkingDir:  "/test",
		})

		assert.NotNil(t, api)
	})

	t.Run("uses default logger", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{
			ConfigStore: newMockConfigStore(true),
		})

		assert.NotNil(t, api)
	})
}

func TestAPI_EnabledMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("passes through when enabled", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{ConfigStore: newMockConfigStore(true)})
		middleware := api.enabledMiddleware()

		called := false
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("blocks when disabled", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{ConfigStore: newMockConfigStore(false)})
		middleware := api.enabledMiddleware()

		handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAPI_HandleNewConversation(t *testing.T) {
	t.Parallel()

	t.Run("creates conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v2/agent/conversations/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp NewConversationResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.ConversationID)
		assert.Equal(t, "accepted", resp.Status)
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v2/agent/conversations/new", ChatRequest{Message: ""})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, false, false, "")
		rec := setup.postJSON("/api/v2/agent/conversations/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")

		req := httptest.NewRequest(http.MethodPost, "/api/v2/agent/conversations/new", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("provider error returns service unavailable", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v2/agent/conversations/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("with model override", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v2/agent/conversations/new", ChatRequest{
			Message: "hello",
			Model:   "custom-model",
		})

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("with conversation store persistence", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}
		convStore := newMockConversationStore()

		api := NewAPI(APIConfig{
			ConfigStore:       configStore,
			WorkingDir:        t.TempDir(),
			ConversationStore: convStore,
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest(http.MethodPost, "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp NewConversationResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

		convStore.mu.Lock()
		_, exists := convStore.conversations[resp.ConversationID]
		convStore.mu.Unlock()
		assert.True(t, exists, "conversation should be persisted")
	})
}

func TestAPI_HandleListConversations(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v2/agent/conversations")

		assert.Equal(t, http.StatusOK, rec.Code)

		var conversations []ConversationWithState
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &conversations))
		assert.Empty(t, conversations)
	})

	t.Run("returns active conversations", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		setup.createConversation(t, "hello")

		rec := setup.get("/api/v2/agent/conversations")

		assert.Equal(t, http.StatusOK, rec.Code)

		var conversations []ConversationWithState
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &conversations))
		assert.Len(t, conversations, 1)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, false, false, "")
		rec := setup.get("/api/v2/agent/conversations")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAPI_HandleCancel(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v2/agent/conversations/non-existent/cancel", nil)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cancels active conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		rec := setup.postJSON("/api/v2/agent/conversations/"+convID+"/cancel", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var cancelResp map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cancelResp))
		assert.Equal(t, "cancelled", cancelResp["status"])
	})
}

func TestAPI_HandleGetConversation(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v2/agent/conversations/non-existent")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns active conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		rec := setup.get("/api/v2/agent/conversations/" + convID)

		assert.Equal(t, http.StatusOK, rec.Code)

		var getResp StreamResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &getResp))
		assert.NotNil(t, getResp.ConversationState)
		assert.Equal(t, convID, getResp.ConversationState.ConversationID)
	})
}

func TestAPI_HandleChat(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v2/agent/conversations/non-existent/chat", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("sends message to existing conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		rec := setup.postJSON("/api/v2/agent/conversations/"+convID+"/chat", ChatRequest{Message: "follow up"})

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var chatResp map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &chatResp))
		assert.Equal(t, "accepted", chatResp["status"])
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		rec := setup.postJSON("/api/v2/agent/conversations/"+convID+"/chat", ChatRequest{Message: ""})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		req := httptest.NewRequest(http.MethodPost, "/api/v2/agent/conversations/"+convID+"/chat", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAPI_HandleStream(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v2/agent/conversations/non-existent/stream")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns SSE headers for active conversation", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		convID := setup.createConversation(t, "hello")

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/v2/agent/conversations/"+convID+"/stream", nil)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			defer close(done)
			setup.router.ServeHTTP(rec, req)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done

		assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	})
}

func TestFormatMessageWithContexts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		message  string
		contexts []ResolvedDAGContext
		contains []string
	}{
		{
			name:     "no contexts",
			message:  "hello",
			contexts: nil,
			contains: []string{"hello"},
		},
		{
			name:    "with single context",
			message: "explain this dag",
			contexts: []ResolvedDAGContext{
				{DAGFilePath: "/path/to/dag.yaml", DAGName: "my-dag"},
			},
			contains: []string{"Referenced DAGs", "my-dag", "/path/to/dag.yaml", "explain this dag"},
		},
		{
			name:    "with run ID",
			message: "show run status",
			contexts: []ResolvedDAGContext{
				{DAGFilePath: "/dag.yaml", DAGName: "test", DAGRunID: "run-123"},
			},
			contains: []string{"run: run-123"},
		},
		{
			name:    "with run status",
			message: "check status",
			contexts: []ResolvedDAGContext{
				{DAGFilePath: "/dag.yaml", DAGName: "test", RunStatus: "success"},
			},
			contains: []string{"status: success"},
		},
		{
			name:    "with multiple contexts",
			message: "compare these dags",
			contexts: []ResolvedDAGContext{
				{DAGFilePath: "/dag1.yaml", DAGName: "dag-1"},
				{DAGFilePath: "/dag2.yaml", DAGName: "dag-2"},
			},
			contains: []string{"dag-1", "dag-2", "/dag1.yaml", "/dag2.yaml"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := formatMessageWithContexts(tc.message, tc.contexts)
			for _, expected := range tc.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatContextLine(t *testing.T) {
	t.Parallel()

	t.Run("formats with file path", func(t *testing.T) {
		t.Parallel()

		ctx := ResolvedDAGContext{
			DAGFilePath: "/path/to/dag.yaml",
			DAGName:     "my-dag",
		}

		result := formatContextLine(ctx)
		assert.Contains(t, result, "my-dag")
		assert.Contains(t, result, "/path/to/dag.yaml")
	})

	t.Run("uses unknown for empty name", func(t *testing.T) {
		t.Parallel()

		ctx := ResolvedDAGContext{
			DAGFilePath: "/path/to/dag.yaml",
		}

		result := formatContextLine(ctx)
		assert.Contains(t, result, "unknown")
	})

	t.Run("returns empty for empty context", func(t *testing.T) {
		t.Parallel()

		ctx := ResolvedDAGContext{}
		result := formatContextLine(ctx)
		assert.Empty(t, result)
	})

	t.Run("includes run ID when present", func(t *testing.T) {
		t.Parallel()

		ctx := ResolvedDAGContext{
			DAGFilePath: "/path/to/dag.yaml",
			DAGName:     "my-dag",
			DAGRunID:    "run-abc123",
		}

		result := formatContextLine(ctx)
		assert.Contains(t, result, "run: run-abc123")
	})

	t.Run("includes status when present", func(t *testing.T) {
		t.Parallel()

		ctx := ResolvedDAGContext{
			DAGFilePath: "/path/to/dag.yaml",
			DAGName:     "my-dag",
			RunStatus:   "failed",
		}

		result := formatContextLine(ctx)
		assert.Contains(t, result, "status: failed")
	})
}

func TestSelectModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  string
		conv     string
		config   string
		expected string
	}{
		{
			name:     "request model takes priority",
			request:  "req-model",
			conv:     "conv-model",
			config:   "cfg-model",
			expected: "req-model",
		},
		{
			name:     "conversation model when no request model",
			request:  "",
			conv:     "conv-model",
			config:   "cfg-model",
			expected: "conv-model",
		},
		{
			name:     "config model when no request or conversation model",
			request:  "",
			conv:     "",
			config:   "cfg-model",
			expected: "cfg-model",
		},
		{
			name:     "empty when all empty",
			request:  "",
			conv:     "",
			config:   "",
			expected: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := selectModel(tc.request, tc.conv, tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPtrTo(t *testing.T) {
	t.Parallel()

	t.Run("returns pointer to string", func(t *testing.T) {
		t.Parallel()

		result := ptrTo("hello")
		assert.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})

	t.Run("returns pointer to int", func(t *testing.T) {
		t.Parallel()

		result := ptrTo(42)
		assert.NotNil(t, result)
		assert.Equal(t, 42, *result)
	})

	t.Run("returns pointer to struct", func(t *testing.T) {
		t.Parallel()

		conv := Conversation{ID: "test"}
		result := ptrTo(conv)
		assert.NotNil(t, result)
		assert.Equal(t, "test", result.ID)
	})

	t.Run("returns pointer to zero value", func(t *testing.T) {
		t.Parallel()

		result := ptrTo(0)
		assert.NotNil(t, result)
		assert.Equal(t, 0, *result)
	})
}

func TestGetUserIDFromContext(t *testing.T) {
	t.Parallel()

	t.Run("returns default user ID when no user in context", func(t *testing.T) {
		t.Parallel()

		ctx := httptest.NewRequest("GET", "/", nil).Context()
		result := getUserIDFromContext(ctx)
		assert.Equal(t, defaultUserID, result)
	})
}
