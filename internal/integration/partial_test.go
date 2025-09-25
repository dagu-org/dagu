package integration_test

import (
	"testing"

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
			name: "BasicPartialSuccess",
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
			name: "SuccessByMarkingStepAsSuccessful",
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
			name: "SingleStepWithContinueOnFailure",
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
			name: "SingleStepWithContinueOnMarkingAsSuccess",
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
			testDAG := th.DAG(t, tc.yaml)

			// Run the DAG
			agent := testDAG.Agent()
			err := agent.Run(agent.Context)

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
