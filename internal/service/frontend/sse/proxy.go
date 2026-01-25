package sse

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// proxyToRemoteNode proxies the SSE request to a remote node
func (h *Handler) proxyToRemoteNode(w http.ResponseWriter, r *http.Request, nodeName string) {
	node, ok := h.remoteNodes[nodeName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown remote node: %s", nodeName), http.StatusBadRequest)
		return
	}

	// Build remote SSE URL
	name := chi.URLParam(r, "name")
	dagRunId := chi.URLParam(r, "dagRunId")
	token := r.URL.Query().Get("token")

	remoteURL := fmt.Sprintf("%s/events/dag-runs/%s/%s",
		strings.TrimSuffix(node.APIBaseURL, "/"),
		name,
		dagRunId,
	)
	if token != "" {
		remoteURL += "?token=" + token
	}

	// Create request with client's context (auto-cancel on disconnect)
	req, err := http.NewRequestWithContext(r.Context(), "GET", remoteURL, nil)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Set SSE accept header
	req.Header.Set("Accept", "text/event-stream")

	// Add authentication
	if node.IsBasicAuth {
		req.SetBasicAuth(node.BasicAuthUsername, node.BasicAuthPassword)
	} else if node.IsAuthToken {
		req.Header.Set("Authorization", "Bearer "+node.AuthToken)
	}

	// Create HTTP client with TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: node.SkipTLSVerify, //nolint:gosec
		},
	}
	client := &http.Client{Transport: transport}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to remote node: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Remote node returned status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	// Set SSE headers for the client
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream data from remote to client
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return // Client disconnected
			}
			flusher.Flush()
		}
		if err != nil {
			return // Connection closed (remote or context cancelled)
		}
	}
}
