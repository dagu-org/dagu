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

func TestStore_ConversationCRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store
	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	now := time.Now()

	// Test CreateConversation
	conv := &agent.Conversation{
		ID:        "test-conv-1",
		UserID:    "user-1",
		Title:     "Test Conversation",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store.CreateConversation(ctx, conv)
	require.NoError(t, err, "CreateConversation() failed")

	// Test GetConversation
	retrieved, err := store.GetConversation(ctx, conv.ID)
	require.NoError(t, err, "GetConversation() failed")
	assert.Equal(t, conv.ID, retrieved.ID)
	assert.Equal(t, conv.UserID, retrieved.UserID)
	assert.Equal(t, conv.Title, retrieved.Title)

	// Test ListConversations
	conversations, err := store.ListConversations(ctx, "user-1")
	require.NoError(t, err, "ListConversations() failed")
	assert.Len(t, conversations, 1)
	assert.Equal(t, conv.ID, conversations[0].ID)

	// Test UpdateConversation
	conv.Title = "Updated Title"
	conv.UpdatedAt = time.Now()
	err = store.UpdateConversation(ctx, conv)
	require.NoError(t, err, "UpdateConversation() failed")
	retrieved, err = store.GetConversation(ctx, conv.ID)
	require.NoError(t, err, "GetConversation() after Update failed")
	assert.Equal(t, "Updated Title", retrieved.Title)

	// Test DeleteConversation
	err = store.DeleteConversation(ctx, conv.ID)
	require.NoError(t, err, "DeleteConversation() failed")
	_, err = store.GetConversation(ctx, conv.ID)
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)
}

func TestStore_Messages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	now := time.Now()

	// Create a conversation first
	conv := &agent.Conversation{
		ID:        "test-conv-messages",
		UserID:    "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store.CreateConversation(ctx, conv)
	require.NoError(t, err, "CreateConversation() failed")

	// Test AddMessage
	msg1 := &agent.Message{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeUser,
		SequenceID:     1,
		Content:        "Hello, how can you help me?",
		CreatedAt:      now,
	}
	err = store.AddMessage(ctx, conv.ID, msg1)
	require.NoError(t, err, "AddMessage() failed")

	msg2 := &agent.Message{
		ID:             "msg-2",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeAssistant,
		SequenceID:     2,
		Content:        "I can help you with DAG workflows.",
		CreatedAt:      now.Add(time.Second),
	}
	err = store.AddMessage(ctx, conv.ID, msg2)
	require.NoError(t, err, "AddMessage() second message failed")

	// Test GetMessages
	messages, err := store.GetMessages(ctx, conv.ID)
	require.NoError(t, err, "GetMessages() failed")
	assert.Len(t, messages, 2)
	assert.Equal(t, "msg-1", messages[0].ID)
	assert.Equal(t, "msg-2", messages[1].ID)

	// Test GetLatestSequenceID
	seqID, err := store.GetLatestSequenceID(ctx, conv.ID)
	require.NoError(t, err, "GetLatestSequenceID() failed")
	assert.Equal(t, int64(2), seqID)

	// Test auto-generated title from first user message
	retrieved, err := store.GetConversation(ctx, conv.ID)
	require.NoError(t, err, "GetConversation() failed")
	assert.Equal(t, "Hello, how can you help me?", retrieved.Title)
}

func TestStore_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test GetConversation not found
	_, err = store.GetConversation(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)

	// Test GetMessages not found
	_, err = store.GetMessages(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)

	// Test AddMessage to nonexistent conversation
	msg := &agent.Message{
		ID:         "msg-1",
		Type:       agent.MessageTypeUser,
		SequenceID: 1,
		Content:    "Hello",
		CreatedAt:  time.Now(),
	}
	err = store.AddMessage(ctx, "nonexistent-id", msg)
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)

	// Test DeleteConversation not found
	err = store.DeleteConversation(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)
}

func TestStore_MultipleUsers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	now := time.Now()

	// Create conversations for user 1
	conv1 := &agent.Conversation{
		ID:        "conv-1",
		UserID:    "user-1",
		Title:     "User 1 Conv 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store.CreateConversation(ctx, conv1)
	require.NoError(t, err)

	conv2 := &agent.Conversation{
		ID:        "conv-2",
		UserID:    "user-1",
		Title:     "User 1 Conv 2",
		CreatedAt: now.Add(time.Second),
		UpdatedAt: now.Add(time.Second),
	}
	err = store.CreateConversation(ctx, conv2)
	require.NoError(t, err)

	// Create conversation for user 2
	conv3 := &agent.Conversation{
		ID:        "conv-3",
		UserID:    "user-2",
		Title:     "User 2 Conv 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store.CreateConversation(ctx, conv3)
	require.NoError(t, err)

	// List conversations for user 1
	user1Convs, err := store.ListConversations(ctx, "user-1")
	require.NoError(t, err)
	assert.Len(t, user1Convs, 2)

	// List conversations for user 2
	user2Convs, err := store.ListConversations(ctx, "user-2")
	require.NoError(t, err)
	assert.Len(t, user2Convs, 1)

	// List conversations for nonexistent user
	emptyConvs, err := store.ListConversations(ctx, "user-3")
	require.NoError(t, err)
	assert.Len(t, emptyConvs, 0)
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx := context.Background()
	now := time.Now()

	// Create store and add data
	store1, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	conv := &agent.Conversation{
		ID:        "test-conv",
		UserID:    "user-1",
		Title:     "Test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store1.CreateConversation(ctx, conv)
	require.NoError(t, err)

	msg := &agent.Message{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Type:           agent.MessageTypeUser,
		SequenceID:     1,
		Content:        "Hello",
		CreatedAt:      now,
	}
	err = store1.AddMessage(ctx, conv.ID, msg)
	require.NoError(t, err)

	// Create a new store (simulating server restart)
	store2, err := New(tmpDir)
	require.NoError(t, err, "failed to create second store")

	// Verify data is available in the new store
	retrieved, err := store2.GetConversation(ctx, conv.ID)
	require.NoError(t, err, "GetConversation() on reloaded store failed")
	assert.Equal(t, conv.ID, retrieved.ID)

	messages, err := store2.GetMessages(ctx, conv.ID)
	require.NoError(t, err, "GetMessages() on reloaded store failed")
	assert.Len(t, messages, 1)

	conversations, err := store2.ListConversations(ctx, "user-1")
	require.NoError(t, err, "ListConversations() on reloaded store failed")
	assert.Len(t, conversations, 1)
}

func TestStore_ValidationErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileconversation-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test CreateConversation with nil
	err = store.CreateConversation(ctx, nil)
	assert.Error(t, err)

	// Test CreateConversation with empty ID
	err = store.CreateConversation(ctx, &agent.Conversation{
		ID:     "",
		UserID: "user-1",
	})
	assert.ErrorIs(t, err, agent.ErrInvalidConversationID)

	// Test CreateConversation with empty UserID
	err = store.CreateConversation(ctx, &agent.Conversation{
		ID:     "conv-1",
		UserID: "",
	})
	assert.ErrorIs(t, err, agent.ErrInvalidUserID)

	// Test GetConversation with empty ID
	_, err = store.GetConversation(ctx, "")
	assert.ErrorIs(t, err, agent.ErrInvalidConversationID)

	// Test ListConversations with empty userID
	_, err = store.ListConversations(ctx, "")
	assert.ErrorIs(t, err, agent.ErrInvalidUserID)

	// Test AddMessage with nil
	now := time.Now()
	conv := &agent.Conversation{
		ID:        "conv-1",
		UserID:    "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = store.CreateConversation(ctx, conv)
	require.NoError(t, err)

	err = store.AddMessage(ctx, conv.ID, nil)
	assert.Error(t, err)
}
