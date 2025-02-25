package integration_test

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestServer_StartWithConfig(t *testing.T) {
	// Create a temporary config file with BasePath set to "/dagu"
	tempDir := t.TempDir()

	// Set the environment variable to test logs directory configuration.
	os.Setenv("TMP_LOGS_DIR", tempDir)

	configFile := filepath.Join(tempDir, "config.yaml")
	// The YAML configuration sets host, port, and basePath.
	// (Other config fields use default values.)
	configContent := `logDir: ${TMP_LOGS_DIR}/logs`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Use the provided test helper to set up context and cancellation.
	th := test.SetupCommand(t)

	// Execute the DAG using the temporary config.
	th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
		Args:        []string{"start", "--config", configFile, test.TestdataPath(t, path.Join("integration", "basic"))},
		ExpectedOut: []string{"DAG execution finished"},
	})

	// Check if the logs directory was created.
	_, err := os.Stat(tempDir + "/logs/basic")
	require.NoError(t, err)

	// Check if the log file was created.
	// The log file has the format "start_basic.<timestamp>.log".
	files, err := os.ReadDir(tempDir + "/logs/basic")
	require.NoError(t, err)

	// Check if there's at least one log file that matches the expected pattern
	logFileFound := false
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "start_basic.") && strings.HasSuffix(file.Name(), ".log") {
			logFileFound = true
			break
		}
	}
	require.True(t, logFileFound, "No log file found with expected naming pattern")
}
