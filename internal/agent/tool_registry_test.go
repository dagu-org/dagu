// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRemoteContextResolver is a minimal resolver for tests.
type testRemoteContextResolver struct{}

func (r *testRemoteContextResolver) GetByName(_ context.Context, _ string) (RemoteContextInfo, error) {
	return RemoteContextInfo{}, nil
}

func (r *testRemoteContextResolver) ListRemoteContexts(_ context.Context) ([]RemoteContextInfo, error) {
	return nil, nil
}

type testControllerRuntime struct{}

func (r *testControllerRuntime) ListTasks(_ context.Context) ([]ControllerTask, error) {
	return []ControllerTask{{ID: "task-1", Description: "Investigate failure", State: "open"}}, nil
}

func (r *testControllerRuntime) ListWorkflows(_ context.Context) ([]ControllerWorkflow, error) {
	return []ControllerWorkflow{{Name: "example"}}, nil
}

func (r *testControllerRuntime) RunWorkflow(_ context.Context, input ControllerRunWorkflowInput) (ControllerRunWorkflowResult, error) {
	return ControllerRunWorkflowResult{WorkflowName: input.WorkflowName, DAGRunID: "run-1"}, nil
}

func (r *testControllerRuntime) RetryCurrentRun(_ context.Context) (ControllerRunWorkflowResult, error) {
	return ControllerRunWorkflowResult{WorkflowName: "example", DAGRunID: "run-2"}, nil
}

func (r *testControllerRuntime) SetTaskDone(_ context.Context, _ string, _ bool) error {
	return nil
}

func (r *testControllerRuntime) RequestHumanInput(_ context.Context, _ ControllerHumanPrompt) error {
	return nil
}

func (r *testControllerRuntime) Finish(_ context.Context, _ string) error {
	return nil
}

func TestRegisteredTools_ContainsAllExpected(t *testing.T) {
	t.Parallel()

	expected := []string{
		"bash", "read", "patch", "think",
		"navigate", "ask_user",
		"delegate",
		"remote_agent", "list_contexts",
		"list_controller_tasks",
		"list_workflows", "run_workflow", "retry_controller_run",
		"set_controller_task_done", "request_human_input", "finish_controller",
	}

	regs := RegisteredTools()
	names := make(map[string]bool, len(regs))
	for _, reg := range regs {
		names[reg.Name] = true
	}

	for _, name := range expected {
		assert.True(t, names[name], "expected tool %q to be registered", name)
	}
}

func TestRegisteredToolNames_Sorted(t *testing.T) {
	t.Parallel()

	names := RegisteredToolNames()
	require.NotEmpty(t, names)

	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] < names[i],
			"names not sorted: %q should come before %q", names[i-1], names[i])
	}
}

func TestIsRegisteredTool(t *testing.T) {
	t.Parallel()

	assert.True(t, IsRegisteredTool("bash"))
	assert.True(t, IsRegisteredTool("read"))
	assert.True(t, IsRegisteredTool("delegate"))
	assert.False(t, IsRegisteredTool("nonexistent"))
	assert.False(t, IsRegisteredTool(""))
}

func TestRegisteredTools_HaveMetadata(t *testing.T) {
	t.Parallel()

	for _, reg := range RegisteredTools() {
		t.Run(reg.Name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, reg.Name, "Name must be set")
			assert.NotEmpty(t, reg.Label, "Label must be set")
			assert.NotEmpty(t, reg.Description, "Description must be set")
			assert.NotNil(t, reg.Factory, "Factory must be set")
		})
	}
}

func TestRegisteredTools_FactoriesProduceValidTools(t *testing.T) {
	t.Parallel()

	cfg := ToolConfig{
		DAGsDir:               "/tmp/test-dags",
		RemoteContextResolver: &testRemoteContextResolver{},
		ControllerRuntime:     &testControllerRuntime{},
	}
	for _, reg := range RegisteredTools() {
		t.Run(reg.Name, func(t *testing.T) {
			t.Parallel()

			tool := reg.Factory(cfg)
			require.NotNil(t, tool)
			assert.Equal(t, "function", tool.Type)
			assert.Equal(t, reg.Name, tool.Function.Name)
			assert.NotEmpty(t, tool.Function.Description)
			assert.NotNil(t, tool.Run)
		})
	}
}

func TestCreateTools_UsesRegistry(t *testing.T) {
	t.Parallel()

	tools := CreateTools(ToolConfig{
		DAGsDir:               "/tmp/dags",
		RemoteContextResolver: &testRemoteContextResolver{},
		ControllerRuntime:     &testControllerRuntime{},
	})
	regs := RegisteredTools()

	assert.Len(t, tools, len(regs), "CreateTools should produce one tool per registration")

	toolNames := make(map[string]bool, len(tools))
	for _, tool := range tools {
		toolNames[tool.Function.Name] = true
	}

	for _, reg := range regs {
		assert.True(t, toolNames[reg.Name], "CreateTools missing tool %q", reg.Name)
	}
}

func TestKnownToolNames_DerivedFromRegistry(t *testing.T) {
	t.Parallel()

	known := KnownToolNames()
	registered := RegisteredToolNames()

	assert.Equal(t, registered, known,
		"KnownToolNames should return the same names as RegisteredToolNames")
}

func TestIsKnownToolName_DerivedFromRegistry(t *testing.T) {
	t.Parallel()

	for _, reg := range RegisteredTools() {
		assert.True(t, IsKnownToolName(reg.Name),
			"IsKnownToolName(%q) should be true", reg.Name)
	}

	assert.False(t, IsKnownToolName("nonexistent"))
}

func TestDefaultToolPolicy_DerivedFromRegistry(t *testing.T) {
	t.Parallel()

	policy := DefaultToolPolicy()
	regs := RegisteredTools()

	for _, reg := range regs {
		enabled, ok := policy.Tools[reg.Name]
		assert.True(t, ok, "DefaultToolPolicy missing tool %q", reg.Name)
		assert.Equal(t, reg.DefaultEnabled, enabled,
			"DefaultToolPolicy[%q] should match DefaultEnabled", reg.Name)
	}

	assert.Equal(t, BashDefaultBehaviorAllow, policy.Bash.DefaultBehavior)
	assert.Equal(t, BashDenyBehaviorAskUser, policy.Bash.DenyBehavior)
}
