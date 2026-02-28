package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

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
	sessID, _, err := s.api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: message})
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
		sessID, _, err := setup.api.CreateSession(context.Background(), UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}, ChatRequest{Message: "hello"})

		require.NoError(t, err)
		assert.NotEmpty(t, sessID)
	})

	t.Run("empty message returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		_, _, err := setup.api.CreateSession(context.Background(), UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}, ChatRequest{Message: ""})

		assert.ErrorIs(t, err, ErrMessageRequired)
	})

	t.Run("provider error returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		_, _, err := setup.api.CreateSession(context.Background(), UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}, ChatRequest{Message: "hello"})

		assert.ErrorIs(t, err, ErrAgentNotConfigured)
	})

	t.Run("with model override", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID, _, err := setup.api.CreateSession(context.Background(), UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}, ChatRequest{
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

		sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}, ChatRequest{Message: "hello"})

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

func TestAPI_ListSessionsPaginated(t *testing.T) {
	t.Parallel()

	t.Run("returns empty result", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, false, "")
		result := setup.api.ListSessionsPaginated(context.Background(), defaultUserID, 1, 10)

		assert.Empty(t, result.Items)
		assert.Equal(t, 0, result.TotalCount)
		assert.False(t, result.HasNextPage)
	})

	t.Run("paginates active sessions", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		setup.createSession(t, "hello1")
		setup.createSession(t, "hello2")
		setup.createSession(t, "hello3")

		// First page
		result := setup.api.ListSessionsPaginated(context.Background(), defaultUserID, 1, 2)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, 3, result.TotalCount)
		assert.True(t, result.HasNextPage)

		// Second page
		result = setup.api.ListSessionsPaginated(context.Background(), defaultUserID, 2, 2)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, 3, result.TotalCount)
		assert.False(t, result.HasNextPage)
	})

	t.Run("merges active and persisted sessions", func(t *testing.T) {
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

		// Create an active session via API
		sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "active"})
		require.NoError(t, err)

		// Add a persisted-only session directly to the store
		persistedSess := &Session{
			ID:        "persisted-1",
			UserID:    defaultUserID,
			CreatedAt: time.Now().Add(-time.Hour),
			UpdatedAt: time.Now().Add(-time.Hour),
		}
		require.NoError(t, sessStore.CreateSession(context.Background(), persistedSess))

		result := api.ListSessionsPaginated(context.Background(), defaultUserID, 1, 10)

		assert.Equal(t, 2, result.TotalCount)
		// Verify both sessions present
		ids := make(map[string]bool)
		for _, s := range result.Items {
			ids[s.Session.ID] = true
		}
		assert.True(t, ids[sessID], "active session should be present")
		assert.True(t, ids["persisted-1"], "persisted session should be present")
	})

	t.Run("excludes sub-sessions", func(t *testing.T) {
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

		// Create a parent session
		_, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "parent"})
		require.NoError(t, err)

		// Add a sub-session directly to the store
		subSess := &Session{
			ID:              "sub-1",
			UserID:          defaultUserID,
			ParentSessionID: "some-parent",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		require.NoError(t, sessStore.CreateSession(context.Background(), subSess))

		result := api.ListSessionsPaginated(context.Background(), defaultUserID, 1, 10)

		// Sub-session should be excluded
		assert.Equal(t, 1, result.TotalCount)
		for _, s := range result.Items {
			assert.Empty(t, s.Session.ParentSessionID, "sub-sessions should be excluded")
		}
	})

	t.Run("no store falls back to active only", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		setup.createSession(t, "hello")

		result := setup.api.ListSessionsPaginated(context.Background(), defaultUserID, 1, 10)

		assert.Len(t, result.Items, 1)
		assert.Equal(t, 1, result.TotalCount)
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
		err := setup.api.SendMessage(context.Background(), "non-existent", UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "hello"})

		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("sends message to existing session", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.SendMessage(context.Background(), sessID, UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "follow up"})

		assert.NoError(t, err)
	})

	t.Run("empty message returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		sessID := setup.createSession(t, "hello")

		err := setup.api.SendMessage(context.Background(), sessID, UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: ""})

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

		sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "hello"})
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
		sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "hello", Model: "model-a"})
		require.NoError(t, err)

		// Send message with model-b
		err = api.SendMessage(context.Background(), sessID, UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "followup", Model: "model-b"})
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

// createAPIWithSessionStore creates an API with a mock session store and custom provider.
func createAPIWithSessionStore(t *testing.T, provider llm.Provider) (*API, *mockSessionStore) {
	t.Helper()

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
	api.providers.Set(model.ToLLMConfig(), provider)

	return api, sessStore
}

func TestAPI_ListSessions_ExcludesSubSessions(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("hello"))

	now := time.Now()

	// Insert parent session
	parentSess := &Session{
		ID:        "parent-sess-1",
		UserID:    defaultUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.CreateSession(context.Background(), parentSess))

	// Insert sub-session (has ParentSessionID set)
	subSess := &Session{
		ID:              "sub-sess-1",
		UserID:          defaultUserID,
		ParentSessionID: "parent-sess-1",
		DelegateTask:    "analyze data",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, store.CreateSession(context.Background(), subSess))

	sessions := api.ListSessions(context.Background(), defaultUserID)

	// Should only contain the parent session, not the sub-session
	assert.Len(t, sessions, 1)
	assert.Equal(t, "parent-sess-1", sessions[0].Session.ID)
}

func TestAPI_ReactivateSession_RejectsSubSession(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("hello"))

	now := time.Now()

	// Insert a sub-session in the store
	subSess := &Session{
		ID:              "sub-sess-1",
		UserID:          defaultUserID,
		ParentSessionID: "parent-sess-1",
		DelegateTask:    "sub task",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, store.CreateSession(context.Background(), subSess))

	// SendMessage should fail because reactivateSession rejects sub-sessions
	err := api.SendMessage(context.Background(), "sub-sess-1", UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "hello"})
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestAPI_ReactivateSession_AcceptsParentSession(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("hello"))

	now := time.Now()

	// Insert a normal parent session in the store
	parentSess := &Session{
		ID:        "parent-sess-1",
		UserID:    defaultUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.CreateSession(context.Background(), parentSess))

	// SendMessage should succeed by reactivating the session
	err := api.SendMessage(context.Background(), "parent-sess-1", UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "hello"})
	assert.NoError(t, err)

	// Verify session is now active
	_, ok := api.sessions.Load("parent-sess-1")
	assert.True(t, ok, "session should be reactivated and active")
}

func TestAPI_GetSessionDetail_AllowsSubSession(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("hello"))

	now := time.Now()

	// Insert a sub-session in the store
	subSess := &Session{
		ID:              "sub-sess-1",
		UserID:          defaultUserID,
		ParentSessionID: "parent-sess-1",
		DelegateTask:    "sub task",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, store.CreateSession(context.Background(), subSess))

	// Add a message to the sub-session
	require.NoError(t, store.AddMessage(context.Background(), "sub-sess-1", &Message{
		ID:      "msg-1",
		Type:    MessageTypeAssistant,
		Content: "sub result",
	}))

	// GetSessionDetail should work for sub-sessions (read-only access is OK)
	resp, err := api.GetSessionDetail(context.Background(), "sub-sess-1", defaultUserID)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-sess-1", resp.SessionState.SessionID)
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "sub result", resp.Messages[0].Content)
}

func TestAPI_CreateSession_DelegateFlow(t *testing.T) {
	t.Parallel()

	provider := newDelegateProvider("analyze data")
	api, store := createAPIWithSessionStore(t, provider)

	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "delegate test"})
	require.NoError(t, err)
	require.NotEmpty(t, sessID)

	// Wait for the delegate flow to complete (parent loop processes delegate tool call,
	// child loop runs, parent loop gets final response)
	require.Eventually(t, func() bool {
		mgr, ok := api.sessions.Load(sessID)
		if !ok {
			return false
		}
		return !mgr.(*SessionManager).IsWorking()
	}, 10*time.Second, 100*time.Millisecond, "session should finish processing")

	// Verify sub-session was created in the store
	store.mu.Lock()
	defer store.mu.Unlock()

	var subSessions []*Session
	for _, sess := range store.sessions {
		if sess.ParentSessionID == sessID {
			subSessions = append(subSessions, sess)
		}
	}

	require.Len(t, subSessions, 1, "should have exactly one sub-session")
	assert.Equal(t, sessID, subSessions[0].ParentSessionID)
	assert.Equal(t, "analyze data", subSessions[0].DelegateTask)
	assert.Equal(t, defaultUserID, subSessions[0].UserID)
}

func TestAPI_CreateSession_ParentMessagesIntact(t *testing.T) {
	t.Parallel()

	provider := newDelegateProvider("sub task")
	api, _ := createAPIWithSessionStore(t, provider)

	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "parent msg"})
	require.NoError(t, err)

	// Wait for processing to complete
	require.Eventually(t, func() bool {
		mgr, ok := api.sessions.Load(sessID)
		if !ok {
			return false
		}
		return !mgr.(*SessionManager).IsWorking()
	}, 10*time.Second, 100*time.Millisecond)

	mgr, ok := api.sessions.Load(sessID)
	require.True(t, ok)
	msgs := mgr.(*SessionManager).GetMessages()

	// Parent messages should include:
	// 1. User message
	// 2. Assistant message with tool call
	// 3. Tool result (delegate result)
	// 4. Final assistant message
	require.GreaterOrEqual(t, len(msgs), 3, "parent should have at least user + assistant + tool result messages")

	// First message should be user
	assert.Equal(t, MessageTypeUser, msgs[0].Type)
	assert.Contains(t, msgs[0].Content, "parent msg")

	// Check that there's a delegate result message (has DelegateIDs set)
	var hasDelegateResult bool
	for _, msg := range msgs {
		if len(msg.DelegateIDs) > 0 {
			hasDelegateResult = true
			break
		}
	}
	assert.True(t, hasDelegateResult, "parent messages should contain delegate tool result with DelegateIDs")
}

func TestAPI_CreateSession_MultipleDelegates(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	callCount := 0
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			if n == 1 {
				// Return a single batched delegate tool call with 2 tasks
				return &llm.ChatResponse{
					Content:      "",
					FinishReason: "tool_calls",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call-1",
							Type: "function",
							Function: llm.ToolCallFunction{
								Name:      "delegate",
								Arguments: `{"tasks": [{"task": "task one"}, {"task": "task two"}]}`,
							},
						},
					},
				}, nil
			}
			return &llm.ChatResponse{Content: fmt.Sprintf("done-%d", n), FinishReason: "stop"}, nil
		},
	}

	api, store := createAPIWithSessionStore(t, provider)

	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "multi delegate"})
	require.NoError(t, err)

	// Wait for processing to complete
	require.Eventually(t, func() bool {
		mgr, ok := api.sessions.Load(sessID)
		if !ok {
			return false
		}
		return !mgr.(*SessionManager).IsWorking()
	}, 10*time.Second, 100*time.Millisecond)

	// Verify 2 sub-sessions were created
	store.mu.Lock()
	defer store.mu.Unlock()

	var subSessions []*Session
	for _, sess := range store.sessions {
		if sess.ParentSessionID == sessID {
			subSessions = append(subSessions, sess)
		}
	}

	assert.Len(t, subSessions, 2, "should have 2 sub-sessions from batched delegate call")

	// Verify each has a different delegate task
	tasks := map[string]bool{}
	for _, sub := range subSessions {
		tasks[sub.DelegateTask] = true
	}
	assert.Contains(t, tasks, "task one")
	assert.Contains(t, tasks, "task two")
}

func TestAPI_CreateSession_PersistsToStore(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("persisted response"))

	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "persist test"})
	require.NoError(t, err)

	// Verify session exists in store
	assert.True(t, store.HasSession(sessID), "session should be persisted to store")

	// Wait for messages to be persisted
	require.Eventually(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		msgs, exists := store.messages[sessID]
		if !exists {
			return false
		}
		// Should have at least user + assistant messages
		return len(msgs) >= 1
	}, 5*time.Second, 50*time.Millisecond)

	// Verify messages are persisted
	store.mu.Lock()
	msgs := store.messages[sessID]
	store.mu.Unlock()

	assert.GreaterOrEqual(t, len(msgs), 1, "should have persisted messages")

	// Verify session metadata
	sess, err := store.GetSession(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, defaultUserID, sess.UserID)
	assert.Empty(t, sess.ParentSessionID, "parent session should not have ParentSessionID")
}

func TestAPI_SendMessage_ReactivatesFromStore(t *testing.T) {
	t.Parallel()

	api, store := createAPIWithSessionStore(t, newStopProvider("reactivated response"))

	// Create a session
	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "initial"})
	require.NoError(t, err)

	// Wait for initial processing
	require.Eventually(t, func() bool {
		mgr, ok := api.sessions.Load(sessID)
		if !ok {
			return false
		}
		return !mgr.(*SessionManager).IsWorking()
	}, 5*time.Second, 50*time.Millisecond)

	// Remove from active sessions (simulate cleanup)
	api.sessions.Delete(sessID)

	_, stillActive := api.sessions.Load(sessID)
	require.False(t, stillActive, "session should be removed from active sessions")

	// Verify it's still in the store
	assert.True(t, store.HasSession(sessID), "session should still exist in store")

	// SendMessage should reactivate from store
	err = api.SendMessage(context.Background(), sessID, UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "follow up"})
	assert.NoError(t, err)

	// Verify session is now active again
	_, reactivated := api.sessions.Load(sessID)
	assert.True(t, reactivated, "session should be reactivated from store")
}

func TestAPI_GetSessionDetail_IncludesDelegates(t *testing.T) {
	t.Parallel()

	api, _ := createAPIWithSessionStore(t, newDelegateProvider("detail delegate"))

	sessID, _, err := api.CreateSession(context.Background(), UserIdentity{UserID: defaultUserID, Username: defaultUserID, Role: defaultUserRole}, ChatRequest{Message: "delegate detail"})
	require.NoError(t, err)

	// Wait for delegate flow to complete
	require.Eventually(t, func() bool {
		mgr, ok := api.sessions.Load(sessID)
		if !ok {
			return false
		}
		return !mgr.(*SessionManager).IsWorking()
	}, 10*time.Second, 100*time.Millisecond)

	resp, err := api.GetSessionDetail(context.Background(), sessID, defaultUserID)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The session detail should include the delegate snapshot.
	require.NotEmpty(t, resp.Delegates, "GetSessionDetail should include delegate snapshots")
	assert.Equal(t, DelegateStatusCompleted, resp.Delegates[0].Status)
	assert.Equal(t, "detail delegate", resp.Delegates[0].Task)
}

func TestAPI_CleanupIdleSessions_DeletesIdleSession(t *testing.T) {
	t.Parallel()

	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		WorkingDir:  t.TempDir(),
	})

	mgr := NewSessionManager(SessionManagerConfig{ID: "cleanup-cancel"})
	mgr.mu.Lock()
	mgr.lastActivity = time.Now().Add(-1 * time.Hour)
	mgr.working = false
	mgr.mu.Unlock()

	// Track deletion: store the mgr then verify it's removed.
	api.sessions.Store("cleanup-cancel", mgr)

	// Verify session exists before cleanup.
	_, exists := api.sessions.Load("cleanup-cancel")
	require.True(t, exists)

	api.cleanupIdleSessions()

	// Session should be removed.
	_, exists = api.sessions.Load("cleanup-cancel")
	assert.False(t, exists, "idle session should be cleaned up")
}

func TestAPI_CleanupIdleSessions_CancelsStuckSession(t *testing.T) {
	t.Parallel()

	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		WorkingDir:  t.TempDir(),
	})

	mgr := NewSessionManager(SessionManagerConfig{ID: "stuck-sess"})
	mgr.mu.Lock()
	mgr.working = true
	mgr.lastHeartbeat = time.Now().Add(-1 * time.Minute) // stale heartbeat
	mgr.lastActivity = time.Now()                        // recent activity
	mgr.mu.Unlock()

	api.sessions.Store("stuck-sess", mgr)

	api.cleanupIdleSessions()

	// Session should have been cancelled (working set to false)
	require.False(t, mgr.IsWorking(), "stuck session should be cancelled")
}

func TestAPI_CleanupIdleSessions_DoesNotCancelHealthyWorkingSession(t *testing.T) {
	t.Parallel()

	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		WorkingDir:  t.TempDir(),
	})

	mgr := NewSessionManager(SessionManagerConfig{ID: "healthy-sess"})
	mgr.mu.Lock()
	mgr.working = true
	mgr.lastHeartbeat = time.Now() // fresh heartbeat
	mgr.lastActivity = time.Now()
	mgr.mu.Unlock()

	api.sessions.Store("healthy-sess", mgr)

	api.cleanupIdleSessions()

	// Session should still be working
	_, exists := api.sessions.Load("healthy-sess")
	require.True(t, exists, "healthy working session should not be removed")
}

func TestAPI_CleanupIdleSessions_DoesNotCancelZeroHeartbeat(t *testing.T) {
	t.Parallel()

	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		WorkingDir:  t.TempDir(),
	})

	// Working session with zero heartbeat (loop hasn't started heartbeating yet)
	mgr := NewSessionManager(SessionManagerConfig{ID: "no-hb-sess"})
	mgr.mu.Lock()
	mgr.working = true
	mgr.lastActivity = time.Now()
	mgr.mu.Unlock()

	api.sessions.Store("no-hb-sess", mgr)

	api.cleanupIdleSessions()

	// Should not be cancelled because lastHeartbeat is zero
	_, exists := api.sessions.Load("no-hb-sess")
	require.True(t, exists, "session with zero heartbeat should not be cancelled")
}

func TestAPI_CreateSession_IdempotentWithSessionID(t *testing.T) {
	t.Parallel()

	t.Run("duplicate ID returns already_exists", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		user := UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}
		clientID := "550e8400-e29b-41d4-a716-446655440000"

		sessID, status, err := setup.api.CreateSession(context.Background(), user, ChatRequest{
			Message:   "hello",
			SessionID: clientID,
		})
		require.NoError(t, err)
		assert.Equal(t, clientID, sessID)
		assert.Equal(t, "accepted", status)

		// Second call with same ID should return already_exists.
		sessID2, status2, err2 := setup.api.CreateSession(context.Background(), user, ChatRequest{
			Message:   "world",
			SessionID: clientID,
		})
		require.NoError(t, err2)
		assert.Equal(t, clientID, sessID2)
		assert.Equal(t, "already_exists", status2)
	})

	t.Run("duplicate ID from different user returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		user1 := UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}
		user2 := UserIdentity{UserID: "other", Username: "other", Role: auth.RoleAdmin}
		clientID := "660e8400-e29b-41d4-a716-446655440000"

		_, _, err := setup.api.CreateSession(context.Background(), user1, ChatRequest{
			Message:   "hello",
			SessionID: clientID,
		})
		require.NoError(t, err)

		// Different user with same ID should fail.
		_, _, err2 := setup.api.CreateSession(context.Background(), user2, ChatRequest{
			Message:   "world",
			SessionID: clientID,
		})
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "bad request")
	})

	t.Run("invalid UUID returns error", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		user := UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}

		_, _, err := setup.api.CreateSession(context.Background(), user, ChatRequest{
			Message:   "hello",
			SessionID: "not-a-uuid",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "valid UUID")
	})

	t.Run("duplicate ID in persisted store returns already_exists", func(t *testing.T) {
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

		clientID := "770e8400-e29b-41d4-a716-446655440000"
		user := UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}

		// First create succeeds and persists.
		sessID, status, err := api.CreateSession(context.Background(), user, ChatRequest{
			Message:   "hello",
			SessionID: clientID,
		})
		require.NoError(t, err)
		assert.Equal(t, clientID, sessID)
		assert.Equal(t, "accepted", status)

		// Remove from active map to simulate session eviction.
		api.sessions.Delete(clientID)

		// Second call should find it in the store.
		sessID2, status2, err2 := api.CreateSession(context.Background(), user, ChatRequest{
			Message:   "world",
			SessionID: clientID,
		})
		require.NoError(t, err2)
		assert.Equal(t, clientID, sessID2)
		assert.Equal(t, "already_exists", status2)
	})

	t.Run("concurrent creation with same ID is safe", func(t *testing.T) {
		t.Parallel()

		setup := newAPITestSetup(t, true, true, "")
		user := UserIdentity{UserID: "admin", Username: "admin", Role: auth.RoleAdmin}
		clientID := "880e8400-e29b-41d4-a716-446655440000"

		var wg sync.WaitGroup
		results := make([]string, 10)
		errs := make([]error, 10)

		for i := range 10 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, status, err := setup.api.CreateSession(context.Background(), user, ChatRequest{
					Message:   fmt.Sprintf("msg-%d", idx),
					SessionID: clientID,
				})
				results[idx] = status
				errs[idx] = err
			}(i)
		}
		wg.Wait()

		// Exactly one should be "accepted", rest should be "already_exists" or errors.
		acceptedCount := 0
		for i := range 10 {
			if errs[i] == nil && results[i] == "accepted" {
				acceptedCount++
			}
		}
		assert.Equal(t, 1, acceptedCount, "exactly one goroutine should win the creation race")
	})
}
