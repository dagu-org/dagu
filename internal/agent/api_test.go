package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
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

	var modelStore ModelStore
	var model *ModelConfig
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

// createSession creates a new session using the exported CreateSession method.
func (s *apiTestSetup) createSession(t *testing.T, message string) string {
	t.Helper()
	sessID, err := s.api.CreateSession(context.Background(), defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: message})
	require.NoError(t, err)
	require.NotEmpty(t, sessID)
	return sessID
}

// get sends a GET request and returns the recorder.
func (s *apiTestSetup) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
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

func TestAPI_CreateSession(t *testing.T) {
	t.Parallel()

	t.Run("creates session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID, err := setup.api.CreateSession(context.Background(), "admin", "admin", auth.RoleAdmin, "", ChatRequest{Message: "hello"})

		require.NoError(t, err)
		assert.NotEmpty(t, sessID)
	})

	t.Run("empty message returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		_, err := setup.api.CreateSession(context.Background(), "admin", "admin", auth.RoleAdmin, "", ChatRequest{Message: ""})

		assert.ErrorIs(t, err, ErrMessageRequired)
	})

	t.Run("provider error returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		_, err := setup.api.CreateSession(context.Background(), "admin", "admin", auth.RoleAdmin, "", ChatRequest{Message: "hello"})

		assert.ErrorIs(t, err, ErrAgentNotConfigured)
	})

	t.Run("with model override", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID, err := setup.api.CreateSession(context.Background(), "admin", "admin", auth.RoleAdmin, "", ChatRequest{
			Message: "hello",
			Model:   "test-model",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, sessID)
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

		sessID, err := api.CreateSession(context.Background(), "admin", "admin", auth.RoleAdmin, "", ChatRequest{Message: "hello"})

		require.NoError(t, err)
		assert.True(t, sessStore.HasSession(sessID), "session should be persisted")
	})
}

func TestAPI_ListSessions(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		sessions := setup.api.ListSessions(context.Background(), defaultUserID)

		assert.Empty(t, sessions)
	})

	t.Run("returns active sessions", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		setup.createSession(t, "hello")

		sessions := setup.api.ListSessions(context.Background(), defaultUserID)

		assert.Len(t, sessions, 1)
	})
}

func TestAPI_CancelSession(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		err := setup.api.CancelSession(context.Background(), "non-existent", defaultUserID)

		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("cancels active session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.CancelSession(context.Background(), sessID, defaultUserID)

		assert.NoError(t, err)
	})
}

func TestAPI_GetSessionDetail(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		_, err := setup.api.GetSessionDetail(context.Background(), "non-existent", defaultUserID)

		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("returns active session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		resp, err := setup.api.GetSessionDetail(context.Background(), sessID, defaultUserID)

		require.NoError(t, err)
		assert.NotNil(t, resp.SessionState)
		assert.Equal(t, sessID, resp.SessionState.SessionID)
	})
}

func TestAPI_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		err := setup.api.SendMessage(context.Background(), "non-existent", defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: "hello"})

		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("sends message to existing session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.SendMessage(context.Background(), sessID, defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: "follow up"})

		assert.NoError(t, err)
	})

	t.Run("empty message returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.SendMessage(context.Background(), sessID, defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: ""})

		assert.ErrorIs(t, err, ErrMessageRequired)
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

func TestAPI_CreateSession_PassesPricing(t *testing.T) {
	t.Parallel()

	t.Run("session manager receives pricing from model config", func(t *testing.T) {
		t.Parallel()

		model := testModelConfig("priced-model")
		model.InputCostPer1M = 3.0
		model.OutputCostPer1M = 15.0

		api, _ := testAPIWithModels(t, model)
		api.providers.Set(model.ToLLMConfig(), newStopProvider("hello"))

		sessID, err := api.CreateSession(context.Background(), defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: "hello"})
		require.NoError(t, err)

		mgrVal, ok := api.sessions.Load(sessID)
		require.True(t, ok)
		mgr := mgrVal.(*SessionManager)

		usage := &llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}
		cost := mgr.calculateCost(usage)
		assert.InDelta(t, 3.0, cost, 1e-9)
	})
}

func TestAPI_SendMessage_UpdatesPricing(t *testing.T) {
	t.Parallel()

	t.Run("SendMessage updates pricing from new model", func(t *testing.T) {
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

		// Create session with model-a
		sessID, err := api.CreateSession(context.Background(), defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: "hello", Model: "model-a"})
		require.NoError(t, err)

		// Send message with model-b
		err = api.SendMessage(context.Background(), sessID, defaultUserID, defaultUserID, defaultUserRole, "", ChatRequest{Message: "followup", Model: "model-b"})
		require.NoError(t, err)

		mgrVal, ok := api.sessions.Load(sessID)
		require.True(t, ok)
		mgr := mgrVal.(*SessionManager)

		usage := &llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}
		cost := mgr.calculateCost(usage)
		assert.InDelta(t, 5.0, cost, 1e-9, "pricing should be updated to model-b's input cost")
	})
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

func TestAPI_SubmitUserResponse(t *testing.T) {
	t.Parallel()

	t.Run("not found for non-existent session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		err := setup.api.SubmitUserResponse(context.Background(), "non-existent", defaultUserID, UserPromptResponse{
			PromptID:         "some-prompt",
			FreeTextResponse: "yes",
		})

		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("missing prompt_id returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.SubmitUserResponse(context.Background(), sessID, defaultUserID, UserPromptResponse{
			FreeTextResponse: "yes",
		})

		assert.ErrorIs(t, err, ErrPromptIDRequired)
	})
}

func TestAPI_ListTools(t *testing.T) {
	t.Parallel()

	setup := newAPITestSetup(t, true, false, "")
	tools := setup.api.ListTools()

	assert.NotEmpty(t, tools)
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Label)
		assert.NotEmpty(t, tool.Description)
	}
}
