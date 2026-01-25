package sse

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: node.SkipTLSVerify, //nolint:gosec
			},
			MaxIdleConns:       10,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
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
		return fmt.Sprintf("%s/events/dag-runs/%s", baseURL, topic)
	}

	path := buildPathForTopic(TopicType(topicType), identifier)
	url := baseURL + path

	if token == "" {
		return url
	}

	if strings.Contains(url, "?") {
		return url + "&token=" + token
	}
	return url + "?token=" + token
}

// buildPathForTopic returns the URL path for the given topic type and identifier.
func buildPathForTopic(topicType TopicType, identifier string) string {
	switch topicType {
	case TopicTypeDAGRun:
		return "/events/dag-runs/" + identifier

	case TopicTypeDAG:
		return "/events/dags/" + identifier

	case TopicTypeDAGRunLogs:
		return "/events/dag-runs/" + identifier + "/logs"

	case TopicTypeStepLog:
		parts := strings.SplitN(identifier, "/", 3)
		if len(parts) == 3 {
			return fmt.Sprintf("/events/dag-runs/%s/%s/logs/steps/%s", parts[0], parts[1], parts[2])
		}
		return "/events/dag-runs/" + identifier + "/logs/steps"

	case TopicTypeDAGRuns:
		return pathWithOptionalQuery("/events/dag-runs", identifier)

	case TopicTypeQueueItems:
		return "/events/queues/" + identifier + "/items"

	case TopicTypeQueues:
		return pathWithOptionalQuery("/events/queues", identifier)

	case TopicTypeDAGsList:
		return pathWithOptionalQuery("/events/dags", identifier)

	default:
		return fmt.Sprintf("/events/%s/%s", topicType, identifier)
	}
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
