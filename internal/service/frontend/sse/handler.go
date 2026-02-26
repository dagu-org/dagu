package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/remotenode"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/go-chi/chi/v5"
)

const maxQueryLength = 4096

// Handler handles SSE connections for various data types.
// Each handler method builds a topic string and delegates to handleSSE.
type Handler struct {
	hub          *Hub
	remoteNodes  map[string]config.RemoteNode
	nodeResolver *remotenode.Resolver
	authService  *authservice.Service
}

// NewHandler creates a new SSE handler
func NewHandler(hub *Hub, remoteNodes map[string]config.RemoteNode, nodeResolver *remotenode.Resolver, authService *authservice.Service) *Handler {
	return &Handler{
		hub:          hub,
		remoteNodes:  remoteNodes,
		nodeResolver: nodeResolver,
		authService:  authService,
	}
}

// HandleDAGRunEvents handles SSE connections for a specific DAG run.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}
func (h *Handler) HandleDAGRunEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	h.handleSSE(w, r, buildTopic(TopicTypeDAGRun, name, dagRunID))
}

// HandleDAGEvents handles SSE connections for DAG details.
// Endpoint: GET /events/dags/{fileName}
func (h *Handler) HandleDAGEvents(w http.ResponseWriter, r *http.Request) {
	fileName := chi.URLParam(r, "fileName")
	h.handleSSE(w, r, buildTopic(TopicTypeDAG, fileName))
}

// HandleDAGHistoryEvents handles SSE connections for DAG execution history.
// Endpoint: GET /events/dags/{fileName}/dag-runs
func (h *Handler) HandleDAGHistoryEvents(w http.ResponseWriter, r *http.Request) {
	fileName := chi.URLParam(r, "fileName")
	h.handleSSE(w, r, buildTopic(TopicTypeDAGHistory, fileName))
}

// HandleDAGRunLogsEvents handles SSE connections for DAG run logs.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}/logs
func (h *Handler) HandleDAGRunLogsEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	identifier := name + "/" + dagRunID
	if q := sanitizeQueryForTopic(r.URL.RawQuery); q != "" {
		identifier += "?" + q
	}
	h.handleSSE(w, r, buildTopic(TopicTypeDAGRunLogs, identifier))
}

// HandleStepLogEvents handles SSE connections for individual step logs.
// Endpoint: GET /events/dag-runs/{name}/{dagRunId}/logs/steps/{stepName}
func (h *Handler) HandleStepLogEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dagRunID := chi.URLParam(r, "dagRunId")
	stepName := chi.URLParam(r, "stepName")
	h.handleSSE(w, r, buildTopic(TopicTypeStepLog, name, dagRunID, stepName))
}

// HandleDAGRunsListEvents handles SSE connections for the dashboard DAG runs list.
// Endpoint: GET /events/dag-runs
func (h *Handler) HandleDAGRunsListEvents(w http.ResponseWriter, r *http.Request) {
	h.handleSSE(w, r, buildTopic(TopicTypeDAGRuns, sanitizeQueryForTopic(r.URL.RawQuery)))
}

// HandleQueueItemsEvents handles SSE connections for queue items.
// Endpoint: GET /events/queues/{name}/items
func (h *Handler) HandleQueueItemsEvents(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "name")
	h.handleSSE(w, r, buildTopic(TopicTypeQueueItems, queueName))
}

// HandleQueuesListEvents handles SSE connections for the queue list.
// Endpoint: GET /events/queues
func (h *Handler) HandleQueuesListEvents(w http.ResponseWriter, r *http.Request) {
	h.handleSSE(w, r, buildTopic(TopicTypeQueues, sanitizeQueryForTopic(r.URL.RawQuery)))
}

// HandleDAGsListEvents handles SSE connections for the DAGs list.
// Endpoint: GET /events/dags
func (h *Handler) HandleDAGsListEvents(w http.ResponseWriter, r *http.Request) {
	h.handleSSE(w, r, buildTopic(TopicTypeDAGsList, sanitizeQueryForTopic(r.URL.RawQuery)))
}

// buildTopic constructs a topic string from a topic type and identifier parts.
func buildTopic(topicType TopicType, parts ...string) string {
	return fmt.Sprintf("%s:%s", topicType, strings.Join(parts, "/"))
}

// sanitizeQueryForTopic removes sensitive params (token, remoteNode) from query string
// and limits length to prevent unbounded topic keys.
func sanitizeQueryForTopic(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ""
	}

	// Remove sensitive parameters that should not be part of topic identity
	values.Del("token")
	values.Del("remoteNode")

	result := values.Encode()
	if len(result) > maxQueryLength {
		return result[:maxQueryLength]
	}
	return result
}

// handleSSE is the common SSE handling logic shared by all handlers.
// It handles auth, headers, client creation, subscription, and the write pump.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request, topic string) {
	ctx := r.Context()
	query := r.URL.Query()

	if remoteNode := query.Get("remoteNode"); remoteNode != "" && remoteNode != "local" {
		h.proxyToRemoteNode(w, r, remoteNode, topic)
		return
	}

	if !h.validateAuth(w, r) {
		return
	}

	SetSSEHeaders(w)

	client, err := NewClient(w)
	if err != nil {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if err := h.hub.Subscribe(client, topic); err != nil {
		http.Error(w, "unable to subscribe to events", http.StatusServiceUnavailable)
		return
	}
	defer h.hub.Unsubscribe(client)

	// Use json.Marshal for safe JSON encoding to prevent injection from special characters
	topicData, _ := json.Marshal(map[string]string{"topic": topic})
	client.Send(&Event{
		Type: EventTypeConnected,
		Data: string(topicData),
	})

	client.WritePump(ctx)
}

// validateAuth validates the auth token if auth service is configured.
// Returns true if authentication passed (or not required), false otherwise.
func (h *Handler) validateAuth(w http.ResponseWriter, r *http.Request) bool {
	if h.authService == nil {
		return true
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return false
	}

	if _, err := h.authService.GetUserFromToken(r.Context(), token); err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return false
	}

	return true
}
