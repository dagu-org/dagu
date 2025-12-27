package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputsCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		dagYAML         string
		runFunc         func(*testing.T, context.Context, *test.Agent)
		validateFunc    func(*testing.T, execution.DAGRunStatus)
		validateOutputs func(*testing.T, map[string]string)
	}{
		{
			name: "SimpleStringOutput",
			dagYAML: `
steps:
  - name: produce-output
    command: echo "RESULT=42"
    output: RESULT
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.Len(t, status.Nodes, 1)
				require.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				// Output value includes the KEY= prefix from command output
				assert.Equal(t, "RESULT=42", outputs["result"]) // SCREAMING_SNAKE to camelCase
			},
		},
		{
			name: "OutputWithCustomKey",
			dagYAML: `
steps:
  - name: produce-output
    command: echo "MY_VALUE=hello world"
    output:
      name: MY_VALUE
      key: customKeyName
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				// Value includes the original KEY= prefix
				assert.Equal(t, "MY_VALUE=hello world", outputs["customKeyName"])
				_, hasDefault := outputs["myValue"]
				assert.False(t, hasDefault, "should not have default key when custom key is specified")
			},
		},
		{
			name: "OutputWithOmit",
			dagYAML: `
steps:
  - name: step1
    command: echo "VISIBLE=yes"
    output: VISIBLE

  - name: step2
    command: echo "HIDDEN=secret"
    output:
      name: HIDDEN
      omit: true
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.Len(t, status.Nodes, 2)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				assert.Equal(t, "VISIBLE=yes", outputs["visible"])
				_, hasHidden := outputs["hidden"]
				assert.False(t, hasHidden, "omitted output should not be in outputs.json")
			},
		},
		{
			name: "MultipleStepsWithOutputs",
			dagYAML: `
steps:
  - name: step1
    command: echo "COUNT=10"
    output: COUNT

  - name: step2
    command: echo "TOTAL=100"
    output: TOTAL

  - name: step3
    command: echo "STATUS=completed"
    output: STATUS
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.Len(t, status.Nodes, 3)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				assert.Len(t, outputs, 3)
				assert.Equal(t, "COUNT=10", outputs["count"])
				assert.Equal(t, "TOTAL=100", outputs["total"])
				assert.Equal(t, "STATUS=completed", outputs["status"])
			},
		},
		{
			name: "LastOneWinsForDuplicateKeys",
			dagYAML: `
steps:
  - name: step1
    command: echo "VALUE=first"
    output: VALUE

  - name: step2
    depends: [step1]
    command: echo "VALUE=second"
    output: VALUE
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				// Last step wins
				assert.Equal(t, "VALUE=second", outputs["value"])
			},
		},
		{
			name: "NoOutputsProduced",
			dagYAML: `
steps:
  - name: step1
    command: echo "hello"
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				// No outputs.json should be created when no outputs
				assert.Nil(t, outputs)
			},
		},
		{
			name: "OutputWithDollarPrefix",
			dagYAML: `
steps:
  - name: step1
    command: echo "MY_VAR=value123"
    output: $MY_VAR
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				assert.Equal(t, "MY_VAR=value123", outputs["myVar"])
			},
		},
		{
			name: "MixedOutputConfigurations",
			dagYAML: `
steps:
  - name: simple
    command: echo "SIMPLE_OUT=simple_value"
    output: SIMPLE_OUT

  - name: with-key
    command: echo "KEYED=keyed_value"
    output:
      name: KEYED
      key: renamedKey

  - name: omitted
    command: echo "SECRET=secret_value"
    output:
      name: SECRET
      omit: true
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
			},
			validateOutputs: func(t *testing.T, outputs map[string]string) {
				require.NotNil(t, outputs)
				assert.Len(t, outputs, 2) // simple + keyed, NOT secret
				assert.Equal(t, "SIMPLE_OUT=simple_value", outputs["simpleOut"])
				assert.Equal(t, "KEYED=keyed_value", outputs["renamedKey"])
				_, hasSecret := outputs["secret"]
				assert.False(t, hasSecret)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)
			dag := th.DAG(t, tt.dagYAML)
			agent := dag.Agent()

			// Run the DAG
			tt.runFunc(t, agent.Context, agent)

			// Validate DAG run status
			status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
			require.NoError(t, err)
			tt.validateFunc(t, status)

			// Read outputs.json if it exists
			outputs := readOutputsFile(t, th, dag.DAG, status.DAGRunID)
			tt.validateOutputs(t, outputs)
		})
	}
}

func TestOutputsCollection_FailedDAG(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	dag := th.DAG(t, `
steps:
  - name: step1
    command: echo "BEFORE_FAIL=collected"
    output: BEFORE_FAIL

  - name: step2
    depends: [step1]
    command: exit 1

  - name: step3
    depends: [step2]
    command: echo "AFTER_FAIL=not_collected"
    output: AFTER_FAIL
`)
	agent := dag.Agent()

	_ = agent.Run(agent.Context)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Failed, status.Status)

	// Outputs from successful steps should still be collected
	outputs := readOutputsFile(t, th, dag.DAG, status.DAGRunID)
	require.NotNil(t, outputs)
	assert.Equal(t, "BEFORE_FAIL=collected", outputs["beforeFail"])
	_, hasAfterFail := outputs["afterFail"]
	assert.False(t, hasAfterFail, "output from step after failure should not be collected")
}

func TestOutputsCollection_CamelCaseConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		envVarName    string
		expectedKey   string
		expectedValue string
	}{
		{"SIMPLE", "simple", "SIMPLE=test_value"},
		{"TWO_WORDS", "twoWords", "TWO_WORDS=test_value"},
		{"MULTIPLE_WORD_NAME", "multipleWordName", "MULTIPLE_WORD_NAME=test_value"},
		{"ALREADY_CAMEL_Case", "alreadyCamelCase", "ALREADY_CAMEL_Case=test_value"},
	}

	for _, tt := range tests {
		t.Run(tt.envVarName, func(t *testing.T) {
			t.Parallel()

			th := test.Setup(t)
			dag := th.DAG(t, `
steps:
  - name: step1
    command: echo "`+tt.envVarName+`=test_value"
    output: `+tt.envVarName+`
`)
			agent := dag.Agent()
			agent.RunSuccess(t)

			status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
			require.NoError(t, err)

			outputs := readOutputsFile(t, th, dag.DAG, status.DAGRunID)
			require.NotNil(t, outputs)
			// Value includes the KEY= prefix from the original output
			assert.Equal(t, tt.expectedValue, outputs[tt.expectedKey])
		})
	}
}

func TestOutputsCollection_MetadataIncluded(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	dag := th.DAG(t, `
steps:
  - name: step1
    command: echo "RESULT=42"
    output: RESULT
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)

	// Read full outputs including metadata
	fullOutputs := readFullOutputsFile(t, th, dag.DAG, status.DAGRunID)
	require.NotNil(t, fullOutputs)

	// Validate metadata
	assert.Equal(t, dag.DAG.Name, fullOutputs.Metadata.DAGName)
	assert.Equal(t, status.DAGRunID, fullOutputs.Metadata.DAGRunID)
	assert.NotEmpty(t, fullOutputs.Metadata.AttemptID)
	assert.Equal(t, "succeeded", fullOutputs.Metadata.Status)
	assert.NotEmpty(t, fullOutputs.Metadata.CompletedAt)

	// Validate outputs are still present
	assert.Equal(t, "RESULT=42", fullOutputs.Outputs["result"])
}

// readOutputsFile reads the outputs.json file for a given DAG run
// Returns just the outputs map for backward compatibility with existing tests
func readOutputsFile(t *testing.T, th test.Helper, dag *core.DAG, dagRunID string) map[string]string {
	t.Helper()

	fullOutputs := readFullOutputsFile(t, th, dag, dagRunID)
	if fullOutputs == nil {
		return nil
	}
	return fullOutputs.Outputs
}

// readFullOutputsFile reads the full outputs.json file including metadata
func readFullOutputsFile(t *testing.T, th test.Helper, dag *core.DAG, dagRunID string) *execution.DAGRunOutputs {
	t.Helper()

	// Find the attempt directory
	dagRunsDir := th.Config.Paths.DAGRunsDir
	dagRunDir := filepath.Join(dagRunsDir, dag.Name, "dag-runs")

	// Walk to find the outputs.json file
	var outputsPath string
	err := filepath.Walk(dagRunDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == filedagrun.OutputsFile {
			outputsPath = path
			return filepath.SkipAll
		}
		return nil
	})

	if err != nil || outputsPath == "" {
		return nil
	}

	data, err := os.ReadFile(outputsPath)
	if err != nil {
		return nil
	}

	var outputs execution.DAGRunOutputs
	if err := json.Unmarshal(data, &outputs); err != nil {
		t.Fatalf("failed to unmarshal outputs.json: %v", err)
	}

	// Return nil if old format (no metadata)
	if outputs.Metadata.DAGRunID == "" {
		return nil
	}

	return &outputs
}
