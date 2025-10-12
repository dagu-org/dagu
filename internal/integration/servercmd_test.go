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
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestServer_BasePath verifies that when BasePath is set in the configuration,
// the API endpoints are served under that base path and not on the root.
func TestServer_BasePath(t *testing.T) {
	port := findPort(t)
	configFile := writeServerConfig(t, port, "/dagu", false)
	stopServer := startServer(t, configFile, port)

	requireHealthy(t, fmt.Sprintf("http://127.0.0.1:%s/dagu/api/v2/health", port))

	stopServer()
}

// TestServer_RemoteNode verifies that remote node health checks work with and without a base path.
func TestServer_RemoteNode(t *testing.T) {
	testCases := []struct {
		name     string
		basePath string
	}{
		{name: "root", basePath: ""},
		{name: "with base path", basePath: "/dagu"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port := findPort(t)
			configFile := writeServerConfig(t, port, tc.basePath, true)
			stopServer := startServer(t, configFile, port)

			url := fmt.Sprintf("http://127.0.0.1:%s%s/api/v2/health?remoteNode=dev", port, tc.basePath)
			requireHealthy(t, url)

			stopServer()
		})
	}
}

func writeServerConfig(t *testing.T, port, basePath string, includeRemoteNodes bool) string {
	t.Helper()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := fmt.Sprintf(`host: "127.0.0.1"
port: %s
basePath: "%s"
`, port, basePath)

	if includeRemoteNodes {
		configContent += fmt.Sprintf(`remoteNodes:
  - name: "dev"
    apiBaseUrl: "http://127.0.0.1:%s%s/api/v2"
`, port, basePath)
	}

	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o600))
	return configFile
}

func startServer(t *testing.T, configFile, port string) func() {
	t.Helper()
	th := test.SetupCommand(t)

	done := make(chan struct{})
	go func() {
		th.RunCommand(t, cmd.Server(), test.CmdTest{
			Args:        []string{"server", "--config", configFile, "--port=" + port},
			ExpectedOut: []string{"Server is starting"},
		})
		close(done)
	}()

	waitForServer(t, port)

	return func() {
		th.Cancel()
		<-done
	}
}

func waitForServer(t *testing.T, port string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start on port %s", port)
}

func requireHealthy(t *testing.T, url string) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

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
