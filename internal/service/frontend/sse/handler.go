package sse

import (
	"fmt"
	"net/http"

	"github.com/dagu-org/dagu/internal/cmn/config"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/go-chi/chi/v5"
)

// Handler handles SSE connections for DAG run status updates
type Handler struct {
	hub         *Hub
	remoteNodes map[string]config.RemoteNode
	authService *authservice.Service
}

// NewHandler creates a new SSE handler
func NewHandler(hub *Hub, remoteNodes map[string]config.RemoteNode, authService *authservice.Service) *Handler {
	return &Handler{
		hub:         hub,
		remoteNodes: remoteNodes,
		authService: authService,
	}
}

// HandleDAGRunEvents handles SSE connections for a specific DAG run
func (h *Handler) HandleDAGRunEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check remoteNode parameter - proxy if not local
	remoteNode := r.URL.Query().Get("remoteNode")
	if remoteNode != "" && remoteNode != "local" {
		h.proxyToRemoteNode(w, r, remoteNode)
		return
	}

	// Auth validation (token in query for SSE compatibility)
	// Only validate if auth service is configured
	if h.authService != nil {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}
		if _, err := h.authService.GetUserFromToken(ctx, token); err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
	}

	// Set SSE headers
	SetSSEHeaders(w)

	// Create client
	client, err := NewClient(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build topic from URL params
	name := chi.URLParam(r, "name")
	dagRunId := chi.URLParam(r, "dagRunId")
	topic := fmt.Sprintf("%s/%s", name, dagRunId)

	// Subscribe to hub
	if err := h.hub.Subscribe(client, topic); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer h.hub.Unsubscribe(client)

	// Send connected event
	client.Send(&Event{
		Type: EventTypeConnected,
		Data: fmt.Sprintf(`{"topic":"%s","name":"%s","dagRunId":"%s"}`, topic, name, dagRunId),
	})

	// Block on write pump - this handles sending events to the client
	client.WritePump(ctx)
}
