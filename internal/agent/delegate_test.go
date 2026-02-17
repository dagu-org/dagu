package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegateTool_NoDelegateContext(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{}, json.RawMessage(`{"task": "test"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not available")
}

func TestDelegateTool_EmptyTask(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`{"task": ""}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "required")
}

func TestDelegateTool_InvalidInput(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`invalid json`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Invalid input")
}

func TestDelegateTool_Schema(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "delegate", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Function.Parameters)

	params := tool.Function.Parameters
	require.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "task")
	assert.Contains(t, props, "max_iterations")

	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "task")
}

func TestFilterOutTool(t *testing.T) {
	t.Parallel()

	tools := []*AgentTool{
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "bash"}}},
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "delegate"}}},
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "read"}}},
	}

	t.Run("removes named tool", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "delegate")
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.NotEqual(t, "delegate", tool.Function.Name)
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "delegate")
		require.Len(t, filtered, 2)
		assert.Equal(t, "bash", filtered[0].Function.Name)
		assert.Equal(t, "read", filtered[1].Function.Name)
	})

	t.Run("no-op for unknown name", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "unknown")
		assert.Len(t, filtered, 3)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(nil, "delegate")
		assert.Empty(t, filtered)
	})
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, truncate(tc.input, tc.maxLen))
		})
	}
}

func TestDelegateTool_PersistsSubSession(t *testing.T) {
	t.Parallel()

	store := newMockSessionStore()
	provider := newStopProvider("sub result")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider:     provider,
			Model:        "test-model",
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			UserID:       "user-1",
		},
	}, json.RawMessage(`{"task": "analyze data"}`))

	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.DelegateID)

	// Verify sub-session created in store
	store.mu.Lock()
	defer store.mu.Unlock()

	sess, exists := store.sessions[result.DelegateID]
	require.True(t, exists, "sub-session should exist in store")
	assert.Equal(t, "parent-1", sess.ParentSessionID)
	assert.Equal(t, "analyze data", sess.DelegateTask)
	assert.Equal(t, "user-1", sess.UserID)
}

func TestDelegateTool_RecordsMessagesToSubSession(t *testing.T) {
	t.Parallel()

	store := newMockSessionStore()
	provider := newStopProvider("sub agent output")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider:     provider,
			Model:        "test",
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			UserID:       "user-1",
		},
	}, json.RawMessage(`{"task": "do stuff"}`))

	assert.False(t, result.IsError)

	// Verify messages were recorded to sub-session (not parent)
	store.mu.Lock()
	defer store.mu.Unlock()

	msgs, exists := store.messages[result.DelegateID]
	require.True(t, exists, "sub-session should have messages")
	assert.GreaterOrEqual(t, len(msgs), 1, "should have at least assistant message")

	// Verify no messages were added to a non-existent parent session
	_, parentMsgsExist := store.messages["parent-1"]
	assert.False(t, parentMsgsExist, "parent session should not have messages from sub-agent")
}

func TestDelegateTool_ReturnsLastAssistantContent(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("this is the final answer")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    []*AgentTool{},
		},
	}, json.RawMessage(`{"task": "compute"}`))

	assert.False(t, result.IsError)
	assert.Equal(t, "this is the final answer", result.Content)
}

func TestDelegateTool_ReturnsDelegateID(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("ok")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    []*AgentTool{},
		},
	}, json.RawMessage(`{"task": "something"}`))

	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.DelegateID)
	// Should be a valid UUID
	_, err := uuid.Parse(result.DelegateID)
	assert.NoError(t, err, "DelegateID should be a valid UUID")
}

func TestDelegateTool_WithoutSessionStore(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("no store result")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    []*AgentTool{},
			// No SessionStore
		},
	}, json.RawMessage(`{"task": "stateless task"}`))

	assert.False(t, result.IsError)
	assert.Equal(t, "no store result", result.Content)
	assert.NotEmpty(t, result.DelegateID)
}

func TestDelegateTool_ChildHasNoDelegateTool(t *testing.T) {
	t.Parallel()

	var capturedReq *llm.ChatRequest
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			capturedReq = req
			return &llm.ChatResponse{Content: "ok", FinishReason: "stop"}, nil
		},
	}

	parentTools := []*AgentTool{
		NewDelegateTool(),
		{
			Tool: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "bash",
					Description: "run bash",
					Parameters:  map[string]any{"type": "object"},
				},
			},
			Run: func(_ ToolContext, _ json.RawMessage) ToolOut { return ToolOut{Content: "ok"} },
		},
	}

	tool := NewDelegateTool()
	tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    parentTools,
		},
	}, json.RawMessage(`{"task": "check tools"}`))

	require.NotNil(t, capturedReq)

	// The child should have tools but NOT delegate
	for _, tool := range capturedReq.Tools {
		assert.NotEqual(t, "delegate", tool.Function.Name,
			"child loop should not have delegate tool")
	}
}

func TestDelegateTool_MaxIterationsDefault(t *testing.T) {
	t.Parallel()

	var callCount int
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			callCount++
			// Always return stop, but the onWorking callback tracks iterations
			return &llm.ChatResponse{Content: fmt.Sprintf("iter %d", callCount), FinishReason: "stop"}, nil
		},
	}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    []*AgentTool{},
		},
	}, json.RawMessage(`{"task": "iterate"}`))

	assert.False(t, result.IsError)
	// With a simple stop provider, only 1 LLM call needed
	assert.GreaterOrEqual(t, callCount, 1)
}

func TestDelegateTool_MaxIterationsCustom(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("done")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider: provider,
			Model:    "test",
			Tools:    []*AgentTool{},
		},
	}, json.RawMessage(`{"task": "limited", "max_iterations": 2}`))

	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
}

func TestDelegateTool_ProviderError(t *testing.T) {
	t.Parallel()

	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			return nil, fmt.Errorf("provider unavailable")
		},
	}

	store := newMockSessionStore()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Provider:     provider,
			Model:        "test",
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			UserID:       "user-1",
		},
	}, json.RawMessage(`{"task": "fail"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Sub-agent failed")
	assert.NotEmpty(t, result.DelegateID)
}

func TestDelegateTool_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	provider := &mockLLMProvider{
		chatFunc: func(ctx context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			// Block until context cancelled
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	done := make(chan ToolOut, 1)
	tool := NewDelegateTool()
	go func() {
		done <- tool.Run(ToolContext{
			Context:    ctx,
			WorkingDir: t.TempDir(),
			Delegate: &DelegateContext{
				Provider: provider,
				Model:    "test",
				Tools:    []*AgentTool{},
			},
		}, json.RawMessage(`{"task": "cancelled"}`))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		// Should return gracefully (either error or empty content)
		assert.NotEmpty(t, result.DelegateID)
	case <-time.After(5 * time.Second):
		t.Fatal("delegate did not return after context cancellation")
	}
}
