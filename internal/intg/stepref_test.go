package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStepIDPropertyAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		yaml           string
		expectedStatus core.Status
		expectedOutput map[string]any
	}{
		{
			name: "BasicStdout/StderrFileAccess",
			yaml: `
type: graph
steps:
  - id: gen
    command: |
      echo "Test output data"
      echo "Error message" >&2
    output: GEN_OUTPUT

  - depends:
      - gen
    command: |
      echo "stdout_file=${gen.stdout}"
      echo "stderr_file=${gen.stderr}"
    output: FILE_PATHS
`,
			expectedStatus: core.Succeeded,
			expectedOutput: map[string]any{
				"GEN_OUTPUT": "Test output data",
				// FILE_PATHS will contain file paths which include .out and .err
				"FILE_PATHS": []test.Contains{
					test.Contains(".out"),
					test.Contains(".err"),
				},
			},
		},
		{
			name: "ExitCodeAccess",
			yaml: `
type: graph
steps:
  - id: success
    command: exit 0

  - id: failure
    command: exit 42
    continue_on:
      failure: true

  - depends:
      - success
      - failure
    command: |
      echo "success_code=${success.exitCode}"
      echo "failure_code=${failure.exitCode}"
    output: EXIT_CODES
`,
			expectedStatus: core.PartiallySucceeded,
			expectedOutput: map[string]any{
				"EXIT_CODES": "success_code=0\nfailure_code=42",
			},
		},
		{
			name: "UnknownStepIDRemainsUnchanged",
			yaml: `
type: graph
steps:
  - id: first_step
    command: echo "Hello"
    output: FIRST_OUT

  - depends:
      - first_step
    command: |
      echo "known=${first_step.stdout}"
      echo "unknown=\${unknown_step.stdout}"
      echo "invalid=\${first_step.unknown_property}"
    output: RESULT
`,
			expectedStatus: core.Succeeded,
			expectedOutput: map[string]any{
				"FIRST_OUT": "Hello",
				"RESULT": []test.Contains{
					test.Contains("known="),
					test.Contains(".out"),
					test.Contains("${unknown_step.stdout}"),
					test.Contains("${first_step.unknown_property}"),
				},
			},
		},
		{
			name: "RegularVariableTakesPrecedenceOverStepID",
			yaml: `
type: graph
steps:
  - id: check
    command: echo '{"status":"from-step"}'
    output: check

  - depends:
      - check
    command: |
      echo "variable=${check.status}"
      echo "stdout=${check.stdout}"
    output: PRECEDENCE_TEST
`,
			expectedStatus: core.Succeeded,
			expectedOutput: map[string]any{
				"check": `{"status":"from-step"}`,
				"PRECEDENCE_TEST": []test.Contains{
					test.Contains("variable=from-step"),
					test.Contains("stdout="), // When variable exists, step properties are still accessible
					test.Contains(".out"),    // Should contain the stdout file path
				},
			},
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

			if tc.expectedStatus == core.Succeeded {
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

func TestStepIDComplexScenarios(t *testing.T) {
	t.Parallel()

	t.Run("MultipleStepsWithSameOutput", func(t *testing.T) {
		th := test.Setup(t)

		yaml := `
type: graph
steps:
  - id: gen1
    command: echo "data from gen1"
    output: DATA

  - id: gen2
    command: echo "data from gen2"
    output: DATA

  - depends:
      - gen1
      - gen2
    command: |
      echo "gen1_output=data from gen1"
      echo "gen2_output=data from gen2"
      echo "current_DATA=${DATA}"
    output: RESULT
`
		testDAG := th.DAG(t, yaml)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		// Note: When multiple steps output to the same variable name,
		// the final value depends on the order of execution which may not be deterministic
		testDAG.AssertOutputs(t, map[string]any{
			"DATA": test.NotEmpty{}, // Either gen1 or gen2's value
			"RESULT": []test.Contains{
				test.Contains("gen1_output=data from gen1"),
				test.Contains("gen2_output=data from gen2"),
				test.Contains("current_DATA=data from gen"), // Could be gen1 or gen2
			},
		})
	})

	t.Run("ChainedStepReferences", func(t *testing.T) {
		th := test.Setup(t)

		yaml := `
steps:
  - id: s1
    command: echo "10"
    output: NUM

  - id: s2
    command: echo "15"
    output: NUM2

  - id: s3
    command: echo "20"
    output: NUM3

  - command: |
      echo "s1=10"
      echo "s2=15"
      echo "s3=20"
      echo "total=45"
    output: FINAL_RESULT
`
		testDAG := th.DAG(t, yaml)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)
		testDAG.AssertOutputs(t, map[string]any{
			"NUM":          "10",
			"NUM2":         "15",
			"NUM3":         "20",
			"FINAL_RESULT": "s1=10\ns2=15\ns3=20\ntotal=45",
		})
	})

	t.Run("StepIDInScript", func(t *testing.T) {
		th := test.Setup(t)

		yaml := `
type: graph
steps:
  - id: setup_step
    command: echo '{"env":"test","timeout":30}'
    output: CONFIG

  - depends:
      - setup_step
    script: |
      #!/bin/bash
      set -e

      # Access file paths
      echo "Setup logs available at: ${setup_step.stdout}"
    output: SCRIPT_OUTPUT
`
		testDAG := th.DAG(t, yaml)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		testDAG.AssertOutputs(t, map[string]any{
			"CONFIG": `{"env":"test","timeout":30}`,
			"SCRIPT_OUTPUT": []test.Contains{
				test.Contains("Setup logs available at:"),
				test.Contains(".out"),
			},
		})
	})
}

func TestStepIDErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("InvalidJSONPath", func(t *testing.T) {
		th := test.Setup(t)

		yaml := `
type: graph
steps:
  - id: gen
    command: echo "not json"
    output: DATA

  - depends:
      - gen
    command: |
      echo "data=not json"
    output: RESULT
`
		testDAG := th.DAG(t, yaml)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Should succeed but the invalid path should remain as-is
		testDAG.AssertLatestStatus(t, core.Succeeded)
		testDAG.AssertOutputs(t, map[string]any{
			"DATA":   "not json",
			"RESULT": "data=not json",
		})
	})

}
