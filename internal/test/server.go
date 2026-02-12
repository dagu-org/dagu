package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/telemetry"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/frontend"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

// Server represents a test HTTP server instance
type Server struct {
	Helper
}

// SetupServer creates and starts a test server instance
func SetupServer(t *testing.T, opts ...HelperOption) Server {
	t.Helper()

	// Create a listener and keep it alive until the server binds.
	// This prevents race conditions where parallel tests could steal the port
	// between finding it and binding to it.
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "failed to create listener")

	port := listener.Addr().(*net.TCPAddr).Port

	opts = append(opts, WithServerConfig(
		&config.Server{
			Host: "localhost",
			Port: port,
			Permissions: map[config.Permission]bool{
				config.PermissionWriteDAGs: true,
				config.PermissionRunDAGs:   true,
			},
		},
	))

	helper := Setup(t, opts...)

	srv := Server{Helper: helper}

	// Use error channel to detect server startup failures
	errChan := make(chan error, 1)
	go srv.runServer(t, listener, errChan)

	// Wait for the server to start or fail
	select {
	case err := <-errChan:
		if err != nil {
			_ = listener.Close() // Clean up listener on failure
			t.Fatalf("server failed to start: %v", err)
		}
		// Server started successfully, wait for it to be ready
		waitForServerStart(t, fmt.Sprintf("localhost:%d", port))
	case <-time.After(10 * time.Second):
		_ = listener.Close()
		t.Fatal("server failed to start within timeout")
	}

	return srv
}

// runServer starts the HTTP server with the provided listener
func (srv *Server) runServer(t *testing.T, listener net.Listener, errChan chan<- error) {
	t.Helper()

	cc := coordinator.New(srv.ServiceRegistry, coordinator.DefaultConfig())

	collector := telemetry.NewCollector(
		config.Version,
		srv.DAGStore,
		srv.DAGRunStore,
		srv.QueueStore,
		srv.ServiceRegistry,
	)
	mr := telemetry.NewRegistry(collector)

	// Pass the pre-bound listener to the server to avoid port race conditions
	server, err := frontend.NewServer(
		srv.Context, srv.Config, srv.DAGStore, srv.DAGRunStore,
		srv.QueueStore, srv.ProcStore, srv.DAGRunMgr, cc,
		srv.ServiceRegistry, mr, collector, nil,
		frontend.WithListener(listener),
	)
	if err != nil {
		errChan <- fmt.Errorf("failed to create server: %w", err)
		return
	}
	close(errChan) // Signal that server created successfully
	err = server.Serve(srv.Context)
	if err != nil {
		// Log but don't fail - context cancellation is expected
		t.Logf("server.Serve returned: %v", err)
	}
}

// Client returns an HTTP client for the server
func (srv *Server) Client() *APIClient {
	return &APIClient{
		server: srv,
		client: resty.New(),
	}
}

// APIClient handles HTTP requests to the test server
type APIClient struct {
	server *Server
	client *resty.Client
}

// baseURL returns the base URL for the server
func (c *APIClient) baseURL() string {
	return fmt.Sprintf("http://%s:%d", c.server.Config.Server.Host, c.server.Config.Server.Port)
}

// Request represents an HTTP request being prepared
type Request struct {
	client         *APIClient
	method         string
	path           string
	body           any
	expectedStatus int
	headers        map[string]string
}

// Get prepares a GET request
func (c *APIClient) Get(path string) *Request {
	return &Request{
		client: c,
		method: http.MethodGet,
		path:   path,
	}
}

// Post prepares a POST request with the given body
func (c *APIClient) Post(path string, body any) *Request {
	return &Request{
		client: c,
		method: http.MethodPost,
		path:   path,
		body:   body,
	}
}

// Delete prepares a DELETE request
func (c *APIClient) Delete(path string) *Request {
	return &Request{
		client: c,
		method: http.MethodDelete,
		path:   path,
	}
}

// Patch prepares a PATCH request with the given body
func (c *APIClient) Patch(path string, body any) *Request {
	return &Request{
		client: c,
		method: http.MethodPatch,
		path:   path,
		body:   body,
	}
}

// Put prepares a PUT request with the given body
func (c *APIClient) Put(path string, body any) *Request {
	return &Request{
		client: c,
		method: http.MethodPut,
		path:   path,
		body:   body,
	}
}

// ExpectStatus sets the expected HTTP status code
func (r *Request) ExpectStatus(code int) *Request {
	r.expectedStatus = code
	return r
}

// WithHeader adds a header to the request
func (r *Request) WithHeader(key, value string) *Request {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
	return r
}

// WithBearerToken adds a Bearer token to the Authorization header
func (r *Request) WithBearerToken(token string) *Request {
	return r.WithHeader("Authorization", "Bearer "+token)
}

// WithBasicAuth adds Basic authentication to the request
func (r *Request) WithBasicAuth(username, password string) *Request {
	return r.WithHeader("Authorization", "Basic "+basicAuth(username, password))
}

// basicAuth returns the base64 encoded basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// Send executes the request and returns the response
func (r *Request) Send(t *testing.T) *Response {
	t.Helper()

	req := r.client.client.R()
	url := r.client.baseURL() + r.path

	// Apply custom headers
	for key, value := range r.headers {
		req.SetHeader(key, value)
	}

	var res *resty.Response
	var err error

	switch r.method {
	case http.MethodGet:
		res, err = req.Get(url)
	case http.MethodPost:
		jsonBody, jsonErr := json.Marshal(r.body)
		require.NoError(t, jsonErr, "failed to marshal request body")

		res, err = req.
			SetBody(jsonBody).
			SetHeader("Content-Type", "application/json").
			Post(url)
	case http.MethodPatch:
		jsonBody, jsonErr := json.Marshal(r.body)
		require.NoError(t, jsonErr, "failed to marshal request body")

		res, err = req.
			SetBody(jsonBody).
			SetHeader("Content-Type", "application/json").
			Patch(url)
	case http.MethodPut:
		jsonBody, jsonErr := json.Marshal(r.body)
		require.NoError(t, jsonErr, "failed to marshal request body")

		res, err = req.
			SetBody(jsonBody).
			SetHeader("Content-Type", "application/json").
			Put(url)
	case http.MethodDelete:
		res, err = req.Delete(url)
	default:
		t.Fatalf("unsupported HTTP method: %s", r.method)
	}

	require.NoError(t, err, "failed to make %s request", r.method)

	if r.expectedStatus != 0 {
		t.Logf("expected status code: %d, actual status code: %d", r.expectedStatus, res.StatusCode())
		require.Equal(t, r.expectedStatus, res.StatusCode(), "unexpected status code")
	}

	return &Response{
		Body:     string(res.Body()),
		Response: res,
	}
}

// Response represents an HTTP response
type Response struct {
	Body     string
	Response *resty.Response
}

// Unmarshal parses the response body into the provided value
func (r *Response) Unmarshal(t *testing.T, v any) {
	t.Helper()
	err := json.Unmarshal([]byte(r.Body), v)
	require.NoError(t, err, "failed to unmarshal response body")
}

// waitForServerStart polls the server until it responds or times out
func waitForServerStart(t *testing.T, addr string) {
	t.Helper()

	const (
		maxRetries = 10
		retryDelay = 100 * time.Millisecond
	)

	for range maxRetries {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(retryDelay)
	}

	t.Fatalf("server failed to start within %v", maxRetries*retryDelay)
}
