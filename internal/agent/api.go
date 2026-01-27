package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// API handles HTTP requests for the agent.
type API struct {
	conversations sync.Map // id -> *ConversationManager
	provider      llm.Provider
	model         string
	workingDir    string
	logger        *slog.Logger
	mu            sync.Mutex
}

// APIConfig contains configuration for the API.
type APIConfig struct {
	Provider   llm.Provider
	Model      string
	WorkingDir string
	Logger     *slog.Logger
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
	}
}

// RegisterRoutes registers the agent API routes on the given router.
func (a *API) RegisterRoutes(r chi.Router) {
	r.Route("/api/v2/agent", func(r chi.Router) {
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

	// Use request model or default
	model := req.Model
	if model == "" {
		model = a.model
	}

	// Create new conversation manager
	id := uuid.New().String()
	mgr := NewConversationManager(ConversationManagerConfig{
		ID:         id,
		Logger:     a.logger,
		WorkingDir: a.workingDir,
	})

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

// handleListConversations lists all active conversations.
// GET /api/v2/agent/conversations
func (a *API) handleListConversations(w http.ResponseWriter, r *http.Request) {
	var conversations []ConversationWithState

	a.conversations.Range(func(key, value any) bool {
		mgr := value.(*ConversationManager)
		conversations = append(conversations, ConversationWithState{
			Conversation: mgr.GetConversation(),
			Working:      mgr.IsWorking(),
			Model:        mgr.GetModel(),
		})
		return true
	})

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
	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	mgr := mgrValue.(*ConversationManager)

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
}

// handleChat sends a message to an existing conversation.
// POST /api/v2/agent/conversations/{id}/chat
func (a *API) handleChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mgrValue, ok := a.conversations.Load(id)
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

	mgr := mgrValue.(*ConversationManager)

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
	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	mgr := mgrValue.(*ConversationManager)

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
	mgrValue, ok := a.conversations.Load(id)
	if !ok {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	mgr := mgrValue.(*ConversationManager)

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
