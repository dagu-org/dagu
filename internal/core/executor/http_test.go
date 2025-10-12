package executor

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPExecutor_SkipTLSVerify(t *testing.T) {
	t.Run("HTTPSRequestWithSelfSignedCertificate", func(t *testing.T) {
		// Create a test server with a self-signed certificate
		server := httptest.NewTLSServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "success",
			})
		}))
		defer server.Close()

		// Test with skipTLSVerify = true (should succeed)
		step := core.Step{
			Command: "GET",
			Args:    []string{server.URL + "/test"},
			ExecutorConfig: core.ExecutorConfig{
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

	t.Run("HTTPSRequestWithoutSkipTLSVerify", func(t *testing.T) {
		// Create a test server with a self-signed certificate
		server := httptest.NewTLSServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "success",
			})
		}))
		defer server.Close()

		// Test with skipTLSVerify = false (should fail due to certificate verification)
		step := core.Step{
			Command: "GET",
			Args:    []string{server.URL + "/test"},
			ExecutorConfig: core.ExecutorConfig{
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

	t.Run("ConfigParsingWithSkipTLSVerify", func(t *testing.T) {
		step := core.Step{
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
	t.Run("GETRequestWithHeadersAndQueryParams", func(t *testing.T) {
		server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))

			w.WriteHeader(nethttp.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		step := core.Step{
			Command: "GET",
			Args:    []string{server.URL},
			ExecutorConfig: core.ExecutorConfig{
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

// TestHTTPExecutor_CrossPlatform tests behavior across different platforms
func TestHTTPExecutor_CrossPlatform(t *testing.T) {
	t.Run("BehaviorConsistencyAcrossPlatforms", func(t *testing.T) {
		// Test data that should behave the same on all platforms
		testCases := []struct {
			name   string
			method string
			config map[string]any
		}{
			{
				name:   "SimpleGET",
				method: "GET",
				config: map[string]any{"silent": true},
			},
			{
				name:   "POSTWithJSON",
				method: "POST",
				config: map[string]any{
					"headers": map[string]string{"Content-Type": "application/json"},
					"body":    `{"test": "data"}`,
					"silent":  true,
				},
			},
			{
				name:   "GETWithCustomHeaders",
				method: "GET",
				config: map[string]any{
					"headers": map[string]string{
						"User-Agent":    "Dagu-Test/1.0",
						"Accept":        "application/json",
						"Custom-Header": "cross-platform-test",
					},
					"silent": true,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
					// Echo back some request info
					response := map[string]any{
						"method":   r.Method,
						"platform": runtime.GOOS,
						"headers":  r.Header,
						"url":      r.URL.String(),
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(nethttp.StatusOK)
					_ = json.NewEncoder(w).Encode(response)
				}))
				defer server.Close()

				step := core.Step{
					Command: tc.method,
					Args:    []string{server.URL},
					ExecutorConfig: core.ExecutorConfig{
						Type:   "http",
						Config: tc.config,
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

				// Parse response to verify it contains expected data
				var response map[string]any
				err = json.Unmarshal([]byte(out.String()), &response)
				assert.NoError(t, err)
				assert.Equal(t, tc.method, response["method"])
				assert.Equal(t, runtime.GOOS, response["platform"])

				// Log platform-specific information for comparison
				t.Logf("Platform: %s, Method: %s, Response length: %d",
					runtime.GOOS, tc.method, len(out.String()))
			})
		}
	})

	t.Run("JSONResponseFormattingConsistency", func(t *testing.T) {
		server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Test-Header", "cross-platform")
			w.WriteHeader(nethttp.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message":  "JSON response test",
				"platform": runtime.GOOS,
			})
		}))
		defer server.Close()

		step := core.Step{
			Command: "GET",
			Args:    []string{server.URL},
			ExecutorConfig: core.ExecutorConfig{
				Type: "http",
				Config: map[string]any{
					"json":   true,
					"silent": false, // Don't suppress headers for this test
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

		// Verify JSON response structure is consistent
		var jsonResponse httpJSONResult
		err = json.Unmarshal([]byte(out.String()), &jsonResponse)
		assert.NoError(t, err)
		assert.Equal(t, 200, jsonResponse.StatusCode)
		assert.NotEmpty(t, jsonResponse.Headers)
		assert.Contains(t, jsonResponse.Headers, "Content-Type")
		assert.Contains(t, jsonResponse.Headers, "X-Test-Header")

		// Verify response body structure
		bodyMap, ok := jsonResponse.Body.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "JSON response test", bodyMap["message"])
		assert.Equal(t, runtime.GOOS, bodyMap["platform"])

		t.Logf("Platform: %s, JSON response verified", runtime.GOOS)
	})

	t.Run("ErrorHandlingConsistency", func(t *testing.T) {
		server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			w.WriteHeader(nethttp.StatusInternalServerError)
			_, _ = w.Write([]byte("Server error for cross-platform test"))
		}))
		defer server.Close()

		step := core.Step{
			Command: "GET",
			Args:    []string{server.URL},
			ExecutorConfig: core.ExecutorConfig{
				Type: "http",
				Config: map[string]any{
					"silent": false,
				},
			},
		}

		executor, err := newHTTP(context.Background(), step)
		require.NoError(t, err)

		out := &testWriter{}
		stderr := &testWriter{}
		httpExec, ok := executor.(*http)
		require.True(t, ok)
		httpExec.SetStdout(out)
		httpExec.SetStderr(stderr)

		err = httpExec.Run(context.Background())

		// Error handling should be consistent across platforms
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http status code not 2xx")
		assert.Contains(t, err.Error(), "500")

		// Output should contain status information
		output := out.String()
		assert.Contains(t, output, "500")
		assert.Contains(t, output, "Internal Server Error")

		t.Logf("Platform: %s, Error handling verified", runtime.GOOS)
	})
}

type testWriter struct {
	data []byte
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.data = append(tw.data, p...)
	return len(p), nil
}

func (tw *testWriter) String() string {
	return string(tw.data)
}
