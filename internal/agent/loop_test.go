package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoop(t *testing.T) {
	t.Parallel()

	t.Run("initializes with config", func(t *testing.T) {
		t.Parallel()

		provider := &mockLLMProvider{name: "test"}
		loop := NewLoop(LoopConfig{
			Provider: provider,
			Model:    "test-model",
			Tools:    CreateTools(),
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

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
		})

		msg := llm.Message{Role: llm.RoleUser, Content: "test"}
		loop.QueueUserMessage(msg)

		// Verify message is queued by checking internal state
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
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
				select {
				case requestCh <- req:
				default:
				}
				return &llm.ChatResponse{Content: "response", FinishReason: "stop"}, nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider: provider,
			Model:    "test-model",
		})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		// Wait for captured request
		select {
		case capturedRequest := <-requestCh:
			require.NotNil(t, capturedRequest)
			assert.Equal(t, "test-model", capturedRequest.Model)
			assert.NotEmpty(t, capturedRequest.Messages)
		case <-time.After(400 * time.Millisecond):
			t.Fatal("timeout waiting for LLM request")
		}
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
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider: provider,
			OnWorking: func(working bool) {
				mu.Lock()
				workingStates = append(workingStates, working)
				mu.Unlock()
			},
		})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		states := append([]bool{}, workingStates...)
		mu.Unlock()

		// Should have called with true (working) and false (done)
		assert.Contains(t, states, true)
		assert.Contains(t, states, false)
	})

	t.Run("records messages via callback", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		var recordedMessages []Message
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Content: "response", FinishReason: "stop"}, nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider:       provider,
			ConversationID: "conv-1",
			RecordMessage: func(_ context.Context, msg Message) error {
				mu.Lock()
				recordedMessages = append(recordedMessages, msg)
				mu.Unlock()
				return nil
			},
		})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		msgs := append([]Message{}, recordedMessages...)
		mu.Unlock()

		require.NotEmpty(t, msgs)
		assert.Equal(t, MessageTypeAssistant, msgs[0].Type)
		assert.Equal(t, "conv-1", msgs[0].ConversationID)
	})

	t.Run("handles tool calls", func(t *testing.T) {
		t.Parallel()

		callCount := atomic.Int32{}
		done := make(chan struct{})
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
				count := callCount.Add(1)
				if count == 1 {
					// First call: request tool call
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
				// Second call: return final response
				close(done)
				return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider: provider,
			Tools:    CreateTools(),
		})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "test"})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		select {
		case <-done:
			// Success - tool handling completed
		case <-time.After(800 * time.Millisecond):
			t.Fatal("timeout waiting for tool handling")
		}

		// Should have made two LLM calls (initial + after tool)
		assert.Equal(t, int32(2), callCount.Load())
	})

	t.Run("includes system prompt", func(t *testing.T) {
		t.Parallel()

		requestCh := make(chan *llm.ChatRequest, 1)
		provider := &mockLLMProvider{
			chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
				select {
				case requestCh <- req:
				default:
				}
				return &llm.ChatResponse{Content: "response", FinishReason: "stop"}, nil
			},
		}

		loop := NewLoop(LoopConfig{
			Provider:     provider,
			SystemPrompt: "You are a helpful assistant.",
		})

		loop.QueueUserMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		select {
		case capturedRequest := <-requestCh:
			require.NotNil(t, capturedRequest)
			require.NotEmpty(t, capturedRequest.Messages)
			assert.Equal(t, llm.RoleSystem, capturedRequest.Messages[0].Role)
			assert.Equal(t, "You are a helpful assistant.", capturedRequest.Messages[0].Content)
		case <-time.After(400 * time.Millisecond):
			t.Fatal("timeout waiting for LLM request")
		}
	})

	t.Run("accumulates token usage", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
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

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			_ = loop.Go(ctx)
		}()

		select {
		case <-done:
			// Wait a tiny bit for accumulateUsage to complete
			time.Sleep(10 * time.Millisecond)
		case <-time.After(400 * time.Millisecond):
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
			Tools:    CreateTools(),
		})

		tc := llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: `{"thought": "testing"}`,
			},
		}

		result := loop.executeTool(context.Background(), tc)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Thought recorded")
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(),
		})

		tc := llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "unknown_tool",
				Arguments: `{}`,
			},
		}

		result := loop.executeTool(context.Background(), tc)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("handles empty arguments", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider: &mockLLMProvider{},
			Tools:    CreateTools(),
		})

		tc := llm.ToolCall{
			ID:   "test-id",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "think",
				Arguments: "",
			},
		}

		result := loop.executeTool(context.Background(), tc)

		// Should work with empty arguments (defaults to {})
		assert.False(t, result.IsError)
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

		history := []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}

		messages := loop.buildMessages(history)

		assert.Len(t, messages, 1)
		assert.Equal(t, llm.RoleUser, messages[0].Role)
	})

	t.Run("with system prompt", func(t *testing.T) {
		t.Parallel()

		loop := NewLoop(LoopConfig{
			Provider:     &mockLLMProvider{},
			SystemPrompt: "Be helpful.",
		})

		history := []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}

		messages := loop.buildMessages(history)

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
			Tools:    CreateTools(),
		})

		tools := loop.buildToolDefinitions()

		assert.Len(t, tools, 6)
		for _, tool := range tools {
			assert.Equal(t, "function", tool.Type)
			assert.NotEmpty(t, tool.Function.Name)
		}
	})
}
