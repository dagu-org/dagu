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

// maxRequestBodySize is the maximum allowed size for JSON request bodies (1 MB).
const maxRequestBodySize = 1 << 20

// defaultUserID is used when no user is authenticated (e.g., auth disabled).
// This value should match the system's expected default user identifier.
const defaultUserID = "admin"

// getUserIDFromContext extracts the user ID from the request context.
// Returns "admin" if no user is authenticated (e.g., auth mode is "none").
func getUserIDFromContext(ctx context.Context) string {
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		return user.ID
	}
	return defaultUserID
}

// getUserContextFromRequest extracts user identity and IP from the request context.
func getUserContextFromRequest(r *http.Request) (userID, username, ipAddress string) {
	userID, username = defaultUserID, defaultUserID
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userID = user.ID
		username = user.Username
	}
	ipAddress, _ = auth.ClientIPFromContext(r.Context())
	return
}

// API handles HTTP requests for the agent.
type API struct {
	conversations sync.Map // id -> *ConversationManager (active conversations)
	store         ConversationStore
	configStore   ConfigStore
	modelStore    ModelStore
	providers     *ProviderCache
	workingDir    string
	logger        *slog.Logger
	dagStore      exec.DAGStore // For resolving DAG file paths
	environment   EnvironmentInfo
	hooks         *Hooks
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	ConfigStore       ConfigStore
	ModelStore        ModelStore
	WorkingDir        string
	Logger            *slog.Logger
	ConversationStore ConversationStore
	DAGStore          exec.DAGStore // For resolving DAG file paths
	Environment       EnvironmentInfo
	Hooks             *Hooks
}

// ConversationWithState is a conversation with its current state.
type ConversationWithState struct {
	Conversation Conversation `json:"conversation"`
	Working      bool         `json:"working"`
	Model        string       `json:"model,omitempty"`
	TotalCost    float64      `json:"total_cost"`
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
		providers:   NewProviderCache(),
		workingDir:  cfg.WorkingDir,
		logger:      logger,
		store:       cfg.ConversationStore,
		dagStore:    cfg.DAGStore,
		environment: cfg.Environment,
		hooks:       cfg.Hooks,
	}
}

// RegisterRoutes registers the agent API routes on the given router.
// The authMiddleware parameter should be the same auth middleware used for other API routes.
func (a *API) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/agent", func(r chi.Router) {
		// Check if agent is enabled (must be first middleware)
		r.Use(a.enabledMiddleware())

		// Apply auth middleware to all agent routes
		if authMiddleware != nil {
			r.Use(authMiddleware)
		}

		// Conversation management
		r.Post("/conversations/new", a.handleNewConversation)
		r.Get("/conversations", a.handleListConversations)

		// Single conversation operations
		r.Route("/conversations/{id}", func(r chi.Router) {
			r.Get("/", a.handleGetConversation)
			r.Post("/chat", a.handleChat)
			r.Get("/stream", a.handleStream)
			r.Post("/cancel", a.handleCancel)
			r.Post("/respond", a.handleUserResponse)
		})
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
// Priority: requestModel > conversationModel > config default.
func selectModel(requestModel, conversationModel, configModel string) string {
	return cmp.Or(requestModel, conversationModel, configModel)
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

// createMessageCallback returns a persistence callback for the given conversation ID.
// Returns nil if no store is configured.
func (a *API) createMessageCallback(id string) func(ctx context.Context, msg Message) error {
	if a.store == nil {
		return nil
	}
	return func(ctx context.Context, msg Message) error {
		return a.store.AddMessage(ctx, id, &msg)
	}
}

// persistNewConversation saves a new conversation to the store if configured.
func (a *API) persistNewConversation(ctx context.Context, id, userID string, now time.Time) {
	if a.store == nil {
		return
	}
	conv := &Conversation{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.store.CreateConversation(ctx, conv); err != nil {
		a.logger.Warn("Failed to persist conversation", "error", err)
	}
}

// formatMessage resolves DAG contexts and formats the message with context information.
func (a *API) formatMessage(ctx context.Context, message string, contexts []DAGContext) string {
	resolved := a.resolveContexts(ctx, contexts)
	return formatMessageWithContexts(message, resolved)
}

// respondJSON writes a JSON response with the given status code.
func (a *API) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// respondError writes a JSON error response matching the v2 API format.
func (a *API) respondError(w http.ResponseWriter, status int, code api.ErrorCode, message string) {
	respondErrorDirect(w, status, code, message)
}

// handleNewConversation creates a new conversation and sends the first message.
// POST /api/v1/agent/conversations/new
func (a *API) handleNewConversation(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Invalid JSON")
		return
	}

	if req.Message == "" {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Message is required")
		return
	}

	userID, username, ipAddress := getUserContextFromRequest(r)
	model := selectModel(req.Model, "", a.getDefaultModelID(r.Context()))

	provider, modelCfg, err := a.resolveProvider(r.Context(), model)
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		a.respondError(w, http.StatusServiceUnavailable, api.ErrorCodeInternalError, "Agent is not configured properly")
		return
	}

	id := uuid.New().String()
	now := time.Now()

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:              id,
		UserID:          userID,
		Logger:          a.logger,
		WorkingDir:      a.workingDir,
		OnMessage:       a.createMessageCallback(id),
		Environment:     a.environment,
		SafeMode:        req.SafeMode,
		Hooks:           a.hooks,
		Username:        username,
		IPAddress:       ipAddress,
		InputCostPer1M:  modelCfg.InputCostPer1M,
		OutputCostPer1M: modelCfg.OutputCostPer1M,
	})

	a.persistNewConversation(r.Context(), id, userID, now)
	a.conversations.Store(id, mgr)

	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)
	if err := mgr.AcceptUserMessage(r.Context(), provider, model, modelCfg.Model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		a.respondError(w, http.StatusInternalServerError, api.ErrorCodeInternalError, "Failed to process message")
		return
	}

	a.respondJSON(w, http.StatusCreated, NewConversationResponse{
		ConversationID: id,
		Status:         "accepted",
	})
}

// handleListConversations lists all conversations for the current user.
// GET /api/v1/agent/conversations
func (a *API) handleListConversations(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r.Context())

	activeIDs := make(map[string]struct{})
	conversations := a.collectActiveConversations(userID, activeIDs)
	conversations = a.appendPersistedConversations(r.Context(), userID, activeIDs, conversations)

	// Ensure we return [] instead of null in JSON when no conversations exist
	if conversations == nil {
		conversations = []ConversationWithState{}
	}

	a.respondJSON(w, http.StatusOK, conversations)
}

// collectActiveConversations gathers active conversations for a user.
func (a *API) collectActiveConversations(userID string, activeIDs map[string]struct{}) []ConversationWithState {
	var conversations []ConversationWithState

	a.conversations.Range(func(key, value any) bool {
		mgr, ok := value.(*ConversationManager)
		if !ok {
			return true // skip invalid entry
		}
		if mgr.UserID() != userID {
			return true
		}

		id, ok := key.(string)
		if !ok {
			return true // skip invalid key
		}
		activeIDs[id] = struct{}{}
		conversations = append(conversations, ConversationWithState{
			Conversation: mgr.GetConversation(),
			Working:      mgr.IsWorking(),
			Model:        mgr.GetModel(),
			TotalCost:    mgr.GetTotalCost(),
		})
		return true
	})

	return conversations
}

// appendPersistedConversations adds non-active persisted conversations to the list.
func (a *API) appendPersistedConversations(ctx context.Context, userID string, activeIDs map[string]struct{}, conversations []ConversationWithState) []ConversationWithState {
	if a.store == nil {
		return conversations
	}

	persisted, err := a.store.ListConversations(ctx, userID)
	if err != nil {
		a.logger.Warn("Failed to list persisted conversations", "error", err)
		return conversations
	}

	for _, conv := range persisted {
		if _, exists := activeIDs[conv.ID]; exists {
			continue
		}
		conversations = append(conversations, ConversationWithState{
			Conversation: *conv,
			Working:      false,
		})
	}

	return conversations
}

// handleGetConversation returns conversation details and messages.
// GET /api/v1/agent/conversations/{id}
func (a *API) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	// Check active conversations first
	if mgr, ok := a.getActiveConversation(id, userID); ok {
		a.respondJSON(w, http.StatusOK, StreamResponse{
			Messages:     mgr.GetMessages(),
			Conversation: ptrTo(mgr.GetConversation()),
			ConversationState: &ConversationState{
				ConversationID: id,
				Working:        mgr.IsWorking(),
				Model:          mgr.GetModel(),
				TotalCost:      mgr.GetTotalCost(),
			},
		})
		return
	}

	// Fall back to store for inactive conversations
	conv, messages, ok := a.getStoredConversation(r.Context(), id, userID)
	if !ok {
		a.respondError(w, http.StatusNotFound, api.ErrorCodeNotFound, "Conversation not found")
		return
	}

	a.respondJSON(w, http.StatusOK, StreamResponse{
		Messages:     messages,
		Conversation: conv,
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        false,
		},
	})
}

// getActiveConversation retrieves an active conversation if it exists and belongs to the user.
func (a *API) getActiveConversation(id, userID string) (*ConversationManager, bool) {
	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		return nil, false
	}
	mgr, ok := mgrValue.(*ConversationManager)
	if !ok {
		return nil, false
	}
	if mgr.UserID() != userID {
		return nil, false
	}
	return mgr, true
}

// getStoredConversation retrieves a conversation from the store if it exists and belongs to the user.
func (a *API) getStoredConversation(ctx context.Context, id, userID string) (*Conversation, []Message, bool) {
	if a.store == nil {
		return nil, nil, false
	}

	conv, err := a.store.GetConversation(ctx, id)
	if err != nil || conv == nil || conv.UserID != userID {
		return nil, nil, false
	}

	messages, err := a.store.GetMessages(ctx, id)
	if err != nil {
		a.logger.Error("Failed to get messages from store", "error", err)
		messages = []Message{}
	}

	return conv, messages, true
}

// handleChat sends a message to an existing conversation.
// POST /api/v1/agent/conversations/{id}/chat
func (a *API) handleChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, username, ipAddress := getUserContextFromRequest(r)

	mgr, ok := a.getOrReactivateConversation(r.Context(), id, userID, username, ipAddress)
	if !ok {
		a.respondError(w, http.StatusNotFound, api.ErrorCodeNotFound, "Conversation not found")
		return
	}

	var req ChatRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Invalid JSON")
		return
	}

	if req.Message == "" {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Message is required")
		return
	}

	model := selectModel(req.Model, mgr.GetModel(), a.getDefaultModelID(r.Context()))

	provider, modelCfg, err := a.resolveProvider(r.Context(), model)
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		a.respondError(w, http.StatusServiceUnavailable, api.ErrorCodeInternalError, "Agent is not configured properly")
		return
	}
	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)

	mgr.SetSafeMode(req.SafeMode)
	mgr.UpdatePricing(modelCfg.InputCostPer1M, modelCfg.OutputCostPer1M)

	if err := mgr.AcceptUserMessage(r.Context(), provider, model, modelCfg.Model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		a.respondError(w, http.StatusInternalServerError, api.ErrorCodeInternalError, "Failed to process message")
		return
	}

	a.respondJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// getOrReactivateConversation retrieves an active conversation or reactivates it from storage.
func (a *API) getOrReactivateConversation(ctx context.Context, id, userID, username, ipAddress string) (*ConversationManager, bool) {
	// Check active conversations first
	if mgr, ok := a.getActiveConversation(id, userID); ok {
		return mgr, true
	}

	// Try to reactivate from store
	return a.reactivateConversation(ctx, id, userID, username, ipAddress)
}

// reactivateConversation restores a conversation from storage and makes it active.
func (a *API) reactivateConversation(ctx context.Context, id, userID, username, ipAddress string) (*ConversationManager, bool) {
	if a.store == nil {
		return nil, false
	}

	conv, err := a.store.GetConversation(ctx, id)
	if err != nil || conv == nil || conv.UserID != userID {
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

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:          id,
		UserID:      userID,
		Logger:      a.logger,
		WorkingDir:  a.workingDir,
		OnMessage:   a.createMessageCallback(id),
		History:     messages,
		SequenceID:  seqID,
		Environment: a.environment,
		SafeMode:    true, // Default to safe mode for reactivated conversations
		Hooks:       a.hooks,
		Username:    username,
		IPAddress:   ipAddress,
	})
	a.conversations.Store(id, mgr)

	return mgr, true
}

// handleStream provides SSE streaming for conversation updates.
// GET /api/v1/agent/conversations/{id}/stream
func (a *API) handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveConversation(id, userID)
	if !ok {
		a.respondError(w, http.StatusNotFound, api.ErrorCodeNotFound, "Conversation not found")
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

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ch := make(chan streamResult, 1)
	go func() {
		for {
			resp, cont := next()
			ch <- streamResult{resp, cont}
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
			heartbeat.Reset(15 * time.Second)
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

// handleCancel cancels an active conversation.
// POST /api/v1/agent/conversations/{id}/cancel
func (a *API) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveConversation(id, userID)
	if !ok {
		a.respondError(w, http.StatusNotFound, api.ErrorCodeNotFound, "Conversation not found")
		return
	}

	if err := mgr.Cancel(r.Context()); err != nil {
		a.logger.Error("Failed to cancel conversation", "error", err)
		a.respondError(w, http.StatusInternalServerError, api.ErrorCodeInternalError, "Failed to cancel conversation")
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// handleUserResponse submits a user's response to an agent prompt.
// POST /api/v1/agent/conversations/{id}/respond
func (a *API) handleUserResponse(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveConversation(id, userID)
	if !ok {
		a.respondError(w, http.StatusNotFound, api.ErrorCodeNotFound, "Conversation not found")
		return
	}

	var req UserPromptResponse
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Invalid JSON")
		return
	}

	if req.PromptID == "" {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "prompt_id is required")
		return
	}

	if !mgr.SubmitUserResponse(req) {
		a.respondError(w, http.StatusGone, api.ErrorCodeNotFound, "Prompt expired or already answered")
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// idleConversationTimeout is the duration after which idle conversations are cleaned up.
const idleConversationTimeout = 30 * time.Minute

// cleanupInterval is how often the cleanup goroutine runs.
const cleanupInterval = 5 * time.Minute

// StartCleanup begins periodic cleanup of idle conversations.
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
				a.cleanupIdleConversations()
			}
		}
	}()
}

// cleanupIdleConversations removes conversations that have been idle too long and are not working.
func (a *API) cleanupIdleConversations() {
	cutoff := time.Now().Add(-idleConversationTimeout)
	var toDelete []string

	a.conversations.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		mgr, ok := value.(*ConversationManager)
		if !ok {
			return true
		}
		if !mgr.IsWorking() && mgr.LastActivity().Before(cutoff) {
			toDelete = append(toDelete, id)
		}
		return true
	})

	for _, id := range toDelete {
		a.conversations.Delete(id)
		a.logger.Debug("Cleaned up idle conversation", "conversation_id", id)
	}
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
