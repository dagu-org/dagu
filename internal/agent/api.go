package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// respondErrorDirect writes a JSON error response (for use outside API methods).
func respondErrorDirect(w http.ResponseWriter, status int, code api.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"code":    string(code),
		"message": message,
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// defaultUserID is used when no user is authenticated (e.g., auth disabled).
// This value should match the system's expected default user identifier.
const defaultUserID = "admin"
const defaultUserRole = auth.RoleAdmin

// getUserIDFromContext extracts the user ID from the request context.
// Returns "admin" if no user is authenticated (e.g., auth mode is "none").
func getUserIDFromContext(ctx context.Context) string {
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		return user.ID
	}
	return defaultUserID
}

// API handles HTTP requests for the agent.
type API struct {
	sessions    sync.Map // id -> *SessionManager (active sessions)
	store       SessionStore
	configStore ConfigStore
	modelStore  ModelStore
	skillStore  SkillStore
	providers   *ProviderCache
	workingDir  string
	logger      *slog.Logger
	dagStore    DAGMetadataStore // For resolving DAG file paths
	environment EnvironmentInfo
	hooks       *Hooks
	memoryStore MemoryStore
	soulStore   SoulStore
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	ConfigStore  ConfigStore
	ModelStore   ModelStore
	SkillStore   SkillStore
	SoulStore    SoulStore
	WorkingDir   string
	Logger       *slog.Logger
	SessionStore SessionStore
	DAGStore     DAGMetadataStore // For resolving DAG file paths
	Environment  EnvironmentInfo
	Hooks        *Hooks
	MemoryStore  MemoryStore
}

// SessionWithState is a session with its current state.
type SessionWithState struct {
	Session          Session `json:"session"`
	Working          bool    `json:"working"`
	HasPendingPrompt bool    `json:"has_pending_prompt"`
	Model            string  `json:"model,omitempty"`
	TotalCost        float64 `json:"total_cost"`
}

// NewAPI creates a new API instance.
func NewAPI(cfg APIConfig) *API {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &API{
		configStore: cfg.ConfigStore,
		modelStore:  cfg.ModelStore,
		skillStore:  cfg.SkillStore,
		soulStore:   cfg.SoulStore,
		providers:   NewProviderCache(),
		workingDir:  cfg.WorkingDir,
		logger:      logger,
		store:       cfg.SessionStore,
		dagStore:    cfg.DAGStore,
		environment: cfg.Environment,
		hooks:       cfg.Hooks,
		memoryStore: cfg.MemoryStore,
	}
}

// RegisterRoutes registers the agent SSE stream route on the given router.
// All other agent endpoints are served through the OpenAPI handler.
func (a *API) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/agent", func(r chi.Router) {
		r.Use(a.enabledMiddleware())
		if authMiddleware != nil {
			r.Use(authMiddleware)
		}
		r.Get("/sessions/{id}/stream", a.handleStream)
	})
}

// enabledMiddleware returns middleware that checks if agent is enabled.
func (a *API) enabledMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !a.configStore.IsEnabled(r.Context()) {
				respondErrorDirect(w, http.StatusNotFound, api.ErrorCodeNotFound, "Agent feature is disabled")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveContexts resolves DAG file names to full paths using the DAG store.
func (a *API) resolveContexts(ctx context.Context, contexts []DAGContext) []ResolvedDAGContext {
	if len(contexts) == 0 || a.dagStore == nil {
		return nil
	}

	var resolved []ResolvedDAGContext
	for _, c := range contexts {
		if c.DAGFile == "" {
			continue
		}

		dag, err := a.dagStore.GetMetadata(ctx, c.DAGFile)
		if err != nil || dag == nil {
			continue
		}

		resolved = append(resolved, ResolvedDAGContext{
			DAGFilePath: dag.Location,
			DAGName:     dag.Name,
			DAGRunID:    c.DAGRunID,
		})
	}
	return resolved
}

// formatMessageWithContexts prepends DAG context information to the user message.
func formatMessageWithContexts(message string, contexts []ResolvedDAGContext) string {
	if len(contexts) == 0 {
		return message
	}

	var contextLines []string
	for _, ctx := range contexts {
		line := formatContextLine(ctx)
		if line != "" {
			contextLines = append(contextLines, line)
		}
	}

	if len(contextLines) == 0 {
		return message
	}

	return fmt.Sprintf("[Referenced DAGs:\n%s]\n\n%s", strings.Join(contextLines, "\n"), message)
}

// formatContextLine formats a single DAG context as a readable line.
func formatContextLine(ctx ResolvedDAGContext) string {
	var parts []string
	if ctx.DAGFilePath != "" {
		parts = append(parts, "file: "+ctx.DAGFilePath)
	}
	if ctx.DAGRunID != "" {
		parts = append(parts, "run: "+ctx.DAGRunID)
	}
	if ctx.RunStatus != "" {
		parts = append(parts, "status: "+ctx.RunStatus)
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("- %s (%s)", cmp.Or(ctx.DAGName, "unknown"), strings.Join(parts, ", "))
}

// selectModel returns the first non-empty model from the provided choices,
// falling back to the default model from config.
// Priority: requestModel > sessionModel > config default.
func selectModel(requestModel, sessionModel, configModel string) string {
	return cmp.Or(requestModel, sessionModel, configModel)
}

// getDefaultModelID returns the default model ID from config.
func (a *API) getDefaultModelID(ctx context.Context) string {
	cfg, err := a.configStore.Load(ctx)
	if err != nil {
		a.logger.Warn("Failed to load agent config for default model", "error", err)
		return ""
	}
	return cfg.DefaultModelID
}

// resolveProvider resolves a model ID to an LLM provider and model config.
// If modelID is empty, uses the default from config.
// If the requested model is not found (e.g., deleted), falls back to the default.
func (a *API) resolveProvider(ctx context.Context, modelID string) (llm.Provider, *ModelConfig, error) {
	if a.modelStore == nil {
		return nil, nil, errors.New("model store not configured")
	}

	defaultID := a.getDefaultModelID(ctx)
	modelID = cmp.Or(modelID, defaultID)
	if modelID == "" {
		return nil, nil, errors.New("no model configured")
	}

	model, err := a.modelStore.GetByID(ctx, modelID)
	if errors.Is(err, ErrModelNotFound) && defaultID != "" && defaultID != modelID {
		// Requested model was deleted; fall back to default
		model, err = a.modelStore.GetByID(ctx, defaultID)
	}
	if err != nil {
		return nil, nil, err
	}

	provider, _, err := a.providers.GetOrCreate(model.ToLLMConfig())
	if err != nil {
		return nil, nil, err
	}
	return provider, model, nil
}

// loadEnabledSkills returns the list of enabled skill IDs from the agent config.
func (a *API) loadEnabledSkills(ctx context.Context) []string {
	cfg, err := a.configStore.Load(ctx)
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.EnabledSkills
}

// loadSelectedSoul returns the selected soul from the agent config.
// Falls back to "default" soul if the configured soul is not found.
// Returns nil if no soul store is configured or no soul is available.
func (a *API) loadSelectedSoul(ctx context.Context) *Soul {
	if a.soulStore == nil {
		return nil
	}
	cfg, err := a.configStore.Load(ctx)
	if err != nil {
		a.logger.Debug("failed to load agent config for soul selection", "error", err)
		return nil
	}
	if cfg == nil {
		return nil
	}
	soulID := cfg.SelectedSoulID
	if soulID == "" {
		soulID = "default"
	}
	soul, err := a.soulStore.GetByID(ctx, soulID)
	if err != nil {
		if soulID != "default" {
			a.logger.Debug("selected soul not found, falling back to default", "soulId", soulID, "error", err)
			soul, err = a.soulStore.GetByID(ctx, "default")
			if err != nil {
				a.logger.Debug("default soul not found", "error", err)
				return nil
			}
		} else {
			a.logger.Debug("default soul not found", "error", err)
			return nil
		}
	}
	return soul
}

// createMessageCallback returns a persistence callback for the given session ID.
// Returns nil if no store is configured.
func (a *API) createMessageCallback(id string) func(ctx context.Context, msg Message) error {
	if a.store == nil {
		return nil
	}
	return func(ctx context.Context, msg Message) error {
		return a.store.AddMessage(ctx, id, &msg)
	}
}

// persistNewSession saves a new session to the store if configured.
func (a *API) persistNewSession(ctx context.Context, id, userID, dagName string, now time.Time) {
	if a.store == nil {
		return
	}
	sess := &Session{
		ID:        id,
		UserID:    userID,
		DAGName:   dagName,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.store.CreateSession(ctx, sess); err != nil {
		a.logger.Warn("Failed to persist session", "error", err)
	}
}

// formatMessage resolves DAG contexts and formats the message with context information.
func (a *API) formatMessage(ctx context.Context, message string, contexts []DAGContext) string {
	resolved := a.resolveContexts(ctx, contexts)
	return formatMessageWithContexts(message, resolved)
}

// collectActiveSessions gathers active sessions for a user.
func (a *API) collectActiveSessions(userID string, activeIDs map[string]struct{}) []SessionWithState {
	var sessions []SessionWithState

	a.sessions.Range(func(key, value any) bool {
		mgr, ok := value.(*SessionManager)
		if !ok {
			return true // skip invalid entry
		}
		if mgr.UserID() != userID {
			return true
		}

		sess := mgr.GetSession()
		// Skip sub-sessions (delegate sessions) from the main listing.
		if sess.ParentSessionID != "" {
			return true
		}

		id, ok := key.(string)
		if !ok {
			return true // skip invalid key
		}
		activeIDs[id] = struct{}{}
		sessions = append(sessions, SessionWithState{
			Session:          sess,
			Working:          mgr.IsWorking(),
			HasPendingPrompt: mgr.HasPendingPrompt(),
			Model:            mgr.GetModel(),
			TotalCost:        mgr.GetTotalCost(),
		})
		return true
	})

	return sessions
}

// appendPersistedSessions adds non-active persisted sessions to the list.
func (a *API) appendPersistedSessions(ctx context.Context, userID string, activeIDs map[string]struct{}, sessions []SessionWithState) []SessionWithState {
	if a.store == nil {
		return sessions
	}

	persisted, err := a.store.ListSessions(ctx, userID)
	if err != nil {
		a.logger.Warn("Failed to list persisted sessions", "error", err)
		return sessions
	}

	for _, sess := range persisted {
		if _, exists := activeIDs[sess.ID]; exists {
			continue
		}
		// Exclude sub-sessions (delegate sessions) from the main listing.
		if sess.ParentSessionID != "" {
			continue
		}
		sessions = append(sessions, SessionWithState{
			Session: *sess,
			Working: false,
		})
	}

	return sessions
}

// getActiveSession retrieves an active session if it exists and belongs to the user.
func (a *API) getActiveSession(id, userID string) (*SessionManager, bool) {
	mgrValue, ok := a.sessions.Load(id)
	if !ok {
		return nil, false
	}
	mgr, ok := mgrValue.(*SessionManager)
	if !ok {
		return nil, false
	}
	if mgr.UserID() != userID {
		return nil, false
	}
	return mgr, true
}

// getStoredSession retrieves a session from the store if it exists and belongs to the user.
func (a *API) getStoredSession(ctx context.Context, id, userID string) (*Session, []Message, bool) {
	if a.store == nil {
		return nil, nil, false
	}

	sess, err := a.store.GetSession(ctx, id)
	if err != nil || sess == nil || sess.UserID != userID {
		return nil, nil, false
	}

	messages, err := a.store.GetMessages(ctx, id)
	if err != nil {
		a.logger.Error("Failed to get messages from store", "error", err)
		messages = []Message{}
	}

	return sess, messages, true
}

// getOrReactivateSession retrieves an active session or reactivates it from storage.
func (a *API) getOrReactivateSession(ctx context.Context, id string, user UserIdentity) (*SessionManager, bool) {
	// Check active sessions first
	if mgr, ok := a.getActiveSession(id, user.UserID); ok {
		return mgr, true
	}

	// Try to reactivate from store
	return a.reactivateSession(ctx, id, user)
}

// reactivateSession restores a session from storage and makes it active.
func (a *API) reactivateSession(ctx context.Context, id string, user UserIdentity) (*SessionManager, bool) {
	if a.store == nil {
		return nil, false
	}

	sess, err := a.store.GetSession(ctx, id)
	if err != nil || sess == nil || sess.UserID != user.UserID {
		return nil, false
	}

	// Prevent reactivating sub-sessions as top-level sessions.
	if sess.ParentSessionID != "" {
		return nil, false
	}

	messages, err := a.store.GetMessages(ctx, id)
	if err != nil {
		a.logger.Warn("Failed to load messages for reactivation", "error", err)
		messages = []Message{}
	}

	seqID, err := a.store.GetLatestSequenceID(ctx, id)
	if err != nil {
		seqID = int64(len(messages))
	}

	mgr := NewSessionManager(SessionManagerConfig{
		ID:            id,
		User:          user,
		Logger:        a.logger,
		WorkingDir:    a.workingDir,
		OnMessage:     a.createMessageCallback(id),
		History:       messages,
		SequenceID:    seqID,
		Environment:   a.environment,
		SafeMode:      true, // Default to safe mode for reactivated sessions
		Hooks:         a.hooks,
		MemoryStore:   a.memoryStore,
		SkillStore:    a.skillStore,
		EnabledSkills: a.loadEnabledSkills(ctx),
		DAGName:       sess.DAGName,
		SessionStore:  a.store,
		Soul:          a.loadSelectedSoul(ctx),
	})
	mgr.registry = &sessionRegistry{sessions: &a.sessions, parent: mgr}
	a.sessions.Store(id, mgr)

	return mgr, true
}

// handleStream provides SSE streaming for session updates.
// GET /api/v1/agent/sessions/{id}/stream
func (a *API) handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveSession(id, userID)
	if !ok {
		respondErrorDirect(w, http.StatusNotFound, api.ErrorCodeNotFound, "Session not found")
		return
	}

	a.setupSSEHeaders(w)

	// Use atomic subscribe+snapshot to prevent race condition
	// where messages could be missed between getting initial state and subscribing
	snapshot, next := mgr.SubscribeWithSnapshot(r.Context())
	a.sendSSEMessage(w, snapshot)

	type streamResult struct {
		resp StreamResponse
		cont bool
	}

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	done := make(chan struct{})
	defer close(done)

	ch := make(chan streamResult, 1)
	go func() {
		for {
			resp, cont := next()
			select {
			case ch <- streamResult{resp, cont}:
			case <-done:
				return
			}
			if !cont {
				return
			}
		}
	}()

	for {
		select {
		case res := <-ch:
			if !res.cont {
				return
			}
			a.sendSSEMessage(w, res.resp)
			heartbeat.Reset(heartbeatInterval)
		case <-heartbeat.C:
			// SSE comment as heartbeat to keep connection alive
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				a.logger.Debug("SSE heartbeat write failed", "error", err)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

// setupSSEHeaders configures the response headers for Server-Sent Events.
// Note: CORS headers are typically handled by middleware at the router level.
// If SSE-specific CORS headers are needed, configure them via the server config.
func (a *API) setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// CORS headers are managed by the server's CORS middleware configuration.
	// Do not set Access-Control-Allow-Origin here to avoid security issues.
}

// sendSSEMessage sends a single SSE message to the client.
func (a *API) sendSSEMessage(w http.ResponseWriter, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal SSE data", "error", err)
		return
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		a.logger.Debug("SSE write failed", "error", err)
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// ToolInfo contains metadata for a registered agent tool.
type ToolInfo struct {
	Name        string
	Label       string
	Description string
}

// ListTools returns metadata for all registered tools.
func (a *API) ListTools() []ToolInfo {
	regs := RegisteredTools()
	result := make([]ToolInfo, len(regs))
	for i, reg := range regs {
		result[i] = ToolInfo{Name: reg.Name, Label: reg.Label, Description: reg.Description}
	}
	return result
}

// CreateSession creates a new session with the first message.
// Returns the session ID on success.
func (a *API) CreateSession(ctx context.Context, user UserIdentity, req ChatRequest) (string, error) {
	if req.Message == "" {
		return "", ErrMessageRequired
	}

	model := selectModel(req.Model, "", a.getDefaultModelID(ctx))

	provider, modelCfg, err := a.resolveProvider(ctx, model)
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		return "", ErrAgentNotConfigured
	}

	id := uuid.New().String()
	now := time.Now()

	resolved := a.resolveContexts(ctx, req.DAGContexts)
	var dagName string
	if len(resolved) > 0 {
		dagName = resolved[0].DAGName
	}

	mgr := NewSessionManager(SessionManagerConfig{
		ID:              id,
		User:            user,
		Logger:          a.logger,
		WorkingDir:      a.workingDir,
		OnMessage:       a.createMessageCallback(id),
		Environment:     a.environment,
		SafeMode:        req.SafeMode,
		Hooks:           a.hooks,
		InputCostPer1M:  modelCfg.InputCostPer1M,
		OutputCostPer1M: modelCfg.OutputCostPer1M,
		MemoryStore:     a.memoryStore,
		SkillStore:      a.skillStore,
		EnabledSkills:   a.loadEnabledSkills(ctx),
		DAGName:         dagName,
		SessionStore:    a.store,
		Soul:            a.loadSelectedSoul(ctx),
	})
	mgr.registry = &sessionRegistry{sessions: &a.sessions, parent: mgr}

	// Persist session before accepting the first message so that
	// the onMessage callback (store.AddMessage) can find the session.
	a.persistNewSession(ctx, id, user.UserID, dagName, now)
	a.sessions.Store(id, mgr)

	messageWithContext := formatMessageWithContexts(req.Message, resolved)
	if err := mgr.AcceptUserMessage(ctx, provider, model, modelCfg.Model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		a.sessions.Delete(id)
		return "", ErrFailedToProcessMessage
	}

	return id, nil
}

// ListSessions returns all sessions for the given user.
func (a *API) ListSessions(ctx context.Context, userID string) []SessionWithState {
	activeIDs := make(map[string]struct{})
	sessions := a.collectActiveSessions(userID, activeIDs)
	sessions = a.appendPersistedSessions(ctx, userID, activeIDs, sessions)

	if sessions == nil {
		sessions = []SessionWithState{}
	}
	return sessions
}

// ListSessionsPaginated returns a paginated list of sessions for the given user.
// Active sessions appear first, followed by persisted inactive sessions.
func (a *API) ListSessionsPaginated(ctx context.Context, userID string, page, perPage int) exec.PaginatedResult[SessionWithState] {
	pg := exec.NewPaginator(page, perPage)

	activeIDs := make(map[string]struct{})
	activeSessions := a.collectActiveSessions(userID, activeIDs)

	combined := make([]SessionWithState, 0, len(activeSessions))
	combined = append(combined, activeSessions...)

	if a.store != nil {
		persisted, err := a.store.ListSessions(ctx, userID)
		if err != nil {
			a.logger.Warn("Failed to list persisted sessions", "error", err)
		} else {
			for _, sess := range persisted {
				if _, exists := activeIDs[sess.ID]; exists {
					continue
				}
				if sess.ParentSessionID != "" {
					continue
				}
				combined = append(combined, SessionWithState{Session: *sess})
			}
		}
	}

	total := len(combined)
	start := min(pg.Offset(), total)
	end := min(pg.Offset()+pg.Limit(), total)

	return exec.NewPaginatedResult(combined[start:end], total, pg)
}

// GetSessionDetail returns session details including messages and state.
func (a *API) GetSessionDetail(ctx context.Context, sessionID, userID string) (*StreamResponse, error) {
	// Check active sessions first
	if mgr, ok := a.getActiveSession(sessionID, userID); ok {
		sess := mgr.GetSession()
		return &StreamResponse{
			Messages: mgr.GetMessages(),
			Session:  &sess,
			SessionState: &SessionState{
				SessionID:        sessionID,
				Working:          mgr.IsWorking(),
				HasPendingPrompt: mgr.HasPendingPrompt(),
				Model:            mgr.GetModel(),
				TotalCost:        mgr.GetTotalCost(),
			},
			Delegates: mgr.GetDelegates(),
		}, nil
	}

	// Fall back to store for inactive sessions
	sess, messages, ok := a.getStoredSession(ctx, sessionID, userID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	// Build delegate snapshots from stored sub-sessions.
	var delegates []DelegateSnapshot
	if a.store != nil {
		subSessions, err := a.store.ListSubSessions(ctx, sessionID)
		if err == nil {
			for _, sub := range subSessions {
				delegates = append(delegates, DelegateSnapshot{
					ID:     sub.ID,
					Task:   sub.DelegateTask,
					Status: DelegateStatusCompleted,
				})
			}
		}
	}

	return &StreamResponse{
		Messages: messages,
		Session:  sess,
		SessionState: &SessionState{
			SessionID: sessionID,
			Working:   false,
		},
		Delegates: delegates,
	}, nil
}

// SendMessage sends a message to an existing session.
func (a *API) SendMessage(ctx context.Context, sessionID string, user UserIdentity, req ChatRequest) error {
	mgr, ok := a.getOrReactivateSession(ctx, sessionID, user)
	if !ok {
		return ErrSessionNotFound
	}
	mgr.UpdateUserContext(user)

	if req.Message == "" {
		return ErrMessageRequired
	}

	model := selectModel(req.Model, mgr.GetModel(), a.getDefaultModelID(ctx))

	provider, modelCfg, err := a.resolveProvider(ctx, model)
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		return ErrAgentNotConfigured
	}
	messageWithContext := a.formatMessage(ctx, req.Message, req.DAGContexts)

	mgr.SetSafeMode(req.SafeMode)
	mgr.UpdatePricing(modelCfg.InputCostPer1M, modelCfg.OutputCostPer1M)

	if err := mgr.AcceptUserMessage(ctx, provider, model, modelCfg.Model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		return ErrFailedToProcessMessage
	}

	return nil
}

// CancelSession cancels an active session.
func (a *API) CancelSession(ctx context.Context, sessionID, userID string) error {
	mgr, ok := a.getActiveSession(sessionID, userID)
	if !ok {
		return ErrSessionNotFound
	}

	if err := mgr.Cancel(ctx); err != nil {
		a.logger.Error("Failed to cancel session", "error", err)
		return ErrFailedToCancel
	}

	return nil
}

// SubmitUserResponse submits a user's response to an agent prompt.
func (a *API) SubmitUserResponse(_ context.Context, sessionID, userID string, resp UserPromptResponse) error {
	mgr, ok := a.getActiveSession(sessionID, userID)
	if !ok {
		return ErrSessionNotFound
	}

	if resp.PromptID == "" {
		return ErrPromptIDRequired
	}

	if !mgr.SubmitUserResponse(resp) {
		return ErrPromptExpired
	}

	return nil
}

// EnabledMiddleware returns middleware that checks if agent is enabled.
func (a *API) EnabledMiddleware() func(http.Handler) http.Handler {
	return a.enabledMiddleware()
}

// HandleStream provides SSE streaming for session updates.
func (a *API) HandleStream(w http.ResponseWriter, r *http.Request) {
	a.handleStream(w, r)
}

// heartbeatInterval is the SSE heartbeat period to keep connections alive.
const heartbeatInterval = 15 * time.Second

// idleSessionTimeout is the duration after which idle sessions are cleaned up.
const idleSessionTimeout = 30 * time.Minute

// cleanupInterval is how often the cleanup goroutine runs.
const cleanupInterval = 5 * time.Minute

// stuckHeartbeatTimeout is the maximum time without a heartbeat before
// a working session is considered stuck and cancelled.
const stuckHeartbeatTimeout = 3 * loopHeartbeatInterval

// StartCleanup begins periodic cleanup of idle sessions.
// It should be called once when the API is initialized and will
// stop when the context is cancelled.
func (a *API) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.cleanupIdleSessions()
			}
		}
	}()
}

// cleanupIdleSessions removes sessions that have been idle too long and are not working.
func (a *API) cleanupIdleSessions() {
	cutoff := time.Now().Add(-idleSessionTimeout)
	var toDelete []string

	a.sessions.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		mgr, ok := value.(*SessionManager)
		if !ok {
			return true
		}
		// Skip delegate sub-sessions — their lifecycle is managed by the parent.
		sess := mgr.GetSession()
		if sess.ParentSessionID != "" {
			return true
		}
		// Detect stuck sessions: working but no heartbeat in 30s (3x the 10s interval).
		if mgr.IsWorking() {
			lastHB := mgr.LastHeartbeat()
			if !lastHB.IsZero() && time.Since(lastHB) > stuckHeartbeatTimeout {
				if err := mgr.Cancel(context.Background()); err != nil {
					a.logger.Warn("Failed to cancel stuck session", "session_id", id, "error", err)
				} else {
					a.logger.Warn("Cancelled stuck session", "session_id", id)
				}
			}
		}
		// Cancelled sessions remain in the map until the next cleanup cycle
		// so they can still be viewed or reactivated by the user.
		if !mgr.IsWorking() && mgr.LastActivity().Before(cutoff) {
			toDelete = append(toDelete, id)
		}
		return true
	})

	for _, id := range toDelete {
		if val, ok := a.sessions.Load(id); ok {
			if mgr, ok := val.(*SessionManager); ok {
				_ = mgr.Cancel(context.Background())
			}
		}
		a.sessions.Delete(id)
		a.logger.Debug("Cleaned up idle session", "session_id", id)
	}
}
