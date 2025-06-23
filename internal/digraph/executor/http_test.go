package executor

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPExecutor_SkipTLSVerify(t *testing.T) {
	t.Run("HTTPS request with self-signed certificate", func(t *testing.T) {
		// Create a test server with a self-signed certificate
		server := httptest.NewTLSServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "success",
			})
		}))
		defer server.Close()

		// Test with skipTLSVerify = true (should succeed)
		step := digraph.Step{
			Command: "GET",
			Args:    []string{server.URL + "/test"},
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "http",
				Config: map[string]any{
					"skipTLSVerify": true,
					"silent":        true,
					"json":          true,
				},
			},
		}

		executor, err := newHTTP(context.Background(), step)
		require.NoError(t, err)

		httpExec, ok := executor.(*http)
		require.True(t, ok)
		httpExec.SetStdout(&testWriter{})
		httpExec.SetStderr(&testWriter{})

		err = httpExec.Run(context.Background())
		assert.NoError(t, err)
	})

	t.Run("HTTPS request without skipTLSVerify", func(t *testing.T) {
		// Create a test server with a self-signed certificate
		server := httptest.NewTLSServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "success",
			})
		}))
		defer server.Close()

		// Test with skipTLSVerify = false (should fail due to certificate verification)
		step := digraph.Step{
			Command: "GET",
			Args:    []string{server.URL + "/test"},
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "http",
				Config: map[string]any{
					"skipTLSVerify": false,
					"silent":        true,
				},
			},
		}

		executor, err := newHTTP(context.Background(), step)
		require.NoError(t, err)

		httpExec, ok := executor.(*http)
		require.True(t, ok)
		httpExec.SetStdout(&testWriter{})
		httpExec.SetStderr(&testWriter{})

		err = httpExec.Run(context.Background())
		// Should fail with certificate verification error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate")
	})

	t.Run("Config parsing with skipTLSVerify", func(t *testing.T) {
		step := digraph.Step{
			Command: "GET",
			Args:    []string{"https://example.com"},
			Script: `{
				"skipTLSVerify": true,
				"timeout": 30,
				"headers": {"Authorization": "Bearer token"}
			}`,
		}

		executor, err := newHTTP(context.Background(), step)
		require.NoError(t, err)

		httpExec, ok := executor.(*http)
		require.True(t, ok)
		assert.True(t, httpExec.cfg.SkipTLSVerify)
		assert.Equal(t, 30, httpExec.cfg.Timeout)
		assert.Equal(t, "Bearer token", httpExec.cfg.Headers["Authorization"])
	})
}

func TestHTTPExecutor_StandardFeatures(t *testing.T) {
	t.Run("GET request with headers and query params", func(t *testing.T) {
		server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))

			w.WriteHeader(nethttp.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		step := digraph.Step{
			Command: "GET",
			Args:    []string{server.URL},
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "http",
				Config: map[string]any{
					"headers": map[string]string{
						"Authorization": "Bearer test-token",
					},
					"query": map[string]string{
						"param1": "value1",
					},
					"silent": true,
				},
			},
		}

		executor, err := newHTTP(context.Background(), step)
		require.NoError(t, err)

		out := &testWriter{}
		httpExec, ok := executor.(*http)
		require.True(t, ok)
		httpExec.SetStdout(out)
		httpExec.SetStderr(&testWriter{})

		err = httpExec.Run(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "test response", out.String())
	})
}
