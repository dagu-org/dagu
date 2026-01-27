package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
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
	mu            sync.Mutex
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	Provider          llm.Provider
	Model             string
	WorkingDir        string
	Logger            *slog.Logger
	ConversationStore ConversationStore
}

// NewAPI creates a new API instance.
func NewAPI(cfg APIConfig) *API {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &API{
		provider:   cfg.Provider,
		model:      cfg.Model,
		workingDir: cfg.WorkingDir,
		logger:     logger,
		store:      cfg.ConversationStore,
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

	// Get user ID from context
	userID := getUserIDFromContext(r.Context())

	// Use request model or default
	model := req.Model
	if model == "" {
		model = a.model
	}

	// Create new conversation manager
	id := uuid.New().String()
	now := time.Now()

	// Create OnMessage callback for persistence
	var onMessage func(ctx context.Context, msg Message) error
	if a.store != nil {
		onMessage = func(ctx context.Context, msg Message) error {
			return a.store.AddMessage(ctx, id, &msg)
		}
	}

	mgr := NewConversationManager(ConversationManagerConfig{
		ID:         id,
		UserID:     userID,
		Logger:     a.logger,
		WorkingDir: a.workingDir,
		OnMessage:  onMessage,
	})

	// Persist conversation to store
	if a.store != nil {
		conv := &Conversation{
			ID:        id,
			UserID:    userID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := a.store.CreateConversation(r.Context(), conv); err != nil {
			a.logger.Error("Failed to persist conversation", "error", err)
			// Continue anyway - in-memory operation should still work
		}
	}

	a.conversations.Store(id, mgr)

	// Accept the user message
	if err := mgr.AcceptUserMessage(r.Context(), a.provider, model, req.Message); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(NewConversationResponse{
		ConversationID: id,
		Status:         "accepted",
	})
}

// handleListConversations lists all conversations for the current user.
// GET /api/v2/agent/conversations
func (a *API) handleListConversations(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r.Context())

	// Track active conversation IDs
	activeIDs := make(map[string]bool)
	var conversations []ConversationWithState

	// First, get active conversations from sync.Map
	a.conversations.Range(func(key, value any) bool {
		mgr := value.(*ConversationManager)
		// Only include conversations belonging to the current user
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

	// Then, get persisted conversations from store (if available)
	if a.store != nil {
		persisted, err := a.store.ListConversations(r.Context(), userID)
		if err != nil {
			a.logger.Warn("Failed to list persisted conversations", "error", err)
		} else {
			// Add persisted conversations that aren't active
			for _, conv := range persisted {
				if !activeIDs[conv.ID] {
					conversations = append(conversations, ConversationWithState{
						Conversation: *conv,
						Working:      false,
						Model:        "",
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversations)
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

	// First check active conversations
	if mgrValue, ok := a.conversations.Load(id); ok {
		mgr := mgrValue.(*ConversationManager)

		// Check user ownership
		if mgr.UserID() != userID {
			http.Error(w, "Conversation not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StreamResponse{
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
	if a.store == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	conv, err := a.store.GetConversation(r.Context(), id)
	if err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if conv.UserID != userID {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Get messages from store
	messages, err := a.store.GetMessages(r.Context(), id)
	if err != nil {
		a.logger.Error("Failed to get messages from store", "error", err)
		messages = []Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StreamResponse{
		Messages:     messages,
		Conversation: conv,
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        false,
			Model:          "",
		},
	})
}

// handleChat sends a message to an existing conversation.
// POST /api/v2/agent/conversations/{id}/chat
func (a *API) handleChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	var mgr *ConversationManager

	// First check if conversation is active
	if mgrValue, ok := a.conversations.Load(id); ok {
		mgr = mgrValue.(*ConversationManager)
		// Check user ownership
		if mgr.UserID() != userID {
			http.Error(w, "Conversation not found", http.StatusNotFound)
			return
		}
	} else if a.store != nil {
		// Try to reactivate from store
		conv, err := a.store.GetConversation(r.Context(), id)
		if err != nil {
			http.Error(w, "Conversation not found", http.StatusNotFound)
			return
		}

		// Verify ownership
		if conv.UserID != userID {
			http.Error(w, "Conversation not found", http.StatusNotFound)
			return
		}

		// Load existing messages for history restoration
		messages, err := a.store.GetMessages(r.Context(), id)
		if err != nil {
			a.logger.Warn("Failed to load messages for reactivation", "error", err)
			messages = []Message{}
		}

		// Get latest sequence ID
		seqID, err := a.store.GetLatestSequenceID(r.Context(), id)
		if err != nil {
			seqID = int64(len(messages))
		}

		// Create OnMessage callback for persistence
		onMessage := func(ctx context.Context, msg Message) error {
			return a.store.AddMessage(ctx, id, &msg)
		}

		// Reactivate the conversation with restored history
		mgr = NewConversationManager(ConversationManagerConfig{
			ID:         id,
			UserID:     userID,
			Logger:     a.logger,
			WorkingDir: a.workingDir,
			OnMessage:  onMessage,
			History:    messages,
			SequenceID: seqID,
		})
		a.conversations.Store(id, mgr)
	} else {
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

	// Use request model or conversation's model
	model := req.Model
	if model == "" {
		model = mgr.GetModel()
	}
	if model == "" {
		model = a.model
	}

	if err := mgr.AcceptUserMessage(r.Context(), a.provider, model, req.Message); err != nil {
		a.logger.Error("Failed to accept user message", "error", err)
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// handleStream provides SSE streaming for conversation updates.
// GET /api/v2/agent/conversations/{id}/stream
func (a *API) handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	mgr := mgrValue.(*ConversationManager)

	// Check user ownership
	if mgr.UserID() != userID {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial state
	initialData := StreamResponse{
		Messages:     mgr.GetMessages(),
		Conversation: ptrTo(mgr.GetConversation()),
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        mgr.IsWorking(),
			Model:          mgr.GetModel(),
		},
	}

	data, _ := json.Marshal(initialData)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Subscribe to updates
	next := mgr.Subscribe(r.Context())
	for {
		resp, cont := next()
		if !cont {
			break
		}

		data, _ := json.Marshal(resp)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// handleCancel cancels an active conversation.
// POST /api/v2/agent/conversations/{id}/cancel
func (a *API) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r.Context())

	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	mgr := mgrValue.(*ConversationManager)

	// Check user ownership
	if mgr.UserID() != userID {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if err := mgr.Cancel(r.Context()); err != nil {
		a.logger.Error("Failed to cancel conversation", "error", err)
		http.Error(w, "Failed to cancel conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

// CleanupOldConversations removes conversations that haven't been active for the given duration.
func (a *API) CleanupOldConversations(maxAge time.Duration) {
	threshold := time.Now().Add(-maxAge)

	a.conversations.Range(func(key, value any) bool {
		id := key.(string)
		mgr := value.(*ConversationManager)

		mgr.mu.Lock()
		lastActivity := mgr.lastActivity
		mgr.mu.Unlock()

		if lastActivity.Before(threshold) {
			mgr.Cancel(nil)
			a.conversations.Delete(id)
			a.logger.Info("Cleaned up old conversation", "id", id)
		}
		return true
	})
}

// helper function
func ptrTo[T any](v T) *T {
	return &v
}
