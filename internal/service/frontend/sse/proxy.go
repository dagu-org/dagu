package sse

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
)

// proxyToRemoteNode proxies the SSE request to a remote node.
// The topic parameter contains the full topic string (e.g., "dagrun:mydag/run123").
func (h *Handler) proxyToRemoteNode(w http.ResponseWriter, r *http.Request, nodeName, topic string) {
	cn, err := h.nodeResolver.GetByName(r.Context(), nodeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("unknown remote node: %s", nodeName), http.StatusBadRequest)
		return
	}
	node := *cn

	remoteURL := buildRemoteURL(node.APIBaseURL, topic, r.URL.Query().Get("token"))

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, remoteURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	applyNodeAuth(req, node)

	client := &http.Client{
		// Timeout: 0 is safe for SSE because:
		// 1. Request is created with r.Context() which is cancelled when client disconnects
		// 2. client.Do() will return with context.Canceled error when that happens
		Timeout: 0,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: node.SkipTLSVerify, //nolint:gosec
				MinVersion:         tls.VersionTLS12,
			},
			MaxIdleConns:       10,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if r.Context().Err() != nil {
			// Client cancelled or connection closed - don't write error
			return
		}
		http.Error(w, "failed to connect to remote node", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("remote node returned status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	SetSSEHeaders(w)
	streamResponse(w, flusher, resp.Body)
}

// buildRemoteURL constructs the SSE URL for a remote node based on the topic.
// Topic format: "type:identifier"
func buildRemoteURL(baseURL, topic, token string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")

	topicType, identifier, ok := strings.Cut(topic, ":")
	if !ok {
		return fmt.Sprintf("%s/events/dag-runs/%s", baseURL, topic)
	}

	path := buildPathForTopic(TopicType(topicType), identifier)
	result := baseURL + path

	if token == "" {
		return result
	}

	// URL-encode the token to handle special characters
	encodedToken := url.QueryEscape(token)
	if strings.Contains(result, "?") {
		return result + "&token=" + encodedToken
	}
	return result + "?token=" + encodedToken
}

// buildPathForTopic returns the URL path for the given topic type and identifier.
func buildPathForTopic(topicType TopicType, identifier string) string {
	switch topicType {
	case TopicTypeDAGRun:
		return "/events/dag-runs/" + identifier
	case TopicTypeDAG:
		return "/events/dags/" + identifier
	case TopicTypeDAGRunLogs:
		return buildDAGRunLogsPath(identifier)
	case TopicTypeStepLog:
		return buildStepLogPath(identifier)
	case TopicTypeDAGRuns:
		return pathWithOptionalQuery("/events/dag-runs", identifier)
	case TopicTypeQueueItems:
		return "/events/queues/" + identifier + "/items"
	case TopicTypeQueues:
		return pathWithOptionalQuery("/events/queues", identifier)
	case TopicTypeDAGsList:
		return pathWithOptionalQuery("/events/dags", identifier)
	case TopicTypeDAGHistory:
		return "/events/dags/" + identifier + "/dag-runs"
	default:
		return fmt.Sprintf("/events/%s/%s", topicType, identifier)
	}
}

// buildStepLogPath constructs the path for step log events.
// Expected identifier format: "dagName/dagRunId/stepName"
func buildStepLogPath(identifier string) string {
	parts := strings.SplitN(identifier, "/", 3)
	if len(parts) == 3 {
		return fmt.Sprintf("/events/dag-runs/%s/%s/logs/steps/%s", parts[0], parts[1], parts[2])
	}
	return "/events/dag-runs/" + identifier + "/logs/steps"
}

// buildDAGRunLogsPath constructs the path for DAG run logs events.
// Handles identifiers that may include query params (e.g., "name/dagRunId?tail=1000").
func buildDAGRunLogsPath(identifier string) string {
	pathPart, query, hasQuery := strings.Cut(identifier, "?")
	basePath := "/events/dag-runs/" + pathPart + "/logs"
	if hasQuery {
		return basePath + "?" + query
	}
	return basePath
}

// pathWithOptionalQuery appends a query string to the path if provided.
func pathWithOptionalQuery(basePath, query string) string {
	if query != "" {
		return basePath + "?" + query
	}
	return basePath
}

// applyNodeAuth adds authentication headers based on node configuration.
func applyNodeAuth(req *http.Request, node config.RemoteNode) {
	switch {
	case node.IsBasicAuth:
		req.SetBasicAuth(node.BasicAuthUsername, node.BasicAuthPassword)
	case node.IsAuthToken:
		req.Header.Set("Authorization", "Bearer "+node.AuthToken)
	}
}

// streamResponse copies data from the response body to the client.
func streamResponse(w http.ResponseWriter, flusher http.Flusher, body io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				_, _ = io.Copy(io.Discard, body) // Drain remaining to allow connection reuse
				return
			}
			flusher.Flush()
		}
		if readErr != nil {
			return
		}
	}
}
