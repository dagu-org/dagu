package integration_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestServer_StartWithConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setupFunc  func(t *testing.T) (string, string) // returns configFile and tempDir
		dagPath    func(t *testing.T, tempDir string) string
		envVarName string
	}{
		{
			name: "GlobalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				configFile := filepath.Join(tempDir, "config.yaml")
				configContent := `logDir: ${TMP_LOGS_DIR}/logs`
				require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0600))
				return configFile, tempDir
			},
			dagPath: func(t *testing.T, _ string) string {
				return test.TestdataPath(t, path.Join("integration", "basic.yaml"))
			},
			envVarName: "TMP_LOGS_DIR",
		},
		{
			name: "DAGLocalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				dagFile := filepath.Join(tempDir, "basic.yaml")
				dagContent := `
logDir: ${DAG_TMP_LOGS_DIR}/logs
steps:
  - name: step1
    command: echo "Hello, world!"
`
				require.NoError(t, os.WriteFile(dagFile, []byte(dagContent), 0600))
				return dagFile, tempDir
			},
			dagPath: func(_ *testing.T, tempDir string) string {
				return filepath.Join(tempDir, "basic.yaml")
			},
			envVarName: "DAG_TMP_LOGS_DIR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test case
			configFile, tempDir := tc.setupFunc(t)
			_ = os.Setenv(tc.envVarName, tempDir)

			// Get DAG path
			dagPath := tc.dagPath(t, tempDir)

			// Run command
			th := test.SetupCommand(t)
			args := []string{"start"}
			if tc.name == "GlobalLogDir" {
				args = append(args, "--config", configFile)
			}
			args = append(args, dagPath)

			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
				Args:        args,
				ExpectedOut: []string{"Workflow finished"},
			})
		})
	}
}
