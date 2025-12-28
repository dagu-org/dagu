package agent

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createNodeWithOutput creates a node with populated OutputVariables for testing.
func createNodeWithOutput(name, outputVar, outputValue string) *runtime.Node {
	syncMap := &collections.SyncMap{}
	syncMap.Store(outputVar, stringutil.NewKeyValue(outputVar, outputValue).String())

	step := core.Step{
		Name:   name,
		Output: outputVar,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status:          core.NodeSucceeded,
			OutputVariables: syncMap,
		},
	})
}

// createNodeWithOutputKey creates a node with a custom OutputKey.
func createNodeWithOutputKey(name, outputVar, outputValue, outputKey string) *runtime.Node {
	syncMap := &collections.SyncMap{}
	syncMap.Store(outputVar, stringutil.NewKeyValue(outputVar, outputValue).String())

	step := core.Step{
		Name:      name,
		Output:    outputVar,
		OutputKey: outputKey,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status:          core.NodeSucceeded,
			OutputVariables: syncMap,
		},
	})
}

// createNodeWithOmit creates a node with OutputOmit set to true.
func createNodeWithOmit(name, outputVar, outputValue string) *runtime.Node {
	syncMap := &collections.SyncMap{}
	syncMap.Store(outputVar, stringutil.NewKeyValue(outputVar, outputValue).String())

	step := core.Step{
		Name:       name,
		Output:     outputVar,
		OutputOmit: true,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status:          core.NodeSucceeded,
			OutputVariables: syncMap,
		},
	})
}

// createNodeWithoutOutput creates a node without any output defined.
func createNodeWithoutOutput(name string) *runtime.Node {
	step := core.Step{
		Name: name,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status: core.NodeSucceeded,
		},
	})
}

// createNodeWithNilOutputVariables creates a node with Output defined but nil OutputVariables.
func createNodeWithNilOutputVariables(name, outputVar string) *runtime.Node {
	step := core.Step{
		Name:   name,
		Output: outputVar,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status:          core.NodeSucceeded,
			OutputVariables: nil,
		},
	})
}

// createNodeWithNonStringOutput creates a node with a non-string value in OutputVariables.
func createNodeWithNonStringOutput(name, outputVar string, value any) *runtime.Node {
	syncMap := &collections.SyncMap{}
	syncMap.Store(outputVar, value) // Store non-string value directly

	step := core.Step{
		Name:   name,
		Output: outputVar,
	}

	return runtime.NodeWithData(runtime.NodeData{
		Step: step,
		State: runtime.NodeState{
			Status:          core.NodeSucceeded,
			OutputVariables: syncMap,
		},
	})
}

// createAgentForTest creates an Agent with the given plan and DAG for testing.
func createAgentForTest(plan *runtime.Plan, dag *core.DAG) *Agent {
	if dag == nil {
		dag = &core.DAG{Name: "test-dag"}
	}
	return &Agent{
		plan:            plan,
		dag:             dag,
		dagRunID:        "test-run-123",
		dagRunAttemptID: "test-attempt-456",
	}
}

// contextWithSecrets creates a context with secret environment variables.
func contextWithSecrets(secrets map[string]string) context.Context {
	dag := &core.DAG{Name: "test"}
	var secretEnvs []string
	for k, v := range secrets {
		secretEnvs = append(secretEnvs, k+"="+v)
	}
	return execution.NewContext(
		context.Background(),
		dag,
		"test-run",
		"/tmp/test.log",
		execution.WithSecrets(secretEnvs),
	)
}

// =============================================================================
// Tests for collectOutputs
// =============================================================================

func TestCollectOutputs_SingleStep(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutput("step1", "MY_OUTPUT", "hello world")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	require.Len(t, outputs, 1)
	assert.Equal(t, "hello world", outputs["myOutput"])
}

func TestCollectOutputs_NoOutputsDefined(t *testing.T) {
	t.Parallel()

	node := createNodeWithoutOutput("step1")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	assert.Empty(t, outputs)
}

func TestCollectOutputs_OmitFlag(t *testing.T) {
	t.Parallel()

	node := createNodeWithOmit("step1", "SECRET_OUTPUT", "sensitive data")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	assert.Empty(t, outputs, "outputs with omit flag should be excluded")
}

func TestCollectOutputs_NilOutputVariables(t *testing.T) {
	t.Parallel()

	node := createNodeWithNilOutputVariables("step1", "MY_OUTPUT")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	assert.Empty(t, outputs, "should gracefully handle nil OutputVariables")
}

func TestCollectOutputs_NonStringValue(t *testing.T) {
	t.Parallel()

	// Store an integer instead of a string
	node := createNodeWithNonStringOutput("step1", "MY_OUTPUT", 12345)
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	assert.Empty(t, outputs, "non-string values should be skipped")
}

func TestCollectOutputs_CustomOutputKey(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutputKey("step1", "SCREAMING_CASE", "value", "customKeyName")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	require.Len(t, outputs, 1)
	assert.Equal(t, "value", outputs["customKeyName"])
	_, hasDefault := outputs["screamingCase"]
	assert.False(t, hasDefault, "should use custom key, not camelCase conversion")
}

func TestCollectOutputs_CamelCaseConversion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		inputVar    string
		expectedKey string
	}{
		{"SIMPLE", "simple"},
		{"TWO_WORDS", "twoWords"},
		{"MULTIPLE_WORD_NAME", "multipleWordName"},
		{"ALREADY_LOWER", "alreadyLower"},
	}

	for _, tc := range testCases {
		t.Run(tc.inputVar, func(t *testing.T) {
			node := createNodeWithOutput("step", tc.inputVar, "value")
			plan := runtime.NewPlanWithNodes(node)
			agent := createAgentForTest(plan, nil)

			outputs := agent.collectOutputs(context.Background())

			require.Len(t, outputs, 1)
			assert.Equal(t, "value", outputs[tc.expectedKey])
		})
	}
}

func TestCollectOutputs_LastOneWins(t *testing.T) {
	t.Parallel()

	// Two nodes with the same output key (after camelCase conversion)
	node1 := createNodeWithOutput("step1", "RESULT", "first value")
	node2 := createNodeWithOutput("step2", "RESULT", "second value")
	plan := runtime.NewPlanWithNodes(node1, node2)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	require.Len(t, outputs, 1)
	assert.Equal(t, "second value", outputs["result"], "later step's value should overwrite earlier")
}

func TestCollectOutputs_MultipleSteps(t *testing.T) {
	t.Parallel()

	node1 := createNodeWithOutput("step1", "OUTPUT_ONE", "value1")
	node2 := createNodeWithOutput("step2", "OUTPUT_TWO", "value2")
	node3 := createNodeWithOutput("step3", "OUTPUT_THREE", "value3")
	plan := runtime.NewPlanWithNodes(node1, node2, node3)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	require.Len(t, outputs, 3)
	assert.Equal(t, "value1", outputs["outputOne"])
	assert.Equal(t, "value2", outputs["outputTwo"])
	assert.Equal(t, "value3", outputs["outputThree"])
}

func TestCollectOutputs_MixedConfigurations(t *testing.T) {
	t.Parallel()

	// Node with normal output
	node1 := createNodeWithOutput("step1", "NORMAL_OUTPUT", "normal")
	// Node with custom key
	node2 := createNodeWithOutputKey("step2", "CUSTOM_OUTPUT", "custom", "myCustomKey")
	// Node with omit
	node3 := createNodeWithOmit("step3", "OMITTED_OUTPUT", "omitted")
	// Node without output
	node4 := createNodeWithoutOutput("step4")
	// Node with nil OutputVariables
	node5 := createNodeWithNilOutputVariables("step5", "NIL_OUTPUT")

	plan := runtime.NewPlanWithNodes(node1, node2, node3, node4, node5)
	agent := createAgentForTest(plan, nil)

	outputs := agent.collectOutputs(context.Background())

	require.Len(t, outputs, 2)
	assert.Equal(t, "normal", outputs["normalOutput"])
	assert.Equal(t, "custom", outputs["myCustomKey"])
}

// =============================================================================
// Tests for buildOutputs
// =============================================================================

func TestBuildOutputs_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	node := createNodeWithoutOutput("step1")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	result := agent.buildOutputs(context.Background(), core.Succeeded)

	assert.Nil(t, result, "should return nil when no outputs collected")
}

func TestBuildOutputs_SecretMasking(t *testing.T) {
	t.Parallel()

	secretValue := "super-secret-token-xyz"
	node := createNodeWithOutput("step1", "API_RESPONSE", "Token: "+secretValue)
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	ctx := contextWithSecrets(map[string]string{
		"API_TOKEN": secretValue,
	})

	result := agent.buildOutputs(ctx, core.Succeeded)

	require.NotNil(t, result)
	assert.NotContains(t, result.Outputs["apiResponse"], secretValue, "secret should be masked")
	assert.Contains(t, result.Outputs["apiResponse"], "*******", "masked secret should be replaced with asterisks")
}

func TestBuildOutputs_MultipleSecrets(t *testing.T) {
	t.Parallel()

	secret1 := "password123"
	secret2 := "api-key-abc"
	node := createNodeWithOutput("step1", "LOG_OUTPUT", "Password: "+secret1+", Key: "+secret2)
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	ctx := contextWithSecrets(map[string]string{
		"PASSWORD": secret1,
		"API_KEY":  secret2,
	})

	result := agent.buildOutputs(ctx, core.Succeeded)

	require.NotNil(t, result)
	assert.NotContains(t, result.Outputs["logOutput"], secret1)
	assert.NotContains(t, result.Outputs["logOutput"], secret2)
}

func TestBuildOutputs_NoSecrets(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutput("step1", "MY_OUTPUT", "plain text value")
	plan := runtime.NewPlanWithNodes(node)
	agent := createAgentForTest(plan, nil)

	// Context without secrets
	ctx := context.Background()

	result := agent.buildOutputs(ctx, core.Succeeded)

	require.NotNil(t, result)
	assert.Equal(t, "plain text value", result.Outputs["myOutput"])
}

func TestBuildOutputs_Metadata(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutput("step1", "OUTPUT", "value")
	plan := runtime.NewPlanWithNodes(node)
	dag := &core.DAG{
		Name:   "my-dag",
		Params: []string{"key1=value1", "key2=value2"},
	}
	agent := &Agent{
		plan:            plan,
		dag:             dag,
		dagRunID:        "run-id-123",
		dagRunAttemptID: "attempt-id-456",
	}

	result := agent.buildOutputs(context.Background(), core.Succeeded)

	require.NotNil(t, result)
	assert.Equal(t, "my-dag", result.Metadata.DAGName)
	assert.Equal(t, "run-id-123", result.Metadata.DAGRunID)
	assert.Equal(t, "attempt-id-456", result.Metadata.AttemptID)
	assert.Equal(t, "succeeded", result.Metadata.Status)
	assert.NotEmpty(t, result.Metadata.CompletedAt)
	assert.NotEmpty(t, result.Metadata.Params)
}

func TestBuildOutputs_ParamsJSON(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutput("step1", "OUTPUT", "value")
	plan := runtime.NewPlanWithNodes(node)
	dag := &core.DAG{
		Name:   "test-dag",
		Params: []string{"env=prod", "count=10"},
	}
	agent := createAgentForTest(plan, dag)

	result := agent.buildOutputs(context.Background(), core.Succeeded)

	require.NotNil(t, result)
	assert.Contains(t, result.Metadata.Params, "env=prod")
	assert.Contains(t, result.Metadata.Params, "count=10")
}

func TestBuildOutputs_EmptyParams(t *testing.T) {
	t.Parallel()

	node := createNodeWithOutput("step1", "OUTPUT", "value")
	plan := runtime.NewPlanWithNodes(node)
	dag := &core.DAG{
		Name:   "test-dag",
		Params: nil,
	}
	agent := createAgentForTest(plan, dag)

	result := agent.buildOutputs(context.Background(), core.Succeeded)

	require.NotNil(t, result)
	assert.Empty(t, result.Metadata.Params)
}

func TestBuildOutputs_FinalStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		status       core.Status
		expectedText string
	}{
		{core.Succeeded, "succeeded"},
		{core.Failed, "failed"},
		{core.Aborted, "aborted"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedText, func(t *testing.T) {
			node := createNodeWithOutput("step1", "OUTPUT", "value")
			plan := runtime.NewPlanWithNodes(node)
			agent := createAgentForTest(plan, nil)

			result := agent.buildOutputs(context.Background(), tc.status)

			require.NotNil(t, result)
			assert.Equal(t, tc.expectedText, result.Metadata.Status)
		})
	}
}
