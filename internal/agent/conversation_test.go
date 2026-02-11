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
