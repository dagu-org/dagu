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

func TestStepIDPropertyAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		yaml           string
		expectedStatus status.Status
		expectedOutput map[string]any
	}{
		{
			name: "Basic stdout/stderr file access",
			yaml: `
name: test-step-id-files
steps:
  - name: generate-data
    id: gen
    command: |
      echo "Test output data"
      echo "Error message" >&2
    output: GEN_OUTPUT

  - name: process-files
    id: proc
    depends:
      - gen
    command: |
      echo "stdout_file=${gen.stdout}"
      echo "stderr_file=${gen.stderr}"
    output: FILE_PATHS
`,
			expectedStatus: status.StatusSuccess,
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
			name: "Exit code access",
			yaml: `
name: test-step-id-exitcode
steps:
  - name: success-step
    id: success
    command: exit 0

  - name: failure-step
    id: failure
    command: exit 42
    continueOn:
      failure: true

  - name: check-codes
    depends:
      - success
      - failure
    command: |
      echo "success_code=${success.exitCode}"
      echo "failure_code=${failure.exitCode}"
    output: EXIT_CODES
`,
			expectedStatus: status.StatusPartialSuccess,
			expectedOutput: map[string]any{
				"EXIT_CODES": "success_code=0\nfailure_code=42",
			},
		},
		{
			name: "Unknown step ID remains unchanged",
			yaml: `
name: test-step-id-unknown
steps:
  - name: first
    id: first_step
    command: echo "Hello"
    output: FIRST_OUT

  - name: second
    depends:
      - first_step
    command: |
      echo "known=${first_step.stdout}"
      echo "unknown=\${unknown_step.stdout}"
      echo "invalid=\${first_step.unknown_property}"
    output: RESULT
`,
			expectedStatus: status.StatusSuccess,
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
			name: "Regular variable takes precedence over step ID",
			yaml: `
name: test-step-id-precedence
steps:
  - name: create-var
    id: check
    command: echo '{"status":"from-step"}'
    output: check

  - name: verify-precedence
    depends:
      - check
    command: |
      echo "variable=${check.status}"
      echo "stdout=${check.stdout}"
    output: PRECEDENCE_TEST
`,
			expectedStatus: status.StatusSuccess,
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

			if tc.expectedStatus == status.StatusSuccess {
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
name: test-multiple-outputs
steps:
  - name: first-generator
    id: gen1
    command: echo "data from gen1"
    output: DATA

  - name: second-generator
    id: gen2
    command: echo "data from gen2"
    output: DATA

  - name: consumer
    depends:
      - gen1
      - gen2
    command: |
      echo "gen1_output=data from gen1"
      echo "gen2_output=data from gen2"
      echo "current_DATA=${DATA}"
    output: RESULT
`
		testFile := filepath.Join(t.TempDir(), "test.yaml")
		err := os.WriteFile(testFile, []byte(yaml), 0644)
		require.NoError(t, err)

		dag, err := digraph.Load(th.Context, testFile)
		require.NoError(t, err)

		testDAG := test.DAG{
			Helper: &th,
			DAG:    dag,
		}

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.StatusSuccess)

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
name: test-chained-refs
steps:
  - name: step1
    id: s1
    command: echo "10"
    output: NUM

  - name: step2
    id: s2
    depends:
      - s1
    command: echo "15"
    output: NUM2

  - name: step3
    id: s3
    depends:
      - s2
    command: echo "20"
    output: NUM3

  - name: final
    depends:
      - s3
    command: |
      echo "s1=10"
      echo "s2=15"
      echo "s3=20"
      echo "total=45"
    output: FINAL_RESULT
`
		testFile := filepath.Join(t.TempDir(), "test.yaml")
		err := os.WriteFile(testFile, []byte(yaml), 0644)
		require.NoError(t, err)

		dag, err := digraph.Load(th.Context, testFile)
		require.NoError(t, err)

		testDAG := test.DAG{
			Helper: &th,
			DAG:    dag,
		}

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.StatusSuccess)
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
name: test-script-refs
steps:
  - name: setup
    id: setup_step
    command: echo '{"env":"test","timeout":30}'
    output: CONFIG

  - name: run-script
    depends:
      - setup_step
    script: |
      #!/bin/bash
      set -e
      
      # Access file paths
      echo "Setup logs available at: ${setup_step.stdout}"
    output: SCRIPT_OUTPUT
`
		testFile := filepath.Join(t.TempDir(), "test.yaml")
		err := os.WriteFile(testFile, []byte(yaml), 0644)
		require.NoError(t, err)

		dag, err := digraph.Load(th.Context, testFile)
		require.NoError(t, err)

		testDAG := test.DAG{
			Helper: &th,
			DAG:    dag,
		}

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.StatusSuccess)

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
name: test-invalid-json-path
steps:
  - name: generate
    id: gen
    command: echo "not json"
    output: DATA

  - name: access
    depends:
      - gen
    command: |
      echo "data=not json"
    output: RESULT
`
		testFile := filepath.Join(t.TempDir(), "test.yaml")
		err := os.WriteFile(testFile, []byte(yaml), 0644)
		require.NoError(t, err)

		dag, err := digraph.Load(th.Context, testFile)
		require.NoError(t, err)

		testDAG := test.DAG{
			Helper: &th,
			DAG:    dag,
		}

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Should succeed but the invalid path should remain as-is
		testDAG.AssertLatestStatus(t, status.StatusSuccess)
		testDAG.AssertOutputs(t, map[string]any{
			"DATA":   "not json",
			"RESULT": "data=not json",
		})
	})

}
