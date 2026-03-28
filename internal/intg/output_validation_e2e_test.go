// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Scenario 1: Output Validation — Success
// ---------------------------------------------------------------------------

func TestOutputValidation_Success(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: generate-output
    command: 'echo ''{"summary":"test result","confidence":0.95}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          summary: { type: string }
          confidence: { type: number, minimum: 0.0, maximum: 1.0 }
        required: [summary, confidence]
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, status.Status)
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
	assert.Empty(t, status.Nodes[0].Error)

	// Verify output was captured to the variable
	outputs := readOutputsFile(t, th, dag.DAG)
	require.NotNil(t, outputs)
	assert.Contains(t, outputs["result"], `"summary":"test result"`)
}

func TestOutputValidation_Success_BoundaryValues(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	// confidence exactly at minimum (0.0) and maximum (1.0) boundaries
	t.Run("at minimum boundary", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - name: boundary-min
    command: 'echo ''{"summary":"min","confidence":0.0}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          summary: { type: string }
          confidence: { type: number, minimum: 0.0, maximum: 1.0 }
        required: [summary, confidence]
`)
		dag.Agent().RunSuccess(t)
	})

	t.Run("at maximum boundary", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - name: boundary-max
    command: 'echo ''{"summary":"max","confidence":1.0}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          summary: { type: string }
          confidence: { type: number, minimum: 0.0, maximum: 1.0 }
        required: [summary, confidence]
`)
		dag.Agent().RunSuccess(t)
	})
}

func TestOutputValidation_Success_SchemaReference(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	schemaDir := t.TempDir()
	schemaPath := filepath.Join(schemaDir, "output-schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(`{
  "type": "object",
  "properties": {
    "summary": {"type": "string"},
    "confidence": {"type": "number", "minimum": 0.0, "maximum": 1.0}
  },
  "required": ["summary", "confidence"]
}`), 0o600))

	dag := th.DAG(t, `
steps:
  - name: generate-output
    command: 'echo ''{"summary":"test result","confidence":0.95}'''
    output:
      name: RESULT
      schema: "`+filepath.ToSlash(schemaPath)+`"
`)
	agent := dag.Agent()
	agent.RunSuccess(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, status.Status)
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
	assert.Empty(t, status.Nodes[0].Error)
}

// ---------------------------------------------------------------------------
// Scenario 2: Output Validation — Failure (schema violation)
// ---------------------------------------------------------------------------

func TestOutputValidation_Failure_SchemaViolation(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: generate-bad-output
    command: 'echo ''{"summary":"test","confidence":2.0}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          confidence: { type: number, minimum: 0.0, maximum: 1.0 }
        required: [confidence]
`)
	agent := dag.Agent()
	agent.RunError(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Failed, status.Status)
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, core.NodeFailed, status.Nodes[0].Status)
	assert.Contains(t, status.Nodes[0].Error, "output validation failed")
}

func TestOutputValidation_Failure_MissingRequired(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: missing-field
    command: 'echo ''{"summary":"test"}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          summary: { type: string }
          confidence: { type: number }
        required: [summary, confidence]
`)
	agent := dag.Agent()
	agent.RunError(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Failed, status.Status)
	assert.Contains(t, status.Nodes[0].Error, "output validation failed")
}

func TestOutputValidation_Failure_WrongType(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	// confidence is string instead of number
	dag := th.DAG(t, `
steps:
  - name: wrong-type
    command: 'echo ''{"confidence":"not-a-number"}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          confidence: { type: number }
        required: [confidence]
`)
	agent := dag.Agent()
	agent.RunError(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	assert.Equal(t, core.NodeFailed, status.Nodes[0].Status)
	assert.Contains(t, status.Nodes[0].Error, "output validation failed")
}

// ---------------------------------------------------------------------------
// Scenario 3: Output Validation — Non-JSON Output
// ---------------------------------------------------------------------------

func TestOutputValidation_Failure_NonJSON(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: plain-text-output
    command: echo "this is plain text, not JSON"
    output:
      name: RESULT
      schema:
        type: object
        properties:
          key: { type: string }
`)
	agent := dag.Agent()
	agent.RunCheckErr(t, "not valid JSON")

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	assert.Equal(t, core.Failed, status.Status)
	assert.Contains(t, status.Nodes[0].Error, "not valid JSON")
}

// ---------------------------------------------------------------------------
// Scenario 4: Parameter Validation — Inline Definitions
// ---------------------------------------------------------------------------

func TestParamValidation_InlineDefinitions(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)
	dagFile := th.CreateDAGFile(t, "param-inline-validation.yaml", `
name: param-inline-validation
params:
  - name: DEPLOY_ENV
    type: string
    enum: [dev, staging, prod]
    required: true
  - name: REPLICAS
    type: integer
    minimum: 1
    maximum: 10
    default: 2
  - name: DRY_RUN
    type: boolean
    default: false
steps:
  - name: deploy
    command: echo "env=$DEPLOY_ENV replicas=$REPLICAS dry=$DRY_RUN"
    output: DEPLOY_OUTPUT
`)

	t.Run("valid params succeed", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=prod REPLICAS=5 DRY_RUN=true", dagFile},
			ExpectedOut: []string{"DAG run finished"},
		})
		status, outputs := readAttemptStatusAndOutputs(t, th, "param-inline-validation", runID)
		require.Equal(t, core.Succeeded, status.Status)
		assert.Equal(t, "env=prod replicas=5 dry=true", outputs.Outputs["deployOutput"])
	})

	t.Run("invalid enum value fails", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=production", dagFile},
		})
		require.Error(t, err)
	})

	t.Run("integer above maximum fails", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=dev REPLICAS=100", dagFile},
		})
		require.Error(t, err)
	})

	t.Run("integer below minimum fails", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=dev REPLICAS=0", dagFile},
		})
		require.Error(t, err)
	})

	t.Run("missing required param fails", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, dagFile},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DEPLOY_ENV")
	})

	t.Run("defaults used for optional params", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=staging", dagFile},
			ExpectedOut: []string{"DAG run finished"},
		})
		status, outputs := readAttemptStatusAndOutputs(t, th, "param-inline-validation", runID)
		require.Equal(t, core.Succeeded, status.Status)
		assert.Equal(t, "env=staging replicas=2 dry=false", outputs.Outputs["deployOutput"])
	})
}

func TestParamValidation_ExternalSchema(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)

	schemaDir := t.TempDir()
	schemaPath := filepath.Join(schemaDir, "deploy-schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(`{
  "type": "object",
  "properties": {
    "DEPLOY_ENV": { "type": "string", "enum": ["dev", "staging", "prod"] },
    "REPLICAS": { "type": "integer", "minimum": 1, "maximum": 10, "default": 3 }
  },
  "required": ["DEPLOY_ENV"],
  "additionalProperties": false
}`), 0o600))

	dagFile := th.CreateDAGFile(t, "param-ext-schema.yaml", `
name: param-ext-schema
params:
  schema: "`+schemaPath+`"
  values:
    DEPLOY_ENV: dev
steps:
  - name: show
    command: echo "env=$DEPLOY_ENV replicas=$REPLICAS"
    output: VALUES
`)

	t.Run("defaults accepted", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        []string{"start", "--run-id", runID, dagFile},
			ExpectedOut: []string{"DAG run finished"},
		})
		status, _ := readAttemptStatusAndOutputs(t, th, "param-ext-schema", runID)
		require.Equal(t, core.Succeeded, status.Status)
	})

	t.Run("unknown param rejected by additionalProperties:false", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, "--params", "DEPLOY_ENV=dev UNKNOWN_PARAM=xyz", dagFile},
		})
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Scenario 5: No Schema — No Regression
// ---------------------------------------------------------------------------

func TestNoSchema_NoRegression(t *testing.T) {
	t.Parallel()

	t.Run("simple output without schema works", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: simple-output
    command: echo "hello world"
    output: RESULT
`)
		dag.Agent().RunSuccess(t)

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 1)
		assert.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
	})

	t.Run("no output config at all works", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: no-output
    command: echo "hello"
`)
		dag.Agent().RunSuccess(t)
	})

	t.Run("multi-step DAG without schemas works", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
steps:
  - name: step1
    command: echo "first=1"
    output: FIRST

  - name: step2
    depends: [step1]
    command: echo "second=2"
    output: SECOND
`)
		dag.Agent().RunSuccess(t)

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
	})

	t.Run("output with custom key but no schema works", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: custom-key
    command: echo "MY_VAR=value"
    output:
      name: MY_VAR
      key: customKey
`)
		dag.Agent().RunSuccess(t)

		outputs := readOutputsFile(t, th, dag.DAG)
		require.NotNil(t, outputs)
		assert.Equal(t, "MY_VAR=value", outputs["customKey"])
	})
}

// ---------------------------------------------------------------------------
// Scenario: Mixed — some steps with schemas, some without
// ---------------------------------------------------------------------------

func TestOutputValidation_MixedSchemaAndNoSchema(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
type: graph
steps:
  - name: no-schema-step
    command: echo "plain output"
    output: PLAIN

  - name: schema-step
    depends: [no-schema-step]
    command: 'echo ''{"status":"ok","count":42}'''
    output:
      name: VALIDATED
      schema:
        type: object
        properties:
          status: { type: string }
          count: { type: integer }
        required: [status, count]
`)
	dag.Agent().RunSuccess(t)

	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, status.Status)
	require.Len(t, status.Nodes, 2)
	for _, node := range status.Nodes {
		assert.Equal(t, core.NodeSucceeded, node.Status)
	}
}

// ---------------------------------------------------------------------------
// Scenario: Output validation failure doesn't block subsequent independent steps
// ---------------------------------------------------------------------------

func TestOutputValidation_FailureBlocksDependents(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
type: graph
steps:
  - name: bad-output
    command: 'echo ''{"confidence":2.0}'''
    output:
      name: RESULT
      schema:
        type: object
        properties:
          confidence: { type: number, maximum: 1.0 }

  - name: dependent-step
    depends: [bad-output]
    command: echo "should not run"
`)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	status := agent.Status(agent.Context)
	require.Equal(t, core.Failed, status.Status)

	// Find the bad-output node
	var badNode, depNode *exec1.Node
	for _, n := range status.Nodes {
		switch n.Step.Name {
		case "bad-output":
			badNode = n
		case "dependent-step":
			depNode = n
		}
	}
	require.NotNil(t, badNode)
	require.NotNil(t, depNode)
	assert.Equal(t, core.NodeFailed, badNode.Status)
	assert.Contains(t, badNode.Error, "output validation failed")
	// Dependent step should not have succeeded
	assert.NotEqual(t, core.NodeSucceeded, depNode.Status)
}

// ---------------------------------------------------------------------------
// Scenario: Parameter + Output validation in the same DAG (full pipeline)
// ---------------------------------------------------------------------------

func TestFullPipeline_ParamAndOutputValidation(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)
	dagFile := th.CreateDAGFile(t, "full-pipeline.yaml", `
name: full-pipeline
params:
  - name: THRESHOLD
    type: number
    minimum: 0.0
    maximum: 1.0
    required: true
steps:
  - name: analyze
    command: 'echo ''{"score":0.85,"label":"pass"}'''
    output:
      name: ANALYSIS
      schema:
        type: object
        properties:
          score: { type: number, minimum: 0.0, maximum: 1.0 }
          label: { type: string, enum: ["pass", "fail"] }
        required: [score, label]
`)

	t.Run("valid params and valid output succeeds", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        []string{"start", "--run-id", runID, "--params", "THRESHOLD=0.5", dagFile},
			ExpectedOut: []string{"DAG run finished"},
		})
		status, _ := readAttemptStatusAndOutputs(t, th, "full-pipeline", runID)
		require.Equal(t, core.Succeeded, status.Status)
	})

	t.Run("invalid param rejected before execution", func(t *testing.T) {
		runID := uuid.Must(uuid.NewV7()).String()
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--run-id", runID, "--params", "THRESHOLD=5.0", dagFile},
		})
		require.Error(t, err)
		// Should fail at param validation, not reach execution
		_, lookupErr := th.DAGRunStore.FindAttempt(th.Context, exec1.NewDAGRunRef("full-pipeline", runID))
		require.ErrorIs(t, lookupErr, exec1.ErrDAGRunIDNotFound)
	})
}
