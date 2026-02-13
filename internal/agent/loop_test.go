package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoop(t *testing.T) {
	t.Parallel()

	t.Run("initializes with config", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{name: "test"},
			Model:    "test-model",
			Tools:    CreateTools(""),
		})

		assert.NotNil(t, loop)
	})

	t.Run("uses default logger if not provided", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
		})

		assert.NotNil(t, loop)
	})
}

func TestLoop_QueueUserMessage(t *testing.T) {
	t.Parallel()

	t.Run("adds message to queue", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{Provider: &mockLLMProvider{}})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		loop.mu.Lock()
		defer loop.mu.Unlock()
		assert.Len(t, loop.messageQueue, 1)
		assert.Equal(t, "test", loop.messageQueue[0].Content)
	})
}

func TestLoop_Go(t *testing.T) {
	t.Parallel()

	t.Run("returns error with nil provider", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{Provider: nil})

		err := loop.Go(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no LLM provider")
	})

	t.Run("processes queued messages", func(t *testing.T) {
		t.Parallel()

		requestCh := make(chan *llm.ChatRequest, 1)
		provider := newCapturingProvider(requestCh, simpleStopResponse("response"))

		loop := NewLoop(LoopConfig{
			Provider: provider,
			Model:    "test-model",
		})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() { _ = loop.Go(ctx) }()

		capturedRequest := waitForRequest(t, requestCh, 400*time.Millisecond)
		assert.Equal(t, "test-model", capturedRequest.Model)
		assert.NotEmpty(t, capturedRequest.Messages)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		provider := &mockLLMProvider{
			chatFunc: func(ctx context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(10 * time.Second):
					return &llm.ChatResponse{Content: "late"}, nil
				}
			},
		}

		loop := NewLoop(LoopConfig{Provider: provider})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := loop.Go(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("calls OnWorking callback", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		var workingStates []bool

		loop := NewLoop(LoopConfig{
			Provider: newStopProvider("done"),
			OnWorking: func(working bool) {
				mu.Lock()
				workingStates = append(workingStates, working)
				mu.Unlock()
			},
		})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		runLoopForDuration(t, loop, 200*time.Millisecond)

		mu.Lock()
		states := append([]bool{}, workingStates...)
		mu.Unlock()

		assert.Contains(t, states, true)
		assert.Contains(t, states, false)
	})

	t.Run("records messages via callback", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		var recordedMessages []Message

		loop := NewLoop(LoopConfig{
			Provider:  newStopProvider("response"),
			SessionID: "conv-1",
			RecordMessage: func(_ context.Context, msg Message) error {
				mu.Lock()
				recordedMessages = append(recordedMessages, msg)
				mu.Unlock()
				return nil
			},
		})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		runLoopForDuration(t, loop, 200*time.Millisecond)

		mu.Lock()
		msgs := append([]Message{}, recordedMessages...)
		mu.Unlock()

		require.NotEmpty(t, msgs)
		assert.Equal(t, MessageTypeAssistant, msgs[0].Type)
		assert.Equal(t, "conv-1", msgs[0].SessionID)
	})

	t.Run("handles tool calls", func(t *testing.T) {
		t.Parallel()

		callCount := atomic.Int32{}
		done := make(chan struct{})
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				count := callCount.Add(1)
				if count == 1 {
					return &llm.ChatResponse{
						FinishReason: "tool_calls",
						ToolCalls: []llm.ToolCall{{
							ID:   "call-1",
							Type: "function",
							Function: llm.ToolCallFunction{
								Name:      "think",
								Arguments: `{"thought": "test"}`,
							},
						}},
					}, nil
				}
				close(done)
				return simpleStopResponse("done"), nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider: provider,
			Tools:    CreateTools(""),
		})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() { _ = loop.Go(ctx) }()

		select {
		case <-done:
		case <-time.After(800 * time.Millisecond):
			t.Fatal("timeout waiting for tool handling")
		}

		assert.Equal(t, int32(2), callCount.Load())
	})

	t.Run("includes system prompt", func(t *testing.T) {
		t.Parallel()

		requestCh := make(chan *llm.ChatRequest, 1)
		provider := newCapturingProvider(requestCh, simpleStopResponse("response"))

		loop := NewLoop(LoopConfig{
			Provider:     provider,
			SystemPrompt: "You are a helpful assistant.",
		})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() { _ = loop.Go(ctx) }()

		capturedRequest := waitForRequest(t, requestCh, 400*time.Millisecond)
		require.NotEmpty(t, capturedRequest.Messages)
		assert.Equal(t, llm.RoleSystem, capturedRequest.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", capturedRequest.Messages[0].Content)
	})

	t.Run("accumulates token usage", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{}, 1) // buffered to prevent blocking
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				defer func() {
					select {
					case done <- struct{}{}:
					default:
					}
				}()
				return &llm.ChatResponse{
					Content:      "response",
					FinishReason: "stop",
					Usage: llm.Usage{
						PromptTokens:     10,
						CompletionTokens: 20,
						TotalTokens:      30,
					},
				}, nil
			},
		}

		loop := NewLoop(LoopConfig{Provider: provider})
		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go func() { _ = loop.Go(ctx) }()

		select {
		case <-done:
			time.Sleep(10 * time.Millisecond)
		case <-time.After(1500 * time.Millisecond):
			t.Fatal("timeout waiting for LLM call")
		}

		loop.mu.Lock()
		usage := loop.totalUsage
		loop.mu.Unlock()

		assert.Equal(t, 10, usage.PromptTokens)
		assert.Equal(t, 20, usage.CompletionTokens)
		assert.Equal(t, 30, usage.TotalTokens)
	})
}

func TestLoop_ExecuteTool(t *testing.T) {
	t.Parallel()

	t.Run("executes known tool", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(""),
		})

		result := loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: `{"thought": "testing"}`,
			},
		})

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Thought recorded")
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(""),
		})

		result := loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "unknown_tool",
				Arguments: `{}`,
			},
		})

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("handles empty arguments", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(""),
		})

		result := loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: "",
			},
		})

		assert.False(t, result.IsError)
	})

	t.Run("invokes after hook", func(t *testing.T) {
		t.Parallel()

		hooks := NewHooks()
		var capturedInfo ToolExecInfo
		var capturedResult ToolOut

		hooks.OnAfterToolExec(func(_ context.Context, info ToolExecInfo, result ToolOut) {
			capturedInfo = info
			capturedResult = result
		})

		loop := NewLoop(LoopConfig{
			Provider:  &mockLLMProvider{},
			Tools:     CreateTools(""),
			SessionID: "conv-hook",
			UserID:    "user-1",
			Username:  "alice",
			IPAddress: "10.0.0.1",
			Role:      auth.RoleManager,
			Hooks:     hooks,
		})

		result := loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: `{"thought": "testing hooks"}`,
			},
		})

		assert.False(t, result.IsError)
		assert.Equal(t, "think", capturedInfo.ToolName)
		assert.Equal(t, "conv-hook", capturedInfo.SessionID)
		assert.Equal(t, "user-1", capturedInfo.UserID)
		assert.Equal(t, "alice", capturedInfo.Username)
		assert.Equal(t, "10.0.0.1", capturedInfo.IPAddress)
		assert.Equal(t, auth.RoleManager, capturedInfo.Role)
		assert.Equal(t, result.Content, capturedResult.Content)
		// think tool has nil Audit (not audited)
		assert.Nil(t, capturedInfo.Audit)
	})

	t.Run("populates Audit from tool", func(t *testing.T) {
		t.Parallel()

		hooks := NewHooks()
		var capturedInfo ToolExecInfo

		hooks.OnAfterToolExec(func(_ context.Context, info ToolExecInfo, _ ToolOut) {
			capturedInfo = info
		})

		auditInfo := &AuditInfo{
			Action: "test_action",
			DetailExtractor: func(input json.RawMessage) map[string]any {
				return map[string]any{"raw": string(input)}
			},
		}

		customTool := &AgentTool{
			Tool: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "audited_tool",
					Description: "A tool with audit info",
					Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
				},
			},
			Run: func(_ ToolContext, _ json.RawMessage) ToolOut {
				return ToolOut{Content: "ok"}
			},
			Audit: auditInfo,
		}

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    []*AgentTool{customTool},
			Hooks:    hooks,
		})

		loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "audited_tool",
				Arguments: `{"key":"val"}`,
			},
		})

		require.NotNil(t, capturedInfo.Audit)
		assert.Equal(t, "test_action", capturedInfo.Audit.Action)
		details := capturedInfo.Audit.DetailExtractor(capturedInfo.Input)
		assert.Equal(t, `{"key":"val"}`, details["raw"])
	})

	t.Run("before hook blocks execution", func(t *testing.T) {
		t.Parallel()

		hooks := NewHooks()
		hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
			return errors.New("forbidden")
		})

		toolCalled := false
		hooks.OnAfterToolExec(func(_ context.Context, _ ToolExecInfo, _ ToolOut) {
			toolCalled = true
		})

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(""),
			Hooks:    hooks,
		})

		result := loop.executeTool(context.Background(), llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: `{"thought": "should not run"}`,
			},
		})

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "Blocked by policy")
		assert.Contains(t, result.Content, "forbidden")
		assert.False(t, toolCalled, "after hook should not be called when before hook blocks")
	})
}

func TestLoop_BuildMessages(t *testing.T) {
	t.Parallel()

	t.Run("without system prompt", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider:     &mockLLMProvider{},
			SystemPrompt: "",
		})

		messages := loop.buildMessages([]llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		})

		assert.Len(t, messages, 1)
		assert.Equal(t, llm.RoleUser, messages[0].Role)
	})

	t.Run("with system prompt", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider:     &mockLLMProvider{},
			SystemPrompt: "Be helpful.",
		})

		messages := loop.buildMessages([]llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		})

		assert.Len(t, messages, 2)
		assert.Equal(t, llm.RoleSystem, messages[0].Role)
		assert.Equal(t, "Be helpful.", messages[0].Content)
		assert.Equal(t, llm.RoleUser, messages[1].Role)
	})
}

func TestLoop_BuildToolDefinitions(t *testing.T) {
	t.Parallel()

	t.Run("converts agent tools to LLM tools", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(""),
		})

		tools := loop.buildToolDefinitions()

		assert.Len(t, tools, 8)
		for _, tool := range tools {
			assert.Equal(t, "function", tool.Type)
			assert.NotEmpty(t, tool.Function.Name)
		}
	})
}

// Test helpers

func waitForRequest(t *testing.T, requestCh <-chan *llm.ChatRequest, timeout time.Duration) *llm.ChatRequest {
	t.Helper()
	select {
	case req := <-requestCh:
		require.NotNil(t, req)
		return req
	case <-time.After(timeout):
		t.Fatal("timeout waiting for LLM request")
		return nil
	}
}

func runLoopForDuration(t *testing.T, loop *Loop, duration time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() { _ = loop.Go(ctx) }()

	time.Sleep(duration)
}
