package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// defaultUserID is used when no user is authenticated (e.g., auth disabled).
const defaultUserID = "admin"

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
	conversations sync.Map // id -> *ConversationManager (active conversations)
	store         ConversationStore
	provider      llm.Provider
	model         string
	workingDir    string
	logger        *slog.Logger
	dagStore      exec.DAGStore // For resolving DAG file paths
	environment   EnvironmentInfo
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	Provider          llm.Provider
	Model             string
	WorkingDir        string
	Logger            *slog.Logger
	ConversationStore ConversationStore
	DAGStore          exec.DAGStore // For resolving DAG file paths
	Environment       EnvironmentInfo
}

// NewAPI creates a new API instance.
func NewAPI(cfg APIConfig) *API {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &API{
		provider:    cfg.Provider,
		model:       cfg.Model,
		workingDir:  cfg.WorkingDir,
		logger:      logger,
		store:       cfg.ConversationStore,
		dagStore:    cfg.DAGStore,
		environment: cfg.Environment,
	}
}

// RegisterRoutes registers the agent API routes on the given router.
// The authMiddleware parameter should be the same auth middleware used for other API routes.
func (a *API) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v2/agent", func(r chi.Router) {
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
		})
	})
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
		parts = append(parts, fmt.Sprintf("file: %s", ctx.DAGFilePath))
	}
	if ctx.DAGRunID != "" {
		parts = append(parts, fmt.Sprintf("run: %s", ctx.DAGRunID))
	}
	if ctx.RunStatus != "" {
		parts = append(parts, fmt.Sprintf("status: %s", ctx.RunStatus))
	}
	if len(parts) == 0 {
		return ""
	}

	name := ctx.DAGName
	if name == "" {
		name = "unknown"
	}
	return fmt.Sprintf("- %s (%s)", name, strings.Join(parts, ", "))
}

// selectModel returns the first non-empty model from the provided choices,
// falling back to the API's default model.
func (a *API) selectModel(requestModel, conversationModel string) string {
	if requestModel != "" {
		return requestModel
	}
	if conversationModel != "" {
		return conversationModel
	}
	return a.model
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
		a.logger.Error("Failed to persist conversation", "error", err)
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

// handleNewConversation creates a new conversation and sends the first message.
// POST /api/v2/agent/conversations/new
func (a *API) handleNewConversation(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	userID := getUserIDFromContext(r.Context())
	model := a.selectModel(req.Model, "")
	id := uuid.New().String()
	now := time.Now()

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:          id,
		UserID:      userID,
		Logger:      a.logger,
		WorkingDir:  a.workingDir,
		OnMessage:   a.createMessageCallback(id),
		Environment: a.environment,
	})

	a.persistNewConversation(r.Context(), id, userID, now)
	a.conversations.Store(id, mgr)

	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)
	if err := mgr.AcceptUserMessage(r.Context(), a.provider, model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	a.respondJSON(w, http.StatusCreated, NewConversationResponse{
		ConversationID: id,
		Status:         "accepted",
	})
}

// handleListConversations lists all conversations for the current user.
// GET /api/v2/agent/conversations
func (a *API) handleListConversations(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r.Context())

	activeIDs := make(map[string]bool)
	conversations := a.collectActiveConversations(userID, activeIDs)
	conversations = a.appendPersistedConversations(r.Context(), userID, activeIDs, conversations)

	a.respondJSON(w, http.StatusOK, conversations)
}

// collectActiveConversations gathers active conversations for a user.
func (a *API) collectActiveConversations(userID string, activeIDs map[string]bool) []ConversationWithState {
	var conversations []ConversationWithState
	a.conversations.Range(func(key, value any) bool {
		mgr := value.(*ConversationManager)
		if mgr.UserID() != userID {
			return true
		}
		id := key.(string)
		activeIDs[id] = true
		conversations = append(conversations, ConversationWithState{
			Conversation: mgr.GetConversation(),
			Working:      mgr.IsWorking(),
			Model:        mgr.GetModel(),
		})
		return true
	})
	return conversations
}

// appendPersistedConversations adds non-active persisted conversations to the list.
func (a *API) appendPersistedConversations(ctx context.Context, userID string, activeIDs map[string]bool, conversations []ConversationWithState) []ConversationWithState {
	if a.store == nil {
		return conversations
	}

	persisted, err := a.store.ListConversations(ctx, userID)
	if err != nil {
		a.logger.Warn("Failed to list persisted conversations", "error", err)
		return conversations
	}

	for _, conv := range persisted {
		if activeIDs[conv.ID] {
			continue
		}
		conversations = append(conversations, ConversationWithState{
			Conversation: *conv,
			Working:      false,
		})
	}
	return conversations
}

// ConversationWithState is a conversation with its current state.
type ConversationWithState struct {
	Conversation Conversation `json:"conversation"`
	Working      bool         `json:"working"`
	Model        string       `json:"model,omitempty"`
}

// handleGetConversation returns conversation details and messages.
// GET /api/v2/agent/conversations/{id}
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
			},
		})
		return
	}

	// Fall back to store for inactive conversations
	conv, messages, ok := a.getStoredConversation(r.Context(), id, userID)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
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
	mgr := mgrValue.(*ConversationManager)
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
// POST /api/v2/agent/conversations/{id}/chat
func (a *API) handleChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getOrReactivateConversation(r.Context(), id, userID)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	model := a.selectModel(req.Model, mgr.GetModel())
	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)

	if err := mgr.AcceptUserMessage(r.Context(), a.provider, model, messageWithContext); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	a.respondJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// getOrReactivateConversation retrieves an active conversation or reactivates it from storage.
func (a *API) getOrReactivateConversation(ctx context.Context, id, userID string) (*ConversationManager, bool) {
	// Check active conversations first
	if mgr, ok := a.getActiveConversation(id, userID); ok {
		return mgr, true
	}

	// Try to reactivate from store
	return a.reactivateConversation(ctx, id, userID)
}

// reactivateConversation restores a conversation from storage and makes it active.
func (a *API) reactivateConversation(ctx context.Context, id, userID string) (*ConversationManager, bool) {
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
	})
	a.conversations.Store(id, mgr)

	return mgr, true
}

// handleStream provides SSE streaming for conversation updates.
// GET /api/v2/agent/conversations/{id}/stream
func (a *API) handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveConversation(id, userID)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	a.setupSSEHeaders(w)
	a.sendSSEMessage(w, a.buildStreamResponse(id, mgr))

	next := mgr.Subscribe(r.Context())
	for {
		resp, cont := next()
		if !cont {
			break
		}
		a.sendSSEMessage(w, resp)
	}
}

// setupSSEHeaders configures the response headers for Server-Sent Events.
func (a *API) setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// sendSSEMessage sends a single SSE message to the client.
func (a *API) sendSSEMessage(w http.ResponseWriter, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal SSE data", "error", err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// buildStreamResponse constructs the initial stream response for a conversation.
func (a *API) buildStreamResponse(id string, mgr *ConversationManager) StreamResponse {
	return StreamResponse{
		Messages:     mgr.GetMessages(),
		Conversation: ptrTo(mgr.GetConversation()),
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        mgr.IsWorking(),
			Model:          mgr.GetModel(),
		},
	}
}

// handleCancel cancels an active conversation.
// POST /api/v2/agent/conversations/{id}/cancel
func (a *API) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgr, ok := a.getActiveConversation(id, userID)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if err := mgr.Cancel(r.Context()); err != nil {
		a.logger.Error("Failed to cancel conversation", "error", err)
		http.Error(w, "Failed to cancel conversation", http.StatusInternalServerError)
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// helper function
func ptrTo[T any](v T) *T {
	return &v
}
