package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversationManager(t *testing.T) {
	t.Parallel()

	t.Run("generates ID if not provided", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		assert.NotEmpty(t, cm.ID())
	})

	t.Run("uses provided ID", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{ID: "custom-id"})
		assert.Equal(t, "custom-id", cm.ID())
	})

	t.Run("stores user ID", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{UserID: "user-123"})
		assert.Equal(t, "user-123", cm.UserID())
	})

	t.Run("copies history messages", func(t *testing.T) {
		t.Parallel()

		history := []Message{
			{ID: "1", Content: "first"},
			{ID: "2", Content: "second"},
		}

		cm := NewConversationManager(ConversationManagerConfig{
			History: history,
		})

		msgs := cm.GetMessages()
		assert.Len(t, msgs, 2)
		assert.Equal(t, "first", msgs[0].Content)
		assert.Equal(t, "second", msgs[1].Content)
	})
}

func TestConversationManager_SetWorking(t *testing.T) {
	t.Parallel()

	t.Run("updates working state", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{ID: "test"})

		assert.False(t, cm.IsWorking())

		cm.SetWorking(true)
		assert.True(t, cm.IsWorking())

		cm.SetWorking(false)
		assert.False(t, cm.IsWorking())
	})

	t.Run("calls callback on state change", func(t *testing.T) {
		t.Parallel()

		type workingChange struct {
			id      string
			working bool
		}

		var mu sync.Mutex
		var callbackCalls []workingChange

		cm := NewConversationManager(ConversationManagerConfig{
			ID: "test-conv",
			OnWorkingChange: func(id string, working bool) {
				mu.Lock()
				callbackCalls = append(callbackCalls, workingChange{id, working})
				mu.Unlock()
			},
		})

		cm.SetWorking(true)
		cm.SetWorking(true) // Duplicate should be ignored
		cm.SetWorking(false)

		mu.Lock()
		calls := append([]workingChange{}, callbackCalls...)
		mu.Unlock()

		require.Len(t, calls, 2)
		assert.True(t, calls[0].working)
		assert.False(t, calls[1].working)
		assert.Equal(t, "test-conv", calls[0].id)
	})

	t.Run("broadcasts to subscribers", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{ID: "test"})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := cm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			cm.SetWorking(true)
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, ok := next()
			if ok && resp.ConversationState != nil {
				assert.True(t, resp.ConversationState.Working)
			}
		}()

		select {
		case <-done:
			// Success - broadcast received
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for broadcast")
		}
	})
}

func TestConversationManager_AcceptUserMessage(t *testing.T) {
	t.Parallel()

	t.Run("requires provider", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		err := cm.AcceptUserMessage(context.Background(), nil, "model", "model", "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("starts loop and queues message", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("hi")

		cm := NewConversationManager(ConversationManagerConfig{})
		err := cm.AcceptUserMessage(context.Background(), provider, "model", "model", "hello")

		require.NoError(t, err)
		assert.True(t, cm.IsWorking())

		_ = cm.Cancel(context.Background())
	})

	t.Run("adds message to conversation", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("response")

		cm := NewConversationManager(ConversationManagerConfig{})
		_ = cm.AcceptUserMessage(context.Background(), provider, "model", "model", "hello")

		time.Sleep(50 * time.Millisecond)

		msgs := cm.GetMessages()
		require.NotEmpty(t, msgs)
		assert.Equal(t, MessageTypeUser, msgs[0].Type)
		assert.Equal(t, "hello", msgs[0].Content)

		_ = cm.Cancel(context.Background())
	})
}

func TestConversationManager_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("receives messages", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{ID: "test"})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := cm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			cm.SetWorking(true)
		}()

		resp, ok := next()
		assert.True(t, ok)
		assert.NotNil(t, resp.ConversationState)
	})
}

func TestConversationManager_SubscribeWithSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("returns initial snapshot", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			ID:     "test",
			UserID: "user1",
			History: []Message{
				{ID: "1", Content: "first"},
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		snapshot, _ := cm.SubscribeWithSnapshot(ctx)

		assert.NotNil(t, snapshot.Conversation)
		assert.Equal(t, "test", snapshot.Conversation.ID)
		assert.NotNil(t, snapshot.ConversationState)
		assert.Len(t, snapshot.Messages, 1)
	})
}

func TestConversationManager_Cancel(t *testing.T) {
	t.Parallel()

	t.Run("cancels active loop", func(t *testing.T) {
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

		cm := NewConversationManager(ConversationManagerConfig{})
		_ = cm.AcceptUserMessage(context.Background(), provider, "model", "model", "hello")

		time.Sleep(50 * time.Millisecond)

		err := cm.Cancel(context.Background())
		assert.NoError(t, err)
		assert.False(t, cm.IsWorking())
	})

	t.Run("safe to call when no loop", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		err := cm.Cancel(context.Background())
		assert.NoError(t, err)
	})
}

func TestConversationManager_GetMessages(t *testing.T) {
	t.Parallel()

	t.Run("returns copy of messages", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			History: []Message{
				{ID: "1", Content: "first"},
				{ID: "2", Content: "second"},
			},
		})

		msgs := cm.GetMessages()
		assert.Len(t, msgs, 2)

		// Verify it's a copy (modification shouldn't affect original)
		msgs[0].Content = "modified"
		originalMsgs := cm.GetMessages()
		assert.Equal(t, "first", originalMsgs[0].Content)
	})
}

func TestConversationManager_GetConversation(t *testing.T) {
	t.Parallel()

	t.Run("returns conversation metadata", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			ID:     "conv-123",
			UserID: "user-456",
		})

		conv := cm.GetConversation()

		assert.Equal(t, "conv-123", conv.ID)
		assert.Equal(t, "user-456", conv.UserID)
		assert.False(t, conv.CreatedAt.IsZero())
	})
}

func TestConversationManager_GetModel(t *testing.T) {
	t.Parallel()

	t.Run("returns empty before accept", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		assert.Empty(t, cm.GetModel())
	})

	t.Run("returns model after accept", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("hi")

		cm := NewConversationManager(ConversationManagerConfig{})
		_ = cm.AcceptUserMessage(context.Background(), provider, "test-model", "test-model", "hello")

		assert.Equal(t, "test-model", cm.GetModel())

		_ = cm.Cancel(context.Background())
	})
}

func TestConversationManager_CostTracking(t *testing.T) {
	t.Parallel()

	t.Run("calculateCost with usage", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			InputCostPer1M:  3.0,
			OutputCostPer1M: 15.0,
		})

		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		cost := cm.calculateCost(usage)

		// (1000 * 3 / 1_000_000) + (500 * 15 / 1_000_000) = 0.003 + 0.0075 = 0.0105
		assert.InDelta(t, 0.0105, cost, 1e-9)
	})

	t.Run("calculateCost with nil usage", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			InputCostPer1M:  3.0,
			OutputCostPer1M: 15.0,
		})

		cost := cm.calculateCost(nil)
		assert.Equal(t, 0.0, cost)
	})

	t.Run("addCost accumulates", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})

		cm.addCost(0.01)
		cm.addCost(0.01)
		assert.InDelta(t, 0.02, cm.GetTotalCost(), 1e-9)
	})

	t.Run("GetTotalCost starts at zero", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		assert.Equal(t, 0.0, cm.GetTotalCost())
	})

	t.Run("calculateCost with zero pricing", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})

		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		cost := cm.calculateCost(usage)
		assert.Equal(t, 0.0, cost)
	})
}

func TestConversationManager_UpdatePricing(t *testing.T) {
	t.Parallel()

	t.Run("updates pricing for subsequent cost calculations", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})

		// Initially zero pricing
		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		cost := cm.calculateCost(usage)
		assert.Equal(t, 0.0, cost)

		// Update pricing
		cm.UpdatePricing(5.0, 25.0)

		cost = cm.calculateCost(usage)
		// (1000 * 5 / 1_000_000) + (500 * 25 / 1_000_000) = 0.005 + 0.0125 = 0.0175
		assert.InDelta(t, 0.0175, cost, 1e-9)
	})
}

func TestConversationManager_LastActivity(t *testing.T) {
	t.Parallel()

	t.Run("initial LastActivity is recent", func(t *testing.T) {
		t.Parallel()

		before := time.Now()
		cm := NewConversationManager(ConversationManagerConfig{})
		after := time.Now()

		la := cm.LastActivity()
		assert.False(t, la.Before(before), "LastActivity should not be before creation")
		assert.False(t, la.After(after), "LastActivity should not be after creation")
	})

	t.Run("LastActivity updated on AcceptUserMessage", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})
		initialActivity := cm.LastActivity()

		time.Sleep(10 * time.Millisecond)

		provider := newStopProvider("hi")
		_ = cm.AcceptUserMessage(context.Background(), provider, "model", "model", "hello")
		defer func() { _ = cm.Cancel(context.Background()) }()

		updatedActivity := cm.LastActivity()
		assert.True(t, updatedActivity.After(initialActivity), "LastActivity should be updated after AcceptUserMessage")
	})
}

func TestConversationManager_CostInRecordMessage(t *testing.T) {
	t.Parallel()

	t.Run("assistant message with usage gets cost calculated", func(t *testing.T) {
		t.Parallel()

		var recordedMessages []Message
		var mu sync.Mutex

		cm := NewConversationManager(ConversationManagerConfig{
			InputCostPer1M:  3.0,
			OutputCostPer1M: 15.0,
			OnMessage: func(_ context.Context, msg Message) error {
				mu.Lock()
				recordedMessages = append(recordedMessages, msg)
				mu.Unlock()
				return nil
			},
		})

		recordFunc := cm.createRecordMessageFunc()

		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		msg := Message{
			Type:  MessageTypeAssistant,
			Usage: usage,
		}

		err := recordFunc(context.Background(), msg)
		require.NoError(t, err)

		// Verify cost is accumulated
		assert.InDelta(t, 0.0105, cm.GetTotalCost(), 1e-9)

		// Verify recorded message has Cost field set
		mu.Lock()
		require.Len(t, recordedMessages, 1)
		require.NotNil(t, recordedMessages[0].Cost)
		assert.InDelta(t, 0.0105, *recordedMessages[0].Cost, 1e-9)
		mu.Unlock()
	})

	t.Run("user message does not get cost calculated", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			InputCostPer1M:  3.0,
			OutputCostPer1M: 15.0,
		})

		recordFunc := cm.createRecordMessageFunc()

		msg := Message{
			Type:    MessageTypeUser,
			Content: "hello",
		}

		err := recordFunc(context.Background(), msg)
		require.NoError(t, err)

		assert.Equal(t, 0.0, cm.GetTotalCost())
	})

	t.Run("multiple assistant messages accumulate cost", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			InputCostPer1M:  3.0,
			OutputCostPer1M: 15.0,
		})

		recordFunc := cm.createRecordMessageFunc()

		for i := 0; i < 3; i++ {
			msg := Message{
				Type:  MessageTypeAssistant,
				Usage: &llm.Usage{PromptTokens: 1000, CompletionTokens: 500},
			}
			_ = recordFunc(context.Background(), msg)
		}

		assert.InDelta(t, 0.0105*3, cm.GetTotalCost(), 1e-9)
	})
}

func TestConversationManager_SetWorkingBroadcastsCost(t *testing.T) {
	t.Parallel()

	t.Run("broadcast includes TotalCost", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{ID: "test"})

		// Add some cost
		cm.addCost(0.05)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := cm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			cm.SetWorking(true)
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, ok := next()
			if ok && resp.ConversationState != nil {
				assert.True(t, resp.ConversationState.Working)
				assert.InDelta(t, 0.05, resp.ConversationState.TotalCost, 1e-9)
			}
		}()

		select {
		case <-done:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for broadcast")
		}
	})
}

func TestConversationManager_SetSafeMode(t *testing.T) {
	t.Parallel()

	t.Run("SetSafeMode does not panic", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{
			SafeMode: false,
		})

		// Should not panic and should complete without error
		assert.NotPanics(t, func() {
			cm.SetSafeMode(true)
		})
	})

	t.Run("SetSafeMode is thread-safe", func(t *testing.T) {
		t.Parallel()

		cm := NewConversationManager(ConversationManagerConfig{})

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(val bool) {
				defer wg.Done()
				cm.SetSafeMode(val)
			}(i%2 == 0)
		}
		wg.Wait()
		// If we reach here without race condition, test passes
	})
}
