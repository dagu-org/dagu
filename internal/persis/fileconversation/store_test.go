package fileconversation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	store, err := New(tmpDir)
	require.NoError(t, err)

	return store, context.Background()
}

func TestStore_ConversationCRUD(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now()

	conv := &agent.Conversation{
		ID:        "test-conv-1",
		UserID:    "user-1",
		Title:     "Test Conversation",
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("create conversation", func(t *testing.T) {
		err := store.CreateConversation(ctx, conv)
		require.NoError(t, err)
	})

	t.Run("get conversation", func(t *testing.T) {
		retrieved, err := store.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, conv.ID, retrieved.ID)
		assert.Equal(t, conv.UserID, retrieved.UserID)
		assert.Equal(t, conv.Title, retrieved.Title)
	})

	t.Run("list conversations", func(t *testing.T) {
		conversations, err := store.ListConversations(ctx, "user-1")
		require.NoError(t, err)
		assert.Len(t, conversations, 1)
		assert.Equal(t, conv.ID, conversations[0].ID)
	})

	t.Run("update conversation", func(t *testing.T) {
		conv.Title = "Updated Title"
		conv.UpdatedAt = time.Now()
		err := store.UpdateConversation(ctx, conv)
		require.NoError(t, err)

		retrieved, err := store.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", retrieved.Title)
	})

	t.Run("delete conversation", func(t *testing.T) {
		err := store.DeleteConversation(ctx, conv.ID)
		require.NoError(t, err)

		_, err = store.GetConversation(ctx, conv.ID)
		assert.ErrorIs(t, err, agent.ErrConversationNotFound)
	})
}

func TestStore_Messages(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now()

	conv := &agent.Conversation{
		ID:        "test-conv-messages",
		UserID:    "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.CreateConversation(ctx, conv))

	msg1 := &agent.Message{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeUser,
		SequenceID:     1,
		Content:        "Hello, how can you help me?",
		CreatedAt:      now,
	}
	msg2 := &agent.Message{
		ID:             "msg-2",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeAssistant,
		SequenceID:     2,
		Content:        "I can help you with DAG workflows.",
		CreatedAt:      now.Add(time.Second),
	}

	t.Run("add messages", func(t *testing.T) {
		require.NoError(t, store.AddMessage(ctx, conv.ID, msg1))
		require.NoError(t, store.AddMessage(ctx, conv.ID, msg2))
	})

	t.Run("get messages", func(t *testing.T) {
		messages, err := store.GetMessages(ctx, conv.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "msg-1", messages[0].ID)
		assert.Equal(t, "msg-2", messages[1].ID)
	})

	t.Run("get latest sequence ID", func(t *testing.T) {
		seqID, err := store.GetLatestSequenceID(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), seqID)
	})

	t.Run("auto-generates title from first user message", func(t *testing.T) {
		retrieved, err := store.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, "Hello, how can you help me?", retrieved.Title)
	})
}

func TestStore_NotFound(t *testing.T) {
	store, ctx := setupTestStore(t)

	t.Run("get conversation", func(t *testing.T) {
		_, err := store.GetConversation(ctx, "nonexistent-id")
		assert.ErrorIs(t, err, agent.ErrConversationNotFound)
	})

	t.Run("get messages", func(t *testing.T) {
		_, err := store.GetMessages(ctx, "nonexistent-id")
		assert.ErrorIs(t, err, agent.ErrConversationNotFound)
	})

	t.Run("add message", func(t *testing.T) {
		msg := &agent.Message{
			ID:         "msg-1",
			Type:       agent.MessageTypeUser,
			SequenceID: 1,
			Content:    "Hello",
			CreatedAt:  time.Now(),
		}
		err := store.AddMessage(ctx, "nonexistent-id", msg)
		assert.ErrorIs(t, err, agent.ErrConversationNotFound)
	})

	t.Run("delete conversation", func(t *testing.T) {
		err := store.DeleteConversation(ctx, "nonexistent-id")
		assert.ErrorIs(t, err, agent.ErrConversationNotFound)
	})
}

func TestStore_MultipleUsers(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now()

	conversations := []*agent.Conversation{
		{ID: "conv-1", UserID: "user-1", Title: "User 1 Conv 1", CreatedAt: now, UpdatedAt: now},
		{ID: "conv-2", UserID: "user-1", Title: "User 1 Conv 2", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)},
		{ID: "conv-3", UserID: "user-2", Title: "User 2 Conv 1", CreatedAt: now, UpdatedAt: now},
	}
	for _, conv := range conversations {
		require.NoError(t, store.CreateConversation(ctx, conv))
	}

	t.Run("user 1 has two conversations", func(t *testing.T) {
		convs, err := store.ListConversations(ctx, "user-1")
		require.NoError(t, err)
		assert.Len(t, convs, 2)
	})

	t.Run("user 2 has one conversation", func(t *testing.T) {
		convs, err := store.ListConversations(ctx, "user-2")
		require.NoError(t, err)
		assert.Len(t, convs, 1)
	})

	t.Run("nonexistent user has no conversations", func(t *testing.T) {
		convs, err := store.ListConversations(ctx, "user-3")
		require.NoError(t, err)
		assert.Empty(t, convs)
	})
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	now := time.Now()

	store1, err := New(tmpDir)
	require.NoError(t, err)

	conv := &agent.Conversation{
		ID:        "test-conv",
		UserID:    "user-1",
		Title:     "Test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store1.CreateConversation(ctx, conv))

	msg := &agent.Message{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeUser,
		SequenceID:     1,
		Content:        "Hello",
		CreatedAt:      now,
	}
	require.NoError(t, store1.AddMessage(ctx, conv.ID, msg))

	// Simulate server restart by creating a new store instance
	store2, err := New(tmpDir)
	require.NoError(t, err)

	t.Run("conversation persists after reload", func(t *testing.T) {
		retrieved, err := store2.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, conv.ID, retrieved.ID)
	})

	t.Run("messages persist after reload", func(t *testing.T) {
		messages, err := store2.GetMessages(ctx, conv.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
	})

	t.Run("list conversations works after reload", func(t *testing.T) {
		conversations, err := store2.ListConversations(ctx, "user-1")
		require.NoError(t, err)
		assert.Len(t, conversations, 1)
	})
}

func TestStore_ValidationErrors(t *testing.T) {
	store, ctx := setupTestStore(t)

	t.Run("create conversation with nil", func(t *testing.T) {
		err := store.CreateConversation(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("create conversation with empty ID", func(t *testing.T) {
		err := store.CreateConversation(ctx, &agent.Conversation{ID: "", UserID: "user-1"})
		assert.ErrorIs(t, err, agent.ErrInvalidConversationID)
	})

	t.Run("create conversation with empty UserID", func(t *testing.T) {
		err := store.CreateConversation(ctx, &agent.Conversation{ID: "conv-1", UserID: ""})
		assert.ErrorIs(t, err, agent.ErrInvalidUserID)
	})

	t.Run("get conversation with empty ID", func(t *testing.T) {
		_, err := store.GetConversation(ctx, "")
		assert.ErrorIs(t, err, agent.ErrInvalidConversationID)
	})

	t.Run("list conversations with empty userID", func(t *testing.T) {
		_, err := store.ListConversations(ctx, "")
		assert.ErrorIs(t, err, agent.ErrInvalidUserID)
	})

	t.Run("add message with nil", func(t *testing.T) {
		now := time.Now()
		require.NoError(t, store.CreateConversation(ctx, &agent.Conversation{
			ID:        "conv-for-nil-msg",
			UserID:    "user-1",
			CreatedAt: now,
			UpdatedAt: now,
		}))

		err := store.AddMessage(ctx, "conv-for-nil-msg", nil)
		assert.Error(t, err)
	})
}
