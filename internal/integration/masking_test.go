package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskingE2E(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		yaml           string
		wantMasked     []string // Values that should be masked in logs
		wantUnmasked   []string // Values that should NOT be masked in logs
		wantInLog      []string // Strings that should appear in logs
		skipLogCheck   bool     // Skip checking log content (for disabled test)
	}{
		{
			name: "DefaultEnabled",
			yaml: `
env:
  - API_KEY: mysecretkey123
  - PASSWORD: supersecret456
steps:
  - name: test
    command: echo "API_KEY=$API_KEY PASSWORD=$PASSWORD"
`,
			wantMasked: []string{"mysecretkey123", "supersecret456"},
			wantInLog:  []string{"API_KEY=*******", "PASSWORD=*******"},
		},
		{
			name: "WithSafelist",
			yaml: `
env:
  - API_KEY: mysecretkey123
  - LOG_LEVEL: debug
maskEnv:
  safelist:
    - LOG_LEVEL
steps:
  - name: test
    command: echo "API_KEY=$API_KEY LOG_LEVEL=$LOG_LEVEL"
`,
			wantMasked:   []string{"mysecretkey123"},
			wantUnmasked: []string{"debug"},
			wantInLog:    []string{"API_KEY=*******", "LOG_LEVEL=debug"},
		},
		{
			name: "Disabled",
			yaml: `
env:
  - API_KEY: mysecretkey123
maskEnv:
  disable: true
steps:
  - name: test
    command: echo "API_KEY=$API_KEY"
`,
			wantUnmasked: []string{"mysecretkey123"},
			wantInLog:    []string{"API_KEY=mysecretkey123"},
			skipLogCheck: true,
		},
		{
			name: "StepEnv",
			yaml: `
env:
  - DAG_SECRET: dagsecret123
steps:
  - name: test
    env:
      - STEP_SECRET: stepsecret456
    command: echo "DAG_SECRET=$DAG_SECRET STEP_SECRET=$STEP_SECRET"
`,
			wantMasked: []string{"dagsecret123", "stepsecret456"},
			wantInLog:  []string{"DAG_SECRET=*******", "STEP_SECRET=*******"},
		},
		{
			name: "MinLength",
			yaml: `
env:
  - SHORT: ab
  - LONG: longsecret123
steps:
  - name: test
    command: echo "SHORT=$SHORT LONG=$LONG"
`,
			wantMasked:   []string{"longsecret123"},
			wantUnmasked: []string{"ab"},
			wantInLog:    []string{"SHORT=ab", "LONG=*******"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)
			dag := th.DAG(t, tt.yaml)

			// Execute the DAG
			agent := dag.Agent()
			err := agent.Run(agent.Context)
			require.NoError(t, err)

			// Verify successful completion
			dag.AssertLatestStatus(t, core.Success)

			// Get the latest status
			dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
			require.NoError(t, err)
			require.NotNil(t, dagRunStatus)
			require.Len(t, dagRunStatus.Nodes, 1)

			// Read the stdout log file
			node := dagRunStatus.Nodes[0]
			logContent, err := os.ReadFile(node.Stdout)
			require.NoError(t, err)
			logStr := string(logContent)

			// Check masked values
			for _, val := range tt.wantMasked {
				assert.NotContains(t, logStr, val, "Value %q should be masked", val)
			}

			// Check unmasked values
			for _, val := range tt.wantUnmasked {
				assert.Contains(t, logStr, val, "Value %q should NOT be masked", val)
			}

			// Check expected log content
			if !tt.skipLogCheck {
				for _, expected := range tt.wantInLog {
					assert.Contains(t, logStr, expected, "Log should contain %q", expected)
				}
			}
		})
	}
}

func TestMaskingE2E_StdoutRedirect(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	redirectFile := filepath.Join(t.TempDir(), "redirect.log")

	dag := th.DAG(t, `
env:
  - API_KEY: mysecretkey123
steps:
  - name: test
    command: echo "API_KEY=$API_KEY"
    stdout: `+redirectFile+`
`)

	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Success)

	// Read the redirected stdout file
	logContent, err := os.ReadFile(redirectFile)
	require.NoError(t, err)
	logStr := string(logContent)

	// Verify secrets are masked in redirected output
	assert.NotContains(t, logStr, "mysecretkey123", "Secret should be masked in redirect file")
	assert.Contains(t, logStr, "API_KEY=*******", "Masked value should be in redirect file")
}

func TestMaskingE2E_OutputVariableNotMasked(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	dag := th.DAG(t, `
env:
  - API_KEY: mysecretkey123
steps:
  - name: test
    command: echo "mysecretkey123"
    output: CAPTURED_VALUE
`)

	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1)

	node := dagRunStatus.Nodes[0]
	require.NotNil(t, node.OutputVariables, "Node should have output variables")

	// Verify output variable contains UNMASKED value (output variables are runtime values)
	capturedValue, exists := node.OutputVariables.Load("CAPTURED_VALUE")
	require.True(t, exists, "Output variable CAPTURED_VALUE should exist")
	assert.Contains(t, capturedValue, "mysecretkey123", "Output variable should contain UNMASKED value")
	assert.NotContains(t, capturedValue, "*******", "Output variable should NOT be masked")

	// Verify log file has the masked value
	logContent, err := os.ReadFile(node.Stdout)
	require.NoError(t, err)
	logStr := string(logContent)
	assert.Contains(t, logStr, "*******", "Log file should contain masked value")
	assert.NotContains(t, logStr, "mysecretkey123", "Log file should NOT contain unmasked secret")
}
