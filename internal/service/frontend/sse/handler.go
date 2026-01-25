package sse

import (
	"fmt"
	"net/http"

	"github.com/dagu-org/dagu/internal/cmn/config"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/go-chi/chi/v5"
)

// Handler handles SSE connections for various data types.
// Each handler method builds a topic string and delegates to handleSSE.
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

// HandleDAGRunEvents handles SSE connections for a specific DAG run.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}
func (h *Handler) HandleDAGRunEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	topic := string(TopicTypeDAGRun) + ":" + name + "/" + dagRunID
	h.handleSSE(w, r, topic)
}

// HandleDAGEvents handles SSE connections for DAG details.
// Endpoint: GET /events/dags/{fileName}
func (h *Handler) HandleDAGEvents(w http.ResponseWriter, r *http.Request) {
	fileName := chi.URLParam(r, "fileName")
	topic := string(TopicTypeDAG) + ":" + fileName
	h.handleSSE(w, r, topic)
}

// HandleDAGRunLogsEvents handles SSE connections for DAG run logs.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}/logs
func (h *Handler) HandleDAGRunLogsEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	topic := string(TopicTypeDAGRunLogs) + ":" + name + "/" + dagRunID
	h.handleSSE(w, r, topic)
}

// HandleStepLogEvents handles SSE connections for individual step logs.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}/logs/steps/{stepName}
func (h *Handler) HandleStepLogEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	stepName := chi.URLParam(r, "stepName")
	topic := string(TopicTypeStepLog) + ":" + name + "/" + dagRunID + "/" + stepName
	h.handleSSE(w, r, topic)
}

// HandleDAGRunsListEvents handles SSE connections for the dashboard DAG runs list.
// Endpoint: GET /events/dag-runs
func (h *Handler) HandleDAGRunsListEvents(w http.ResponseWriter, r *http.Request) {
	// Include query params in topic for unique identification of different filters
	topic := string(TopicTypeDAGRuns) + ":" + r.URL.RawQuery
	h.handleSSE(w, r, topic)
}

// HandleQueueItemsEvents handles SSE connections for queue items.
// Endpoint: GET /events/queues/{name}/items
func (h *Handler) HandleQueueItemsEvents(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "name")
	topic := string(TopicTypeQueueItems) + ":" + queueName
	h.handleSSE(w, r, topic)
}

// handleSSE is the common SSE handling logic shared by all handlers.
// It handles auth, headers, client creation, subscription, and the write pump.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request, topic string) {
	ctx := r.Context()

	// Proxy to remote node if specified
	if remoteNode := r.URL.Query().Get("remoteNode"); remoteNode != "" && remoteNode != "local" {
		h.proxyToRemoteNode(w, r, remoteNode, topic)
		return
	}

	// Validate auth token if auth service is configured
	if h.authService != nil {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		if _, err := h.authService.GetUserFromToken(ctx, token); err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
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

	// Subscribe to hub
	if err := h.hub.Subscribe(client, topic); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer h.hub.Unsubscribe(client)

	// Send connected event
	client.Send(&Event{
		Type: EventTypeConnected,
		Data: fmt.Sprintf(`{"topic":"%s"}`, topic),
	})

	// Block on write pump until client disconnects
	client.WritePump(ctx)
}
