package api

import (
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/config"
)

// WithRemoteNode is a middleware that checks if the request has a "remoteNode" query parameter.
// If it does, it proxies the request to the specified remote node.
func WithRemoteNode(remoteNodes map[string]config.RemoteNode, apiBasePath string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Check if the request has a "remoteNode" query parameter
			remoteNodeName := r.URL.Query().Get("remoteNode")
			if remoteNodeName == "" || remoteNodeName == "local" {
				next.ServeHTTP(w, r)
				return
			}
			remoteNode, ok := remoteNodes[remoteNodeName]
			if !ok {
				// remote node not found, return bad request
				WriteErrorResponse(w, &Error{
					Code:       api.ErrorCodeBadRequest,
					HTTPStatus: http.StatusBadRequest,
					Message:    fmt.Sprintf("remote node %s not found", remoteNodeName),
				})
				return
			}
			// If the parameter is present, we need to handle the request differently
			// Call the handleRemoteNodeProxy function to proxy the request
			remoteNodeHandler := &remoteNodeProxy{
				remoteNode:  remoteNode,
				apiBasePath: apiBasePath,
			}
			statusCode, resp, err := remoteNodeHandler.proxy(r)
			if err != nil {
				// If there was an error, write the error response
				WriteErrorResponse(w, err)
				return
			}
			// If the status code is not 200, write the error response
			w.WriteHeader(statusCode)
			if resp != nil {
				// If we have a response, write it to the response writer
				_, err = w.Write(resp)
				if err != nil {
					// If there was an error writing the response, log it
					logger.Error(r.Context(), "failed to write response", "err", err)
				}
			}
		}

		return http.HandlerFunc(fn)
	}
}

type remoteNodeProxy struct {
	remoteNode  config.RemoteNode
	apiBasePath string
}

// handleRemoteNodeProxy checks if 'remoteNode' is present in the query parameters.
// If yes, it proxies the request to the remote node and returns the remote response.
// If not, it returns nil, indicating to proceed locally.
func (h *remoteNodeProxy) proxy(r *http.Request) (int, []byte, error) {
	// Read the request body if it exists
	var body any
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read request body: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &body); err != nil {
				return 0, nil, &Error{
					Code:       api.ErrorCodeBadRequest,
					HTTPStatus: http.StatusBadRequest,
					Message:    fmt.Sprintf("Failed to unmarshal request body: %s", err),
				}
			}
		}
	}

	// forward the request to the remote node
	return h.doRequest(body, r, h.remoteNode)
}

// doRemoteProxy performs the actual proxying of the request to the remote node.
func (h *remoteNodeProxy) doRequest(body any, r *http.Request, node config.RemoteNode) (int, []byte, error) {
	// Copy original query parameters except remoteNode
	q := r.URL.Query()
	q.Del("remoteNode")

	// Build the new remote URL
	urlComponents := strings.Split(r.URL.Path, h.apiBasePath)
	if len(urlComponents) < 2 {
		return 0, nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			HTTPStatus: http.StatusBadRequest,
			Message:    fmt.Sprintf("invalid URL path: %s", r.URL.Path),
		}
	}
	remoteURL := fmt.Sprintf("%s%s?%s", strings.TrimSuffix(node.APIBaseURL, "/"), urlComponents[1], q.Encode())

	method := r.Method
	var bodyJSON io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyJSON = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, remoteURL, bodyJSON)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Copy headers from the original request if needed
	// But we need to override authorization headers
	if node.IsBasicAuth {
		req.SetBasicAuth(node.BasicAuthUsername, node.BasicAuthPassword)
	} else if node.IsAuthToken {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", node.AuthToken))
	}
	for k, v := range r.Header {
		if k == "Authorization" {
			continue
		}
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	// Set the Accept-Encoding header to handle gzip and deflate responses
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	// Create a custom transport that skips certificate verification
	transport := &http.Transport{
		DisableCompression: true,
		TLSClientConfig: &tls.Config{
			// Allow insecure TLS connections if the remote node is configured to skip verification
			// This may be necessary for some enterprise setups
			InsecureSkipVerify: node.SkipTLSVerify, // nolint:gosec
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Add a reasonable timeout
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to send request to remote node: %w", err)
	}

	if resp == nil {
		return 0, nil, fmt.Errorf("received nil response from remote node")
	}

	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() {
			_ = gzReader.Close()
		}()
		reader = gzReader
	case "deflate":
		reader = flate.NewReader(resp.Body)
	}

	respData, err := io.ReadAll(reader)
	if err != nil {
		return 0, nil, NewAPIError(
			http.StatusBadGateway, api.ErrorCodeBadGateway, fmt.Errorf("failed to read response body: %w", err),
		)
	}

	logger.Debug(r.Context(), "received response from remote node",
		"statusCode", resp.StatusCode,
		"contentLength", resp.ContentLength,
		"contentType", resp.Header.Get("Content-Type"),
		"dataLength", len(respData))

	// If not status 200, try to parse the error response
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Only try to decode JSON if we actually got some response data
		if len(respData) > 0 {
			var remoteErr api.Error
			if err := json.Unmarshal(respData, &remoteErr); err == nil && remoteErr.Code != "" {
				return 0, nil, &Error{
					Code:       api.ErrorCodeBadGateway,
					HTTPStatus: resp.StatusCode,
					Message:    remoteErr.Message,
				}
			}
		}
		// If we can't decode a proper error or have no data, return a generic one
		return 0, nil, &Error{
			Code:       api.ErrorCodeBadGateway,
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("remote node responded with status %d", resp.StatusCode),
		}
	}

	return resp.StatusCode, respData, nil
}
