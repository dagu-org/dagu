package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/dagu-org/dagu/internal/llm"
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

	var modelStore iface.ModelStore
	var model *iface.ModelConfig
	if withProvider {
		model = testModelConfig("test-model")
		ms := newMockModelStore().addModel(model)
		configStore.config.DefaultModelID = model.ID
		modelStore = ms
	}

	if workingDir == "" {
		workingDir = t.TempDir()
	}

	api := NewAPI(APIConfig{
		ConfigStore: configStore,
		ModelStore:  modelStore,
		WorkingDir:  workingDir,
	})

	if withProvider {
		api.providers.Set(model.ToLLMConfig(), &mockLLMProvider{})
	}

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

// createSession creates a new session and returns its ID.
func (s *apiTestSetup) createSession(t *testing.T, message string) string {
	t.Helper()
	rec := s.postJSON("/api/v1/agent/sessions/new", ChatRequest{Message: message})
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp NewSessionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.SessionID
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

func TestAPI_HandleNewSession(t *testing.T) {
	t.Parallel()

	t.Run("creates session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v1/agent/sessions/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp NewSessionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.SessionID)
		assert.Equal(t, "accepted", resp.Status)
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v1/agent/sessions/new", ChatRequest{Message: ""})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, false, false, "")
		rec := setup.postJSON("/api/v1/agent/sessions/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/new", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("provider error returns service unavailable", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v1/agent/sessions/new", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("with model override", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v1/agent/sessions/new", ChatRequest{
			Message: "hello",
			Model:   "test-model",
		})

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("with session store persistence", func(t *testing.T) {
		t.Parallel()

		model := testModelConfig("test-model")
		configStore := newMockConfigStore(true)
		configStore.config.DefaultModelID = model.ID
		sessStore := newMockSessionStore()

		api := NewAPI(APIConfig{
			ConfigStore:  configStore,
			ModelStore:   newMockModelStore().addModel(model),
			WorkingDir:   t.TempDir(),
			SessionStore: sessStore,
		})
		api.providers.Set(model.ToLLMConfig(), &mockLLMProvider{})

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp NewSessionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

		assert.True(t, sessStore.HasSession(resp.SessionID), "session should be persisted")
	})
}

func TestAPI_HandleListSessions(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v1/agent/sessions")

		assert.Equal(t, http.StatusOK, rec.Code)

		var sessions []SessionWithState
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sessions))
		assert.Empty(t, sessions)
	})

	t.Run("returns active sessions", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		setup.createSession(t, "hello")

		rec := setup.get("/api/v1/agent/sessions")

		assert.Equal(t, http.StatusOK, rec.Code)

		var sessions []SessionWithState
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sessions))
		assert.Len(t, sessions, 1)
	})

	t.Run("agent disabled returns not found", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, false, false, "")
		rec := setup.get("/api/v1/agent/sessions")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAPI_HandleCancel(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v1/agent/sessions/non-existent/cancel", nil)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cancels active session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		rec := setup.postJSON("/api/v1/agent/sessions/"+sessID+"/cancel", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var cancelResp map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cancelResp))
		assert.Equal(t, "cancelled", cancelResp["status"])
	})
}

func TestAPI_HandleGetSession(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v1/agent/sessions/non-existent")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns active session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		rec := setup.get("/api/v1/agent/sessions/" + sessID)

		assert.Equal(t, http.StatusOK, rec.Code)

		var getResp StreamResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &getResp))
		assert.NotNil(t, getResp.SessionState)
		assert.Equal(t, sessID, getResp.SessionState.SessionID)
	})
}

func TestAPI_HandleChat(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.postJSON("/api/v1/agent/sessions/non-existent/chat", ChatRequest{Message: "hello"})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("sends message to existing session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		rec := setup.postJSON("/api/v1/agent/sessions/"+sessID+"/chat", ChatRequest{Message: "follow up"})

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var chatResp map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &chatResp))
		assert.Equal(t, "accepted", chatResp["status"])
	})

	t.Run("empty message returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		rec := setup.postJSON("/api/v1/agent/sessions/"+sessID+"/chat", ChatRequest{Message: ""})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid JSON returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/"+sessID+"/chat", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAPI_HandleStream(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		rec := setup.get("/api/v1/agent/sessions/non-existent/stream")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns SSE headers for active session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/sessions/"+sessID+"/stream", nil)
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
		sess     string
		config   string
		expected string
	}{
		{
			name:     "request model takes priority",
			request:  "req-model",
			sess:     "sess-model",
			config:   "cfg-model",
			expected: "req-model",
		},
		{
			name:     "session model when no request model",
			request:  "",
			sess:     "sess-model",
			config:   "cfg-model",
			expected: "sess-model",
		},
		{
			name:     "config model when no request or session model",
			request:  "",
			sess:     "",
			config:   "cfg-model",
			expected: "cfg-model",
		},
		{
			name:     "empty when all empty",
			request:  "",
			sess:     "",
			config:   "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := selectModel(tc.request, tc.sess, tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
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

func TestAPI_ResolveProvider(t *testing.T) {
	t.Parallel()

	t.Run("model found returns provider and config", func(t *testing.T) {
		t.Parallel()

		model := testModelConfig("my-model")
		api, _ := testAPIWithModels(t, model)

		provider, modelCfg, err := api.resolveProvider(context.Background(), "my-model")

		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "my-model", modelCfg.ID)
		assert.Equal(t, "gpt-4.1", modelCfg.Model)
	})

	t.Run("empty model ID uses default", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPIWithModels(t, testModelConfig("default-model"))

		provider, modelCfg, err := api.resolveProvider(context.Background(), "")

		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "default-model", modelCfg.ID)
	})

	t.Run("model not found falls back to default", func(t *testing.T) {
		t.Parallel()

		api, _ := testAPIWithModels(t, testModelConfig("default-model"))

		provider, modelCfg, err := api.resolveProvider(context.Background(), "deleted-model")

		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "default-model", modelCfg.ID)
	})

	t.Run("both model and default not found returns error", func(t *testing.T) {
		t.Parallel()

		configStore := newMockConfigStore(true)
		configStore.config.DefaultModelID = "also-missing"

		api := NewAPI(APIConfig{
			ConfigStore: configStore,
			ModelStore:  newMockModelStore(),
			WorkingDir:  t.TempDir(),
		})

		_, _, err := api.resolveProvider(context.Background(), "missing-model")
		require.Error(t, err)
	})

	t.Run("nil model store returns error", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{
			ConfigStore: newMockConfigStore(true),
			WorkingDir:  t.TempDir(),
		})

		_, _, err := api.resolveProvider(context.Background(), "any")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model store not configured")
	})

	t.Run("no model configured returns error", func(t *testing.T) {
		t.Parallel()

		api := NewAPI(APIConfig{
			ConfigStore: newMockConfigStore(true),
			ModelStore:  newMockModelStore(),
			WorkingDir:  t.TempDir(),
		})

		_, _, err := api.resolveProvider(context.Background(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no model configured")
	})
}

func TestAPI_HandleNewSession_PassesPricing(t *testing.T) {
	t.Parallel()

	t.Run("session manager receives pricing from model config", func(t *testing.T) {
		t.Parallel()

		model := testModelConfig("priced-model")
		model.InputCostPer1M = 3.0
		model.OutputCostPer1M = 15.0

		api, _ := testAPIWithModels(t, model)
		api.providers.Set(model.ToLLMConfig(), newStopProvider("hello"))

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		body, _ := json.Marshal(ChatRequest{Message: "hello"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp NewSessionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

		mgrVal, ok := api.sessions.Load(resp.SessionID)
		require.True(t, ok)
		mgr := mgrVal.(*SessionManager)

		usage := &llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}
		cost := mgr.calculateCost(usage)
		assert.InDelta(t, 3.0, cost, 1e-9)
	})
}

func TestAPI_HandleChat_UpdatesPricing(t *testing.T) {
	t.Parallel()

	t.Run("handleChat updates pricing from new model", func(t *testing.T) {
		t.Parallel()

		modelA := testModelConfig("model-a")
		modelA.InputCostPer1M = 3.0
		modelA.OutputCostPer1M = 15.0

		modelB := testModelConfig("model-b")
		modelB.Model = "gpt-5"
		modelB.InputCostPer1M = 5.0
		modelB.OutputCostPer1M = 25.0

		api, _ := testAPIWithModels(t, modelA, modelB)
		api.providers.Set(modelA.ToLLMConfig(), newStopProvider("a"))
		api.providers.Set(modelB.ToLLMConfig(), newStopProvider("b"))

		r := chi.NewRouter()
		api.RegisterRoutes(r, nil)

		// Create session with model-a
		body, _ := json.Marshal(ChatRequest{Message: "hello", Model: "model-a"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/new", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var resp NewSessionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		sessID := resp.SessionID

		// Send chat with model-b
		body, _ = json.Marshal(ChatRequest{Message: "followup", Model: "model-b"})
		req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/"+sessID+"/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusAccepted, rec.Code)

		mgrVal, ok := api.sessions.Load(sessID)
		require.True(t, ok)
		mgr := mgrVal.(*SessionManager)

		usage := &llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}
		cost := mgr.calculateCost(usage)
		assert.InDelta(t, 5.0, cost, 1e-9, "pricing should be updated to model-b's input cost")
	})
}

func TestAPI_RequestBodySizeLimit(t *testing.T) {
	t.Parallel()

	oversizedBody := bytes.Repeat([]byte("x"), maxRequestBodySize+1)

	endpoints := []struct {
		name       string
		pathSuffix string
		needsSess  bool
	}{
		{"handleNewSession", "/new", false},
		{"handleChat", "/chat", true},
		{"handleUserResponse", "/respond", true},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+" rejects oversized body", func(t *testing.T) {
			t.Parallel()

			setup := newAPITestSetup(t, true, true, "")
			path := "/api/v1/agent/sessions"
			if ep.needsSess {
				sessID := setup.createSession(t, "hello")
				path += "/" + sessID
			}
			path += ep.pathSuffix

			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(oversizedBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			setup.router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAPI_CleanupIdleSessions(t *testing.T) {
	t.Parallel()

	newTestAPI := func(t *testing.T) *API {
		t.Helper()
		return NewAPI(APIConfig{
			ConfigStore: newMockConfigStore(true),
			WorkingDir:  t.TempDir(),
		})
	}

	newIdleMgr := func(id string, working bool) *SessionManager {
		mgr := NewSessionManager(SessionManagerConfig{ID: id})
		mgr.mu.Lock()
		mgr.lastActivity = time.Now().Add(-1 * time.Hour)
		mgr.working = working
		mgr.mu.Unlock()
		return mgr
	}

	t.Run("removes idle non-working sessions", func(t *testing.T) {
		t.Parallel()

		api := newTestAPI(t)
		api.sessions.Store("idle-sess", newIdleMgr("idle-sess", false))
		api.sessions.Store("active-sess", NewSessionManager(SessionManagerConfig{ID: "active-sess"}))

		api.cleanupIdleSessions()

		_, idleExists := api.sessions.Load("idle-sess")
		_, activeExists := api.sessions.Load("active-sess")

		assert.False(t, idleExists, "idle session should be removed")
		assert.True(t, activeExists, "active session should remain")
	})

	t.Run("does not remove working sessions even if idle", func(t *testing.T) {
		t.Parallel()

		api := newTestAPI(t)
		api.sessions.Store("working-sess", newIdleMgr("working-sess", true))

		api.cleanupIdleSessions()

		_, exists := api.sessions.Load("working-sess")
		assert.True(t, exists, "working session should not be removed even if idle")
	})

	t.Run("does nothing with empty sessions", func(t *testing.T) {
		t.Parallel()

		api := newTestAPI(t)
		api.cleanupIdleSessions()
	})
}

func TestAPI_HandleUserResponse(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		rec := setup.postJSON("/api/v1/agent/sessions/non-existent/respond", UserPromptResponse{
			PromptID:         "some-prompt",
			FreeTextResponse: "yes",
		})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("missing prompt_id returns bad request", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		rec := setup.postJSON("/api/v1/agent/sessions/"+sessID+"/respond", UserPromptResponse{
			FreeTextResponse: "yes",
		})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
