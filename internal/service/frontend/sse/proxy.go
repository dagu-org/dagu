package sse

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
)

// proxyToRemoteNode proxies the SSE request to a remote node.
// The topic parameter contains the full topic string (e.g., "dagrun:mydag/run123").
func (h *Handler) proxyToRemoteNode(w http.ResponseWriter, r *http.Request, nodeName, topic string) {
	node, ok := h.remoteNodes[nodeName]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown remote node: %s", nodeName), http.StatusBadRequest)
		return
	}

	remoteURL := buildRemoteURL(node.APIBaseURL, topic, r.URL.Query().Get("token"))

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, remoteURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	applyNodeAuth(req, node)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: node.SkipTLSVerify, //nolint:gosec
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to connect to remote node: %v", err), http.StatusBadGateway)
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
		// Fallback: treat entire topic as identifier for dagrun type
		return fmt.Sprintf("%s/events/dag-runs/%s", baseURL, topic)
	}

	var path string
	switch TopicType(topicType) {
	case TopicTypeDAGRun:
		// identifier: "name/dagRunId"
		path = fmt.Sprintf("/events/dag-runs/%s", identifier)

	case TopicTypeDAG:
		// identifier: "fileName"
		path = fmt.Sprintf("/events/dags/%s", identifier)

	case TopicTypeDAGRunLogs:
		// identifier: "name/dagRunId"
		path = fmt.Sprintf("/events/dag-runs/%s/logs", identifier)

	case TopicTypeStepLog:
		// identifier: "name/dagRunId/stepName"
		parts := strings.SplitN(identifier, "/", 3)
		if len(parts) == 3 {
			path = fmt.Sprintf("/events/dag-runs/%s/%s/logs/steps/%s", parts[0], parts[1], parts[2])
		} else {
			path = fmt.Sprintf("/events/dag-runs/%s/logs/steps", identifier)
		}

	case TopicTypeDAGRuns:
		// identifier: URL query string (e.g., "limit=50&offset=0")
		if identifier != "" {
			path = "/events/dag-runs?" + identifier
		} else {
			path = "/events/dag-runs"
		}

	case TopicTypeQueueItems:
		// identifier: "queueName"
		path = fmt.Sprintf("/events/queues/%s/items", identifier)

	case TopicTypeQueues:
		// identifier: URL query string
		if identifier != "" {
			path = "/events/queues?" + identifier
		} else {
			path = "/events/queues"
		}

	case TopicTypeDAGsList:
		// identifier: URL query string (e.g., "page=1&perPage=100&name=mydag")
		if identifier != "" {
			path = "/events/dags?" + identifier
		} else {
			path = "/events/dags"
		}

	default:
		// Unknown topic type - try to use as-is
		path = fmt.Sprintf("/events/%s/%s", topicType, identifier)
	}

	url := baseURL + path

	// Add token if provided (and not already in query string for dagruns)
	if token != "" {
		if strings.Contains(url, "?") {
			url += "&token=" + token
		} else {
			url += "?token=" + token
		}
	}

	return url
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
				return
			}
			flusher.Flush()
		}
		if readErr != nil {
			return
		}
	}
}
