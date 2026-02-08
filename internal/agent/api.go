package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/core/exec"
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
	conversations  sync.Map // id -> *ConversationManager (active conversations)
	store          ConversationStore
	configStore    ConfigStore
	workingDir     string
	logger         *slog.Logger
	dagStore       exec.DAGStore // For resolving DAG file paths
	namespaceStore exec.NamespaceStore
	environment    EnvironmentInfo
	hooks          *Hooks
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	ConfigStore       ConfigStore
	WorkingDir        string
	Logger            *slog.Logger
	ConversationStore ConversationStore
	DAGStore          exec.DAGStore // For resolving DAG file paths
	NamespaceStore    exec.NamespaceStore
	Environment       EnvironmentInfo
	Hooks             *Hooks
}

// ConversationWithState is a conversation with its current state.
type ConversationWithState struct {
	Conversation Conversation `json:"conversation"`
	Working      bool         `json:"working"`
	Model        string       `json:"model,omitempty"`
}

// NewAPI creates a new API instance.
func NewAPI(cfg APIConfig) *API {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &API{
		configStore:    cfg.ConfigStore,
		workingDir:     cfg.WorkingDir,
		logger:         logger,
		store:          cfg.ConversationStore,
		dagStore:       cfg.DAGStore,
		namespaceStore: cfg.NamespaceStore,
		environment:    cfg.Environment,
		hooks:          cfg.Hooks,
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
func (a *API) persistNewConversation(ctx context.Context, id, userID, namespace string, now time.Time) {
	if a.store == nil {
		return
	}
	conv := &Conversation{
		ID:        id,
		UserID:    userID,
		Namespace: namespace,
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Invalid JSON")
		return
	}

	if req.Message == "" {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Message is required")
		return
	}

	// Enforce namespace-scoped access
	nsRole := a.resolveNamespaceRole(r.Context(), req.Namespace)
	if req.Namespace != "" && nsRole == "" {
		a.respondError(w, http.StatusForbidden, api.ErrorCodeForbidden, "No access to namespace: "+req.Namespace)
		return
	}

	provider, configModel, err := a.configStore.GetProvider(r.Context())
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		a.respondError(w, http.StatusServiceUnavailable, api.ErrorCodeInternalError, "Agent is not configured properly")
		return
	}

	userID, username, ipAddress := getUserContextFromRequest(r)
	model := selectModel(req.Model, "", configModel)
	id := uuid.New().String()
	now := time.Now()

	env := a.environmentForNamespace(req.Namespace)

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:            id,
		UserID:        userID,
		Logger:        a.logger,
		WorkingDir:    env.DAGsDir,
		OnMessage:     a.createMessageCallback(id),
		Environment:   env,
		SafeMode:      req.SafeMode,
		Hooks:         a.hooks,
		Username:      username,
		IPAddress:     ipAddress,
		NamespaceRole: nsRole,
	})

	a.persistNewConversation(r.Context(), id, userID, req.Namespace, now)
	a.conversations.Store(id, mgr)

	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)
	if err := mgr.AcceptUserMessage(r.Context(), provider, model, messageWithContext); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Invalid JSON")
		return
	}

	if req.Message == "" {
		a.respondError(w, http.StatusBadRequest, api.ErrorCodeBadRequest, "Message is required")
		return
	}

	provider, configModel, err := a.configStore.GetProvider(r.Context())
	if err != nil {
		a.logger.Error("Failed to get LLM provider", "error", err)
		a.respondError(w, http.StatusServiceUnavailable, api.ErrorCodeInternalError, "Agent is not configured properly")
		return
	}

	model := selectModel(req.Model, mgr.GetModel(), configModel)
	messageWithContext := a.formatMessage(r.Context(), req.Message, req.DAGContexts)

	// Update safe mode setting per request (allows toggling mid-conversation)
	mgr.SetSafeMode(req.SafeMode)

	if err := mgr.AcceptUserMessage(r.Context(), provider, model, messageWithContext); err != nil {
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
// Uses the stored conversation's namespace to scope the environment (namespace locking).
func (a *API) reactivateConversation(ctx context.Context, id, userID, username, ipAddress string) (*ConversationManager, bool) {
	if a.store == nil {
		return nil, false
	}

	conv, err := a.store.GetConversation(ctx, id)
	if err != nil || conv == nil || conv.UserID != userID {
		return nil, false
	}

	// Check namespace access for the stored conversation's namespace
	nsRole := a.resolveNamespaceRole(ctx, conv.Namespace)
	if conv.Namespace != "" && nsRole == "" {
		a.logger.Warn("User lacks access to conversation namespace", "namespace", conv.Namespace, "user", userID)
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

	// Use the stored conversation's namespace for environment (namespace locking)
	env := a.environmentForNamespace(conv.Namespace)

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:            id,
		UserID:        userID,
		Logger:        a.logger,
		WorkingDir:    env.DAGsDir,
		OnMessage:     a.createMessageCallback(id),
		History:       messages,
		SequenceID:    seqID,
		Environment:   env,
		Hooks:         a.hooks,
		Username:      username,
		IPAddress:     ipAddress,
		NamespaceRole: nsRole,
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

	for {
		resp, cont := next()
		if !cont {
			break
		}
		a.sendSSEMessage(w, resp)
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
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
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

// environmentForNamespace returns an EnvironmentInfo scoped to the given namespace.
// If namespace is empty or "default", returns the base environment with Namespace set.
// For non-default namespaces, looks up the namespace ID and adjusts paths.
func (a *API) environmentForNamespace(namespace string) EnvironmentInfo {
	env := a.environment
	env.Namespace = namespace

	if namespace == "" {
		a.logger.Warn("environmentForNamespace called with empty namespace")
	}
	if namespace == "" || namespace == "default" || a.namespaceStore == nil {
		return env
	}

	ns, err := a.namespaceStore.Get(context.Background(), namespace)
	if err != nil {
		a.logger.Warn("Failed to resolve namespace for agent", "namespace", namespace, "error", err)
		return env
	}

	env.DAGsDir = exec.NamespaceDir(a.environment.DAGsDir, ns.ID)
	env.DataDir = exec.NamespaceDir(a.environment.DataDir, ns.ID)
	env.WorkingDir = env.DAGsDir
	return env
}

// resolveNamespaceRole returns the user's effective role in the given namespace.
// Returns an empty string if: auth is disabled (no user in context), namespace is empty,
// or the user has no access to the namespace. When auth is disabled, returns "admin"
// for backward compatibility.
func (a *API) resolveNamespaceRole(ctx context.Context, namespace string) string {
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		return string(auth.RoleAdmin) // Auth disabled: full access
	}
	if namespace == "" {
		return string(user.Role) // No namespace: use global role
	}
	role := user.EffectiveRoleInNamespace(namespace)
	if !role.Valid() {
		return ""
	}
	return string(role)
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
