package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestServer_StartWithConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setupFunc  func(t *testing.T) (string, string) // returns configFile and dagPath
		envVarName string
	}{
		{
			name: "GlobalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				configFile := filepath.Join(tempDir, "config.yaml")
				configContent := `logDir: ${TMP_LOGS_DIR}/logs`
				require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0600))

				// Create DAG with inline YAML
				th := test.Setup(t)
				dagContent := `steps:
  - name: step1
    command: echo "Hello, world!"
`
				dag := th.DAG(t, dagContent)

				return configFile, dag.Location
			},
			envVarName: "TMP_LOGS_DIR",
		},
		{
			name: "DAGLocalLogDir",
			setupFunc: func(t *testing.T) (string, string) {
				// Create DAG with inline YAML
				th := test.Setup(t)
				dagContent := `
logDir: ${DAG_TMP_LOGS_DIR}/logs
steps:
  - name: step1
    command: echo "Hello, world!"
`
				dag := th.DAG(t, dagContent)
				return "", dag.Location
			},
			envVarName: "DAG_TMP_LOGS_DIR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test case
			configFile, dagPath := tc.setupFunc(t)
			tempDir := t.TempDir()
			_ = os.Setenv(tc.envVarName, tempDir)

			// Run command
			th := test.SetupCommand(t)
			args := []string{"start"}
			if tc.name == "GlobalLogDir" && configFile != "" {
				args = append(args, "--config", configFile)
			}
			args = append(args, dagPath)

			th.RunCommand(t, cli.Start(), test.CmdTest{
				Args:        args,
				ExpectedOut: []string{"dag-run finished"},
			})
		})
	}
}
