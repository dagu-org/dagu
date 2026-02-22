package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// singleTaskInput returns JSON for a batched delegate call with one task.
func singleTaskInput(task string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"tasks": [{"task": %q}]}`, task))
}

func TestDelegateTool_NoDelegateContext(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{}, singleTaskInput("test"))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not available")
}

func TestDelegateTool_EmptyTasksArray(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`{"tasks": []}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "At least one task")
}

func TestDelegateTool_EmptyTaskDescription(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: newStopProvider("ok"), Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, json.RawMessage(`{"tasks": [{"task": ""}]}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Task description cannot be empty")
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
	assert.Contains(t, props, "tasks")

	tasksSchema, ok := props["tasks"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", tasksSchema["type"])
	assert.Equal(t, maxConcurrentDelegates, tasksSchema["maxItems"])

	items, ok := tasksSchema["items"].(map[string]any)
	require.True(t, ok)
	itemProps, ok := items["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, itemProps, "task")

	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "tasks")
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
			Models:       []ModelSlot{{Provider: provider, Model: "test-model", Name: "test-model"}},
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
		},
	}, singleTaskInput("analyze data"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)
	assert.NotEmpty(t, result.DelegateIDs[0])

	// Verify sub-session created in store
	store.mu.Lock()
	defer store.mu.Unlock()

	sess, exists := store.sessions[result.DelegateIDs[0]]
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
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
		},
	}, singleTaskInput("do stuff"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	// Verify messages were recorded to sub-session (not parent)
	store.mu.Lock()
	defer store.mu.Unlock()

	msgs, exists := store.messages[result.DelegateIDs[0]]
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
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, singleTaskInput("compute"))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "this is the final answer")
}

func TestDelegateTool_ReturnsDelegateIDs(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("ok")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, singleTaskInput("something"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)
	// Should be a valid UUID
	_, err := uuid.Parse(result.DelegateIDs[0])
	assert.NoError(t, err, "DelegateID should be a valid UUID")
}

func TestDelegateTool_MultipleTasks(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("task done")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
		},
	}, json.RawMessage(`{"tasks": [{"task": "task A"}, {"task": "task B"}, {"task": "task C"}]}`))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 3)

	// Each delegate ID should be unique and valid UUID
	seen := make(map[string]bool)
	for _, id := range result.DelegateIDs {
		assert.NotEmpty(t, id)
		_, err := uuid.Parse(id)
		assert.NoError(t, err)
		assert.False(t, seen[id], "delegate IDs should be unique")
		seen[id] = true
	}

	// Content should reference all tasks
	assert.Contains(t, result.Content, "task A")
	assert.Contains(t, result.Content, "task B")
	assert.Contains(t, result.Content, "task C")
}

func TestDelegateTool_ExceedsMaxTasks(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("ok")

	// Build more than maxConcurrentDelegates tasks
	var tasks []delegateTask
	for i := range maxConcurrentDelegates + 4 {
		tasks = append(tasks, delegateTask{Task: fmt.Sprintf("task %d", i)})
	}
	input, err := json.Marshal(delegateInput{Tasks: tasks})
	require.NoError(t, err)

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, input)

	// Should succeed but only run maxConcurrentDelegates tasks
	assert.False(t, result.IsError)
	assert.Len(t, result.DelegateIDs, maxConcurrentDelegates)
}

func TestDelegateTool_PartialFailure(t *testing.T) {
	t.Parallel()

	callCount := 0
	var mu sync.Mutex
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			// Alternate: odd calls succeed, even calls fail
			if n%2 == 0 {
				return nil, fmt.Errorf("provider error")
			}
			return &llm.ChatResponse{Content: "success", FinishReason: "stop"}, nil
		},
	}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, json.RawMessage(`{"tasks": [{"task": "task1"}, {"task": "task2"}]}`))

	// Not all failed, so IsError should be false
	assert.False(t, result.IsError)
	assert.Len(t, result.DelegateIDs, 2)
}

func TestDelegateTool_AllTasksFail(t *testing.T) {
	t.Parallel()

	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			return nil, fmt.Errorf("provider down")
		},
	}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
		},
	}, json.RawMessage(`{"tasks": [{"task": "fail1"}, {"task": "fail2"}]}`))

	assert.True(t, result.IsError)
	assert.Len(t, result.DelegateIDs, 2)
	assert.Contains(t, result.Content, "ERROR")
}

func TestDelegateTool_WithoutSessionStore(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("no store result")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  []*AgentTool{},
			// No SessionStore
		},
	}, singleTaskInput("stateless task"))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "no store result")
	require.Len(t, result.DelegateIDs, 1)
	assert.NotEmpty(t, result.DelegateIDs[0])
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
			Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:  parentTools,
		},
	}, singleTaskInput("check tools"))

	require.NotNil(t, capturedReq)

	// The child should have tools but NOT delegate
	for _, tool := range capturedReq.Tools {
		assert.NotEqual(t, "delegate", tool.Function.Name,
			"child loop should not have delegate tool")
	}
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
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
		},
	}, singleTaskInput("fail"))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Sub-agent failed")
	require.Len(t, result.DelegateIDs, 1)
	assert.NotEmpty(t, result.DelegateIDs[0])
}

func TestDelegateTool_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	provider := &mockLLMProvider{
		chatFunc: func(ctx context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
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
				Models: []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
				Tools:  []*AgentTool{},
			},
		}, singleTaskInput("cancelled"))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		require.Len(t, result.DelegateIDs, 1)
		assert.NotEmpty(t, result.DelegateIDs[0])
	case <-time.After(5 * time.Second):
		t.Fatal("delegate did not return after context cancellation")
	}
}

func TestDelegateTool_RegistersSubSession(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("sub result")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("register test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	registry.mu.Lock()
	defer registry.mu.Unlock()
	mgr, ok := registry.registered[result.DelegateIDs[0]]
	assert.True(t, ok)
	assert.NotNil(t, mgr)
}

func TestDelegateTool_NotifiesParentStarted(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("sub result")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("notify test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	registry.mu.Lock()
	defer registry.mu.Unlock()
	var events []DelegateEvent
	for _, ev := range registry.events {
		if ev.DelegateEvent != nil {
			events = append(events, *ev.DelegateEvent)
		}
	}
	require.GreaterOrEqual(t, len(events), 1)
	assert.Equal(t, DelegateEventStarted, events[0].Type)
	assert.Equal(t, result.DelegateIDs[0], events[0].DelegateID)
	assert.Equal(t, "notify test", events[0].Task)
}

func TestDelegateTool_NotifiesParentCompleted(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("done")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("complete test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	registry.mu.Lock()
	defer registry.mu.Unlock()
	var events []DelegateEvent
	for _, ev := range registry.events {
		if ev.DelegateEvent != nil {
			events = append(events, *ev.DelegateEvent)
		}
	}
	require.Len(t, events, 2) // started + completed
	assert.Equal(t, DelegateEventStarted, events[0].Type)
	assert.Equal(t, DelegateEventCompleted, events[1].Type)
	assert.Equal(t, result.DelegateIDs[0], events[1].DelegateID)
	assert.Equal(t, "complete test", events[1].Task)
	assert.GreaterOrEqual(t, events[1].Cost, float64(0))
}

func TestDelegateTool_MultipleTasksNotifications(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("done")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, json.RawMessage(`{"tasks": [{"task": "task A"}, {"task": "task B"}]}`))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 2)

	registry.mu.Lock()
	defer registry.mu.Unlock()
	var events []DelegateEvent
	for _, ev := range registry.events {
		if ev.DelegateEvent != nil {
			events = append(events, *ev.DelegateEvent)
		}
	}
	// 2 tasks × 2 events (started + completed) = 4 events
	require.Len(t, events, 4)

	startedCount := 0
	completedCount := 0
	for _, e := range events {
		switch e.Type {
		case DelegateEventStarted:
			startedCount++
		case DelegateEventCompleted:
			completedCount++
		}
	}
	assert.Equal(t, 2, startedCount)
	assert.Equal(t, 2, completedCount)
}

func TestDelegateTool_CostRolledUpToParent(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("done")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("cost test"))

	assert.False(t, result.IsError)

	registry.mu.Lock()
	defer registry.mu.Unlock()
	assert.GreaterOrEqual(t, registry.cost, float64(0))
}

func TestDelegateTool_SubSessionStreamable(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("streamed output")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("stream test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	registry.mu.Lock()
	registeredMgr := registry.registered[result.DelegateIDs[0]]
	registry.mu.Unlock()
	require.NotNil(t, registeredMgr)

	msgs := registeredMgr.GetMessages()
	assert.NotEmpty(t, msgs, "sub-SessionManager should have recorded messages")

	var hasAssistant bool
	for _, msg := range msgs {
		if msg.Type == MessageTypeAssistant {
			hasAssistant = true
			break
		}
	}
	assert.True(t, hasAssistant, "sub-SessionManager should have an assistant message")
}

func TestDelegateTool_SubSessionWorkingState(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	provider := newStopProvider("done")
	tool := NewDelegateTool()
	tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("working state test"))

	registry.mu.Lock()
	defer registry.mu.Unlock()
	require.NotEmpty(t, registry.registered)
	for _, mgr := range registry.registered {
		assert.False(t, mgr.IsWorking(), "sub-SessionManager should not be working after delegate completes")
	}
}

func TestDelegateTool_NoCallbacks(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("no callbacks result")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
		},
	}, singleTaskInput("no callbacks test"))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "no callbacks result")
	require.Len(t, result.DelegateIDs, 1)
	assert.NotEmpty(t, result.DelegateIDs[0])
}

func TestDelegateTool_SubAgentHookUserAttribution(t *testing.T) {
	t.Parallel()

	// Set up hooks to capture ToolExecInfo from the child loop's tool execution.
	hooks := NewHooks()
	var mu sync.Mutex
	var capturedInfos []ToolExecInfo

	hooks.OnAfterToolExec(func(_ context.Context, info ToolExecInfo, _ ToolOut) {
		mu.Lock()
		capturedInfos = append(capturedInfos, info)
		mu.Unlock()
	})

	// Provider: first call returns a tool call for "think", second call returns stop.
	provider := newSequenceProvider(
		&llm.ChatResponse{
			FinishReason: "tool_calls",
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      "think",
					Arguments: `{"thought": "sub-agent thinking"}`,
				},
			}},
		},
		&llm.ChatResponse{Content: "done", FinishReason: "stop"},
	)

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        CreateTools(ToolConfig{}),
			Hooks:        hooks,
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-1",
			User: UserIdentity{
				UserID:    "user-42",
				Username:  "bob",
				IPAddress: "192.168.1.100",
				Role:      auth.RoleManager,
			},
		},
	}, singleTaskInput("test user attribution"))

	assert.False(t, result.IsError)

	mu.Lock()
	defer mu.Unlock()

	// The "think" tool should have been executed by the child loop.
	require.NotEmpty(t, capturedInfos, "hook should have captured at least one tool execution")

	var thinkInfo *ToolExecInfo
	for i := range capturedInfos {
		if capturedInfos[i].ToolName == "think" {
			thinkInfo = &capturedInfos[i]
			break
		}
	}
	require.NotNil(t, thinkInfo, "should have captured 'think' tool execution")

	assert.Equal(t, "user-42", thinkInfo.User.UserID)
	assert.Equal(t, "bob", thinkInfo.User.Username)
	assert.Equal(t, "192.168.1.100", thinkInfo.User.IPAddress)
	assert.Equal(t, auth.RoleManager, thinkInfo.User.Role)
}

func TestDelegateTool_ForwardedMessagesHaveSessionID(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	store := newMockSessionStore()
	provider := newStopProvider("sub output")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("session id test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)
	delegateID := result.DelegateIDs[0]

	// Check that registered sub-SessionManager has messages with the correct session ID.
	registry.mu.Lock()
	mgr := registry.registered[delegateID]
	registry.mu.Unlock()
	require.NotNil(t, mgr)

	msgs := mgr.GetMessages()
	require.NotEmpty(t, msgs)
	for _, msg := range msgs {
		assert.Equal(t, delegateID, msg.SessionID,
			"forwarded message should have the delegate's session ID, got %q", msg.SessionID)
	}
}

func TestDelegateTool_SubSessionHasUserContext(t *testing.T) {
	t.Parallel()

	registry := newMockSubSessionRegistry()
	store := newMockSessionStore()
	provider := newStopProvider("ok")

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: store,
			ParentID:     "parent-1",
			User: UserIdentity{
				UserID:    "user-42",
				Username:  "alice",
				IPAddress: "10.0.0.1",
				Role:      auth.RoleViewer,
			},
			Registry: registry,
		},
	}, singleTaskInput("user context test"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	// The persisted sub-session should carry the parent's user ID.
	store.mu.Lock()
	sess, exists := store.sessions[result.DelegateIDs[0]]
	store.mu.Unlock()
	require.True(t, exists)
	assert.Equal(t, "user-42", sess.UserID)
}

func TestDelegateTool_ParentTracksDelegate(t *testing.T) {
	t.Parallel()

	// Use a real SessionManager as the parent so SetDelegateStarted/Completed actually run.
	parentMgr := NewSessionManager(SessionManagerConfig{
		ID:   "parent-track",
		User: UserIdentity{UserID: "user-1"},
	})

	// Create a registry that delegates to the real parent manager.
	registry := &mockSubSessionRegistry{
		registered: make(map[string]*SessionManager),
		parent:     parentMgr,
	}

	provider := newStopProvider("tracked")
	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:       []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:        []*AgentTool{},
			SessionStore: newMockSessionStore(),
			ParentID:     "parent-track",
			User:         UserIdentity{UserID: "user-1"},
			Registry:     registry,
		},
	}, singleTaskInput("track delegate"))

	assert.False(t, result.IsError)
	require.Len(t, result.DelegateIDs, 1)

	// The parent manager's delegates map should track this delegate as completed.
	delegates := parentMgr.GetDelegates()
	require.Len(t, delegates, 1)
	assert.Equal(t, result.DelegateIDs[0], delegates[0].ID)
	assert.Equal(t, DelegateStatusCompleted, delegates[0].Status)
	assert.Equal(t, "track delegate", delegates[0].Task)
}

func TestDelegateTool_SkillsPreloaded(t *testing.T) {
	t.Parallel()

	var capturedReq *llm.ChatRequest
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			capturedReq = req
			return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
		},
	}

	store := &testSkillStore{
		skills: []*Skill{
			{ID: "sql-expert", Name: "SQL Expert", Knowledge: "SELECT * FROM users;"},
		},
	}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:     []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:      []*AgentTool{},
			SkillStore: store,
		},
	}, json.RawMessage(`{"tasks": [{"task": "write a query", "skills": ["sql-expert"]}]}`))

	assert.False(t, result.IsError)
	require.NotNil(t, capturedReq)

	// The sub-agent's user message should contain pre-loaded skill knowledge.
	require.NotEmpty(t, capturedReq.Messages)
	var userContent string
	for _, msg := range capturedReq.Messages {
		if msg.Role == llm.RoleUser {
			userContent = msg.Content
		}
	}
	assert.Contains(t, userContent, `<skill name="SQL Expert" id="sql-expert">`)
	assert.Contains(t, userContent, "SELECT * FROM users;")
	assert.Contains(t, userContent, "Task: write a query")
}

func TestDelegateTool_SkillsNotFoundWarning(t *testing.T) {
	t.Parallel()

	provider := newStopProvider("done without skill")
	store := &testSkillStore{skills: []*Skill{}}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:     []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:      []*AgentTool{},
			SkillStore: store,
		},
	}, json.RawMessage(`{"tasks": [{"task": "do work", "skills": ["nonexistent"]}]}`))

	// Should succeed — missing skills are warnings, not errors.
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "done without skill")
}

func TestDelegateTool_SkillsNotAllowed(t *testing.T) {
	t.Parallel()

	var capturedReq *llm.ChatRequest
	provider := &mockLLMProvider{
		chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			capturedReq = req
			return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
		},
	}

	store := &testSkillStore{
		skills: []*Skill{
			{ID: "restricted-skill", Name: "Restricted", Knowledge: "secret knowledge"},
		},
	}

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:    context.Background(),
		WorkingDir: t.TempDir(),
		Delegate: &DelegateContext{
			Models:        []ModelSlot{{Provider: provider, Model: "test", Name: "test"}},
			Tools:         []*AgentTool{},
			SkillStore:    store,
			AllowedSkills: map[string]struct{}{"other-skill": {}},
		},
	}, json.RawMessage(`{"tasks": [{"task": "do work", "skills": ["restricted-skill"]}]}`))

	assert.False(t, result.IsError)
	require.NotNil(t, capturedReq)

	// The sub-agent's user message should NOT contain the restricted skill.
	var userContent string
	for _, msg := range capturedReq.Messages {
		if msg.Role == llm.RoleUser {
			userContent = msg.Content
		}
	}
	assert.NotContains(t, userContent, "secret knowledge")
	assert.NotContains(t, userContent, "<skill")
}

func TestDelegateTool_EmptySkillIDRejected(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`{"tasks": [{"task": "test", "skills": [""]}]}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Skill ID cannot be empty")
}

func TestDelegateTool_Schema_IncludesSkills(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	params := tool.Function.Parameters

	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)

	tasksSchema, ok := props["tasks"].(map[string]any)
	require.True(t, ok)

	items, ok := tasksSchema["items"].(map[string]any)
	require.True(t, ok)

	itemProps, ok := items["properties"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, itemProps, "skills")
	skillsSchema, ok := itemProps["skills"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", skillsSchema["type"])
}
