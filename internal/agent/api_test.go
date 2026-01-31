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

// =============================================================================
// Unit Tests for API Constructor and Middleware
// =============================================================================

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

		req := httptest.NewRequest("GET", "/test", nil)
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

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// =============================================================================
// HTTP Handler Tests using httptest
// =============================================================================

func TestAPI_HandleNewConversation(t *testing.T) {
	t.Parallel()

	t.Run("creates conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ConversationID)
		assert.Equal(t, "accepted", resp.Status)
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: ""})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(false)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("provider error returns service unavailable", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = nil // No provider set

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("with model override", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{
			Message: "hello",
			Model:   "custom-model",
		})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

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
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		// Verify conversation was persisted
		var resp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

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

		configStore := newMockConfigStore(true)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("GET", "/api/v2/agent/conversations", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var conversations []ConversationWithState
		err := json.Unmarshal(rec.Body.Bytes(), &conversations)
		require.NoError(t, err)
		assert.Empty(t, conversations)
	})

	t.Run("returns active conversations", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		// List conversations
		req = httptest.NewRequest("GET", "/api/v2/agent/conversations", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var conversations []ConversationWithState
		err := json.Unmarshal(rec.Body.Bytes(), &conversations)
		require.NoError(t, err)
		assert.Len(t, conversations, 1)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(false)
		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("GET", "/api/v2/agent/conversations", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAPI_HandleCancel(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/non-existent/cancel", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cancels active conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Cancel the conversation
		req = httptest.NewRequest("POST", "/api/v2/agent/conversations/"+createResp.ConversationID+"/cancel", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var cancelResp map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &cancelResp)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", cancelResp["status"])
	})
}

func TestAPI_HandleGetConversation(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("GET", "/api/v2/agent/conversations/non-existent", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns active conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Get the conversation
		req = httptest.NewRequest("GET", "/api/v2/agent/conversations/"+createResp.ConversationID, nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var getResp StreamResponse
		err = json.Unmarshal(rec.Body.Bytes(), &getResp)
		require.NoError(t, err)
		assert.NotNil(t, getResp.ConversationState)
		assert.Equal(t, createResp.ConversationID, getResp.ConversationState.ConversationID)
	})
}

func TestAPI_HandleChat(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/non-existent/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("sends message to existing conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Send follow-up message
		body, _ = json.Marshal(ChatRequest{Message: "follow up"})
		req = httptest.NewRequest("POST", "/api/v2/agent/conversations/"+createResp.ConversationID+"/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var chatResp map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &chatResp)
		require.NoError(t, err)
		assert.Equal(t, "accepted", chatResp["status"])
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Send empty message
		body, _ = json.Marshal(ChatRequest{Message: ""})
		req = httptest.NewRequest("POST", "/api/v2/agent/conversations/"+createResp.ConversationID+"/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Send invalid JSON
		req = httptest.NewRequest("POST", "/api/v2/agent/conversations/"+createResp.ConversationID+"/chat", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAPI_HandleStream(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)

		api := NewAPI(APIConfig{ConfigStore: configStore})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		req := httptest.NewRequest("GET", "/api/v2/agent/conversations/non-existent/stream", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns SSE headers for active conversation", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.provider = &mockLLMProvider{}

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			WorkingDir:  t.TempDir(),
		})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create a conversation first
		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest("POST", "/api/v2/agent/conversations/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var createResp NewConversationResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)

		// Stream the conversation with a context that will be canceled
		ctx, cancel := context.WithCancel(context.Background())
		req = httptest.NewRequest("GET", "/api/v2/agent/conversations/"+createResp.ConversationID+"/stream", nil)
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()

		// Run in goroutine since streaming blocks
		done := make(chan struct{})
		go func() {
			defer close(done)
			r.ServeHTTP(rec, req)
		}()

		// Give some time for the response to start, then cancel
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for the handler to finish
		<-done

		// Verify SSE content type
		assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	})
}

// =============================================================================
// Pure Function Tests (No HTTP, No Dependencies)
// =============================================================================

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
