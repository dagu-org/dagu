package api

import (
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/remotenode"
)

// WithRemoteNode is a middleware that checks if the request has a "remoteNode" query parameter.
// If it does, it proxies the request to the specified remote node.
// It tries the resolver first (for dynamic store nodes), then falls back to the static map.
func WithRemoteNode(resolver *remotenode.Resolver, remoteNodes map[string]config.RemoteNode, apiBasePath string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Check if the request has a "remoteNode" query parameter
			remoteNodeName := r.URL.Query().Get("remoteNode")
			if remoteNodeName == "" || remoteNodeName == "local" {
				next.ServeHTTP(w, r)
				return
			}

			// Try resolver first (covers both config and store nodes)
			var remoteNode config.RemoteNode
			if resolver != nil {
				cn, err := resolver.GetByName(r.Context(), remoteNodeName)
				if err == nil {
					remoteNode = *cn
				} else if errors.Is(err, remotenode.ErrRemoteNodeNotFound) {
					WriteErrorResponse(w, &Error{
						HTTPStatus: http.StatusBadRequest,
						Code:       api.ErrorCodeBadRequest,
						Message:    fmt.Sprintf("remote node %s not found", remoteNodeName),
					})
					return
				} else {
					WriteErrorResponse(w, &Error{
						HTTPStatus: http.StatusInternalServerError,
						Code:       api.ErrorCodeInternalError,
						Message:    fmt.Sprintf("failed to resolve remote node %s", remoteNodeName),
					})
					return
				}
			} else if rn, ok := remoteNodes[remoteNodeName]; ok {
				remoteNode = rn
			} else {
				WriteErrorResponse(w, &Error{
					HTTPStatus: http.StatusBadRequest,
					Code:       api.ErrorCodeBadRequest,
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
			resp, err := remoteNodeHandler.proxy(r)
			if err != nil {
				// If there was an error, write the error response
				WriteErrorResponse(w, err)
				return
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
					WriteErrorResponse(w, &Error{
						Code:       api.ErrorCodeBadGateway,
						HTTPStatus: http.StatusBadGateway,
						Message:    fmt.Sprintf("failed to create gzip reader: %s", err.Error()),
					})
					return
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
				WriteErrorResponse(w, &Error{
					Code:       api.ErrorCodeBadGateway,
					HTTPStatus: http.StatusBadGateway,
					Message:    fmt.Sprintf("failed to read response body: %s", err.Error()),
				})
				return
			}

			logger.Info(r.Context(), "Received response from remote node",
				slog.Int("status-code", resp.StatusCode),
				slog.Int64("content-length", resp.ContentLength),
				slog.String("content-type", resp.Header.Get("Content-Type")),
				slog.Int("data-length", len(respData)))

			// If not status 200, try to parse the error response
			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				// Only try to decode JSON if we actually got some response data
				if len(respData) > 0 {
					var remoteErr api.Error
					if err := json.Unmarshal(respData, &remoteErr); err == nil && remoteErr.Code != "" {
						WriteErrorResponse(w, &Error{
							Code:       api.ErrorCodeBadGateway,
							HTTPStatus: resp.StatusCode,
							Message:    remoteErr.Message,
						})
						return
					}
				}
				// If we can't decode a proper error or have no data, return a generic one
				WriteErrorResponse(w, &Error{
					Code:       api.ErrorCodeBadGateway,
					HTTPStatus: resp.StatusCode,
					Message:    fmt.Sprintf("remote node responded with status %d", resp.StatusCode),
				})
				return
			}

			// If the status code is not 200, write the error response
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			if resp != nil {
				// If we have a response, write it to the response writer
				_, err = w.Write(respData)
				if err != nil {
					// If there was an error writing the response, log it
					logger.Error(r.Context(), "Failed to write response", tag.Error(err))
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
func (h *remoteNodeProxy) proxy(r *http.Request) (*http.Response, error) {
	// Read the request body if it exists
	var body any
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &body); err != nil {
				return nil, &Error{
					HTTPStatus: http.StatusBadRequest,
					Code:       api.ErrorCodeBadRequest,
					Message:    fmt.Sprintf("failed to unmarshal request body: %s", err.Error()),
				}
			}
		}
	}
	// forward the request to the remote node
	return h.doRequest(body, r, h.remoteNode)
}

// doRemoteProxy performs the actual proxying of the request to the remote node.
func (h *remoteNodeProxy) doRequest(body any, r *http.Request, node config.RemoteNode) (*http.Response, error) {
	// Copy original query parameters except remoteNode
	q := r.URL.Query()
	q.Del("remoteNode")

	// Build the new remote URL
	urlComponents := strings.Split(r.URL.Path, h.apiBasePath)
	if len(urlComponents) < 2 {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			HTTPStatus: http.StatusBadRequest,
			Message:    fmt.Sprintf("invalid URL path: %s", r.URL.Path),
		}
	}
	remoteURL := fmt.Sprintf("%s%s", strings.TrimSuffix(node.APIBaseURL, "/"), urlComponents[1])
	if params := q.Encode(); params != "" {
		remoteURL += "?" + params
	}

	method := r.Method
	var bodyJSON io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyJSON = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, remoteURL, bodyJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
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
	// Add application/json content type
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

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
		return nil, fmt.Errorf("failed to send request to remote node: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("received nil response from remote node")
	}

	return resp, nil
}
