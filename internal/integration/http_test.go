package integration_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestHTTPExecutor(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load chain DAG
	dag := th.DAG(t, filepath.Join("integration", "http.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify execution order
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify that only successful response is captured in output variable
	dag.AssertOutputs(t, map[string]any{
		"RET200": test.NotEmpty{},
		"RET500": "", // Should be empty because HTTP 500 is an error
		"RET404": "", // Should be empty because HTTP 404 is an error
	})

	// Check that HTTP responses are written to stdout for all response codes
	// This includes both successful (200) and error responses (404, 500)
	testCases := []struct {
		stepName         string
		expectedInLog    string
		outputVar        string
		shouldHaveOutput bool
	}{
		{
			stepName:         "test_200",
			expectedInLog:    `{"code":200,"description":"OK"}`,
			outputVar:        "RET200",
			shouldHaveOutput: true,
		},
		{
			stepName:         "test_500",
			expectedInLog:    `{"code":500,"description":"Internal Server Error"}`,
			outputVar:        "RET500",
			shouldHaveOutput: false, // Error responses should not be captured
		},
		{
			stepName:         "test_404",
			expectedInLog:    `{"code":404,"description":"Not Found"}`,
			outputVar:        "RET404",
			shouldHaveOutput: false, // Error responses should not be captured
		},
	}

	for _, tc := range testCases {
		t.Run(tc.stepName, func(t *testing.T) {
			// Find the node for this step
			var node *models.Node
			for _, n := range status.Nodes {
				if n.Step.Name == tc.stepName {
					node = n
					break
				}
			}
			require.NotNil(t, node, "node %s not found", tc.stepName)

			// Read the stdout log file
			t.Logf("Reading stdout file: %s", node.Stdout)
			logContent, _, _, _, _, err := fileutil.ReadLogContent(node.Stdout, fileutil.LogReadOptions{})
			require.NoError(t, err, "failed to read stdout for step %s", tc.stepName)

			t.Logf("Step %s stdout content: %q", tc.stepName, logContent)

			// Check that the expected content is in the log
			require.Contains(t, logContent, tc.expectedInLog,
				"step %s stdout should contain expected response", tc.stepName)

			// For non-200 responses with silent=true, headers should be included
			if tc.stepName == "test_500" || tc.stepName == "test_404" {
				statusLine := map[string]string{
					"test_500": "500 Internal Server Error",
					"test_404": "404 Not Found",
				}[tc.stepName]
				require.Contains(t, logContent, statusLine,
					"step %s stdout should contain status line", tc.stepName)
				require.Contains(t, logContent, "Content-Type:",
					"step %s stdout should contain headers", tc.stepName)
			}

			// Verify output variable capture behavior
			if tc.outputVar != "" {
				if tc.shouldHaveOutput {
					require.NotNil(t, node.OutputVariables, "OutputVariables should not be nil for step %s", tc.stepName)
					value, ok := node.OutputVariables.Load(tc.outputVar)
					require.True(t, ok, "output variable %s should be set", tc.outputVar)
					require.NotEmpty(t, value, "output variable %s should not be empty", tc.outputVar)

					strValue := value.(string)

					// Verify the output contains the expected JSON response
					require.Contains(t, strValue, tc.expectedInLog,
						"output variable %s should contain response body", tc.outputVar)
				} else {
					// For error responses, output variable should not be set
					if node.OutputVariables != nil {
						value, ok := node.OutputVariables.Load(tc.outputVar)
						require.False(t, ok, "output variable %s should not be set for error response", tc.outputVar)
						require.Nil(t, value, "output variable %s should be nil", tc.outputVar)
					}
				}
			}
		})
	}

	// Additional checks for the echo steps that use the captured variables
	echoSteps := []struct {
		stepName        string
		outputVar       string
		expectedValue   string
		shouldHaveValue bool
	}{
		{
			stepName:        "ret_200",
			outputVar:       "RET200",
			expectedValue:   `{code:200,description:OK}`, // Echo removes quotes from JSON
			shouldHaveValue: true,
		},
		{
			stepName:        "ret_500",
			outputVar:       "RET500",
			expectedValue:   ``, // Should be empty since RET500 is not set
			shouldHaveValue: false,
		},
		{
			stepName:        "ret_404",
			outputVar:       "RET404",
			expectedValue:   ``, // Should be empty since RET404 is not set
			shouldHaveValue: false,
		},
	}

	for _, es := range echoSteps {
		t.Run(es.stepName+"_echo_check", func(t *testing.T) {
			// Find the node for the echo step
			var node *models.Node
			for _, n := range status.Nodes {
				if n.Step.Name == es.stepName {
					node = n
					break
				}
			}
			require.NotNil(t, node, "echo node %s not found", es.stepName)

			// Read the stdout log file for the echo step
			logContent, _, _, _, _, err := fileutil.ReadLogContent(node.Stdout, fileutil.LogReadOptions{})
			require.NoError(t, err, "failed to read stdout for echo step %s", es.stepName)

			if es.shouldHaveValue {
				// The echo step should output the value of the variable
				require.Contains(t, logContent, es.expectedValue,
					"echo step %s should output the captured HTTP response", es.stepName)
			} else {
				// For error responses, the variable is empty so echo outputs nothing (just newline)
				require.Equal(t, "", strings.TrimSpace(logContent),
					"echo step %s should output empty string for unset variable", es.stepName)
			}
		})
	}
}
