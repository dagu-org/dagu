package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test" // provides testSetup helper
	"github.com/stretchr/testify/require"
)

// TestServer_BasePath verifies that when BasePath is set in the configuration,
// the API endpoints are served under that base path and not on the root.
func TestServer_BasePath(t *testing.T) {
	// find an available port
	port := findPort(t)

	// Create a temporary config file with BasePath set to "/dagu"
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	// The YAML configuration sets host, port, and basePath.
	// (Other config fields use default values.)
	configContent := fmt.Sprintf(`host: "127.0.0.1"
port: %s
basePath: "/dagu"
`, port)
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Use the provided test helper to set up context and cancellation.
	th := test.SetupCommand(t) // from github.com/dagu-org/dagu/internal/test

	// Cancel the test context after a delay so the server doesn’t run forever.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		th.Cancel()
	}()

	// Start the server in a goroutine using the temporary config.
	// The command-line arguments override the port and point to our config file.
	go func() {
		th.RunCommand(t, cmd.CmdServer(), test.CmdTest{
			Args:        []string{"server", "--config", configFile, "--port=" + port},
			ExpectedOut: []string{"Serving"},
		})
	}()

	// Wait a moment for the server to start.
	time.Sleep(500 * time.Millisecond)

	// When the config's BasePath is "/dagu", the health endpoint (normally at "/api/v1/health")
	// should be available at "/dagu/api/v1/health" and NOT at "/api/v1/health".

	// 1. Request without the base path should return 404.
	resp, err := http.Get("http://127.0.0.1:" + port + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// 2. Request with the base path should return 200.
	resp, err = http.Get("http://127.0.0.1:" + port + "/dagu/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Optionally, decode the JSON response to check for expected health status.
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var healthResp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &healthResp))
	require.Equal(t, "healthy", healthResp.Status)
}

// findPort finds an available port.
func findPort(t *testing.T) string {
	t.Helper()
	tcpListener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := tcpListener.Addr().(*net.TCPAddr).Port
	_ = tcpListener.Close()
	return strconv.Itoa(port)
}
