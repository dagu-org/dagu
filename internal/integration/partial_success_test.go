package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestPartialSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		yaml           string
		expectedStatus status.Status
		expectedOutput map[string]any
	}{
		{
			name: "Basic partial success",
			yaml: `
steps:
  - name: fail-step
    command: exit 1
    continueOn:
      failure: true

  - name: success-step
    command: echo "This step should run even if the previous one fails"
`,
			expectedStatus: status.PartialSuccess,
		},
		{
			name: "Success by marking step as successful",
			yaml: `
steps:
  - name: fail-step
    command: exit 1
    continueOn:
      failure: true
      markSuccess: true

  - name: success-step
    command: echo "This step should run even if the previous one fails"
`,
			expectedStatus: status.Success,
		},
		{
			name: "Single step with continueOn failure",
			yaml: `
steps:
  - name: fail-step
    command: exit 1
    continueOn:
      failure: true
`,
			expectedStatus: status.Error,
		},
		{
			name: "Single step with continueOn marking as success",
			yaml: `
steps:
  - name: fail-step
    command: exit 1
    continueOn:
      failure: true
      markSuccess: true
`,
			expectedStatus: status.Success,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)

			// Create DAG from YAML
			testFile := filepath.Join(t.TempDir(), "test.yaml")
			err := os.WriteFile(testFile, []byte(tc.yaml), 0644)
			require.NoError(t, err)

			dag, err := digraph.Load(th.Context, testFile)
			require.NoError(t, err)

			testDAG := test.DAG{
				Helper: &th,
				DAG:    dag,
			}

			// Run the DAG
			agent := testDAG.Agent()
			err = agent.Run(agent.Context)

			if tc.expectedStatus == status.Success {
				require.NoError(t, err)
			}

			// Check status
			testDAG.AssertLatestStatus(t, tc.expectedStatus)

			// Check outputs
			if tc.expectedOutput != nil {
				testDAG.AssertOutputs(t, tc.expectedOutput)
			}
		})
	}
}
