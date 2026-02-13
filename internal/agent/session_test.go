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

func TestNewSessionManager(t *testing.T) {
	t.Parallel()

	t.Run("generates ID if not provided", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		assert.NotEmpty(t, sm.ID())
	})

	t.Run("uses provided ID", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{ID: "custom-id"})
		assert.Equal(t, "custom-id", sm.ID())
	})

	t.Run("stores user ID", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{UserID: "user-123"})
		assert.Equal(t, "user-123", sm.UserID())
	})

	t.Run("copies history messages", func(t *testing.T) {
		t.Parallel()

		history := []Message{
			{ID: "1", Content: "first"},
			{ID: "2", Content: "second"},
		}

		sm := NewSessionManager(SessionManagerConfig{
			History: history,
		})

		msgs := sm.GetMessages()
		assert.Len(t, msgs, 2)
		assert.Equal(t, "first", msgs[0].Content)
		assert.Equal(t, "second", msgs[1].Content)
	})
}

func TestSessionManager_SetWorking(t *testing.T) {
	t.Parallel()

	t.Run("updates working state", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{ID: "test"})

		assert.False(t, sm.IsWorking())

		sm.SetWorking(true)
		assert.True(t, sm.IsWorking())

		sm.SetWorking(false)
		assert.False(t, sm.IsWorking())
	})

	t.Run("calls callback on state change", func(t *testing.T) {
		t.Parallel()

		type workingChange struct {
			id      string
			working bool
		}

		var mu sync.Mutex
		var callbackCalls []workingChange

		sm := NewSessionManager(SessionManagerConfig{
			ID: "test-sess",
			OnWorkingChange: func(id string, working bool) {
				mu.Lock()
				callbackCalls = append(callbackCalls, workingChange{id, working})
				mu.Unlock()
			},
		})

		sm.SetWorking(true)
		sm.SetWorking(true) // Duplicate should be ignored
		sm.SetWorking(false)

		mu.Lock()
		calls := append([]workingChange{}, callbackCalls...)
		mu.Unlock()

		require.Len(t, calls, 2)
		assert.True(t, calls[0].working)
		assert.False(t, calls[1].working)
		assert.Equal(t, "test-sess", calls[0].id)
	})

	t.Run("broadcasts to subscribers", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{ID: "test"})

		ctx := t.Context()

		next := sm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			sm.SetWorking(true)
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, ok := next()
			if ok && resp.SessionState != nil {
				assert.True(t, resp.SessionState.Working)
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

func TestSessionManager_AcceptUserMessage(t *testing.T) {
	t.Parallel()

	t.Run("requires provider", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		err := sm.AcceptUserMessage(context.Background(), nil, "config-id", "provider-model", "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("starts loop and queues message", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("hi")

		sm := NewSessionManager(SessionManagerConfig{})
		err := sm.AcceptUserMessage(context.Background(), provider, "config-id", "provider-model", "hello")

		require.NoError(t, err)
		assert.True(t, sm.IsWorking())

		_ = sm.Cancel(context.Background())
	})

	t.Run("adds message to session", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("response")

		sm := NewSessionManager(SessionManagerConfig{})
		_ = sm.AcceptUserMessage(context.Background(), provider, "config-id", "provider-model", "hello")

		time.Sleep(50 * time.Millisecond)

		msgs := sm.GetMessages()
		require.NotEmpty(t, msgs)
		assert.Equal(t, MessageTypeUser, msgs[0].Type)
		assert.Equal(t, "hello", msgs[0].Content)

		_ = sm.Cancel(context.Background())
	})
}

func TestSessionManager_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("receives messages", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{ID: "test"})

		ctx := t.Context()

		next := sm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			sm.SetWorking(true)
		}()

		resp, ok := next()
		assert.True(t, ok)
		assert.NotNil(t, resp.SessionState)
	})
}

func TestSessionManager_SubscribeWithSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("returns initial snapshot", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{
			ID:     "test",
			UserID: "user1",
			History: []Message{
				{ID: "1", Content: "first"},
			},
		})

		ctx := t.Context()

		snapshot, _ := sm.SubscribeWithSnapshot(ctx)

		assert.NotNil(t, snapshot.Session)
		assert.Equal(t, "test", snapshot.Session.ID)
		assert.NotNil(t, snapshot.SessionState)
		assert.Len(t, snapshot.Messages, 1)
	})
}

func TestSessionManager_Cancel(t *testing.T) {
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

		sm := NewSessionManager(SessionManagerConfig{})
		_ = sm.AcceptUserMessage(context.Background(), provider, "config-id", "provider-model", "hello")

		time.Sleep(50 * time.Millisecond)

		err := sm.Cancel(context.Background())
		assert.NoError(t, err)
		assert.False(t, sm.IsWorking())
	})

	t.Run("safe to call when no loop", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		err := sm.Cancel(context.Background())
		assert.NoError(t, err)
	})
}

func TestSessionManager_GetMessages(t *testing.T) {
	t.Parallel()

	t.Run("returns copy of messages", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{
			History: []Message{
				{ID: "1", Content: "first"},
				{ID: "2", Content: "second"},
			},
		})

		msgs := sm.GetMessages()
		assert.Len(t, msgs, 2)

		// Verify it's a copy (modification shouldn't affect original)
		msgs[0].Content = "modified"
		originalMsgs := sm.GetMessages()
		assert.Equal(t, "first", originalMsgs[0].Content)
	})
}

func TestSessionManager_GetSession(t *testing.T) {
	t.Parallel()

	t.Run("returns session metadata", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{
			ID:     "sess-123",
			UserID: "user-456",
		})

		sess := sm.GetSession()

		assert.Equal(t, "sess-123", sess.ID)
		assert.Equal(t, "user-456", sess.UserID)
		assert.False(t, sess.CreatedAt.IsZero())
	})
}

func TestSessionManager_GetModel(t *testing.T) {
	t.Parallel()

	t.Run("returns empty before accept", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		assert.Empty(t, sm.GetModel())
	})

	t.Run("returns model after accept", func(t *testing.T) {
		t.Parallel()

		provider := newStopProvider("hi")

		sm := NewSessionManager(SessionManagerConfig{})
		_ = sm.AcceptUserMessage(context.Background(), provider, "test-model", "test-model", "hello")

		assert.Equal(t, "test-model", sm.GetModel())

		_ = sm.Cancel(context.Background())
	})
}

func TestSessionManager_CostTracking(t *testing.T) {
	t.Parallel()

	pricedConfig := SessionManagerConfig{
		InputCostPer1M:  3.0,
		OutputCostPer1M: 15.0,
	}

	t.Run("calculateCost with usage", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(pricedConfig)
		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		cost := sm.calculateCost(usage)

		// (1000 * 3 / 1_000_000) + (500 * 15 / 1_000_000) = 0.003 + 0.0075 = 0.0105
		assert.InDelta(t, 0.0105, cost, 1e-9)
	})

	t.Run("calculateCost with nil usage returns zero", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(pricedConfig)
		assert.Equal(t, 0.0, sm.calculateCost(nil))
	})

	t.Run("addCost accumulates", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		sm.addCost(0.01)
		sm.addCost(0.01)
		assert.InDelta(t, 0.02, sm.GetTotalCost(), 1e-9)
	})

	t.Run("GetTotalCost starts at zero", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		assert.Equal(t, 0.0, sm.GetTotalCost())
	})

	t.Run("calculateCost with zero pricing returns zero", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		assert.Equal(t, 0.0, sm.calculateCost(usage))
	})
}

func TestSessionManager_UpdatePricing(t *testing.T) {
	t.Parallel()

	t.Run("updates pricing for subsequent cost calculations", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})

		// Initially zero pricing
		usage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}
		cost := sm.calculateCost(usage)
		assert.Equal(t, 0.0, cost)

		// Update pricing
		sm.UpdatePricing(5.0, 25.0)

		cost = sm.calculateCost(usage)
		// (1000 * 5 / 1_000_000) + (500 * 25 / 1_000_000) = 0.005 + 0.0125 = 0.0175
		assert.InDelta(t, 0.0175, cost, 1e-9)
	})
}

func TestSessionManager_LastActivity(t *testing.T) {
	t.Parallel()

	t.Run("initial LastActivity is recent", func(t *testing.T) {
		t.Parallel()

		before := time.Now()
		sm := NewSessionManager(SessionManagerConfig{})
		after := time.Now()

		la := sm.LastActivity()
		assert.False(t, la.Before(before), "LastActivity should not be before creation")
		assert.False(t, la.After(after), "LastActivity should not be after creation")
	})

	t.Run("LastActivity updated on AcceptUserMessage", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})
		initialActivity := sm.LastActivity()

		time.Sleep(10 * time.Millisecond)

		provider := newStopProvider("hi")
		_ = sm.AcceptUserMessage(context.Background(), provider, "config-id", "provider-model", "hello")
		defer func() { _ = sm.Cancel(context.Background()) }()

		updatedActivity := sm.LastActivity()
		assert.True(t, updatedActivity.After(initialActivity), "LastActivity should be updated after AcceptUserMessage")
	})
}

func TestSessionManager_CostInRecordMessage(t *testing.T) {
	t.Parallel()

	const (
		inputCost  = 3.0
		outputCost = 15.0
		// (1000 * 3 / 1_000_000) + (500 * 15 / 1_000_000) = 0.0105
		expectedCostPerMsg = 0.0105
	)

	newPricedManager := func(onMessage func(context.Context, Message) error) *SessionManager {
		return NewSessionManager(SessionManagerConfig{
			InputCostPer1M:  inputCost,
			OutputCostPer1M: outputCost,
			OnMessage:       onMessage,
		})
	}

	testUsage := &llm.Usage{PromptTokens: 1000, CompletionTokens: 500}

	t.Run("assistant message with usage gets cost calculated", func(t *testing.T) {
		t.Parallel()

		var recordedMessages []Message
		var mu sync.Mutex

		sm := newPricedManager(func(_ context.Context, msg Message) error {
			mu.Lock()
			recordedMessages = append(recordedMessages, msg)
			mu.Unlock()
			return nil
		})

		recordFunc := sm.createRecordMessageFunc()
		err := recordFunc(context.Background(), Message{Type: MessageTypeAssistant, Usage: testUsage})
		require.NoError(t, err)

		assert.InDelta(t, expectedCostPerMsg, sm.GetTotalCost(), 1e-9)

		mu.Lock()
		require.Len(t, recordedMessages, 1)
		require.NotNil(t, recordedMessages[0].Cost)
		assert.InDelta(t, expectedCostPerMsg, *recordedMessages[0].Cost, 1e-9)
		mu.Unlock()
	})

	t.Run("user message does not get cost calculated", func(t *testing.T) {
		t.Parallel()

		sm := newPricedManager(nil)
		recordFunc := sm.createRecordMessageFunc()

		err := recordFunc(context.Background(), Message{Type: MessageTypeUser, Content: "hello"})
		require.NoError(t, err)

		assert.Equal(t, 0.0, sm.GetTotalCost())
	})

	t.Run("multiple assistant messages accumulate cost", func(t *testing.T) {
		t.Parallel()

		sm := newPricedManager(nil)
		recordFunc := sm.createRecordMessageFunc()

		for range 3 {
			_ = recordFunc(context.Background(), Message{
				Type:  MessageTypeAssistant,
				Usage: testUsage,
			})
		}

		assert.InDelta(t, expectedCostPerMsg*3, sm.GetTotalCost(), 1e-9)
	})
}

func TestSessionManager_SetWorkingBroadcastsCost(t *testing.T) {
	t.Parallel()

	t.Run("broadcast includes TotalCost", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{ID: "test"})

		// Add some cost
		sm.addCost(0.05)

		ctx := t.Context()

		next := sm.Subscribe(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			sm.SetWorking(true)
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, ok := next()
			if ok && resp.SessionState != nil {
				assert.True(t, resp.SessionState.Working)
				assert.InDelta(t, 0.05, resp.SessionState.TotalCost, 1e-9)
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

func TestSessionManager_SetSafeMode(t *testing.T) {
	t.Parallel()

	t.Run("SetSafeMode does not panic", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{
			SafeMode: false,
		})

		// Should not panic and should complete without error
		assert.NotPanics(t, func() {
			sm.SetSafeMode(true)
		})
	})

	t.Run("SetSafeMode is thread-safe", func(t *testing.T) {
		t.Parallel()

		sm := NewSessionManager(SessionManagerConfig{})

		var wg sync.WaitGroup
		for i := range 10 {
			wg.Add(1)
			go func(val bool) {
				defer wg.Done()
				sm.SetSafeMode(val)
			}(i%2 == 0)
		}
		wg.Wait()
		// If we reach here without race condition, test passes
	})
}
